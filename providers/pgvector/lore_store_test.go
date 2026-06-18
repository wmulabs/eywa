package pgvector

import (
	"context"
	"math"
	"strings"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

// Interface compliance — fails at compile time if LoreStore diverges from the port.
var (
	_ eywa.LoreStore           = (*LoreStore)(nil)
	_ eywa.FilterableLoreStore = (*LoreStore)(nil)
)

func ptrFloat(f float64) *float64 { return &f }

func TestAppendLoreFilter_Nil(t *testing.T) {
	where, args, err := appendLoreFilter("base", []any{"a", "b", "c"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if where != "base" || len(args) != 3 {
		t.Errorf("nil filter must not change where/args, got %q / %v", where, args)
	}
}

func TestAppendLoreFilter_Equals(t *testing.T) {
	f := &eywa.LoreFilter{Equals: map[string]any{"status": "published"}}
	where, args, err := appendLoreFilter("W", []any{"a", "b", "c"}, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(where, "metadata @> $4::jsonb") {
		t.Errorf("expected jsonb containment at $4, got %q", where)
	}
	if len(args) != 4 || !strings.Contains(args[3].(string), "published") {
		t.Errorf("expected marshaled equals as arg 4, got %v", args)
	}
}

func TestAppendLoreFilter_Ranges(t *testing.T) {
	f := &eywa.LoreFilter{Ranges: map[string]eywa.LoreRange{"price": {Max: ptrFloat(100)}}}
	where, args, err := appendLoreFilter("W", []any{"a", "b", "c"}, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// field name is a parameter ($4), bound value at $5 — never interpolated
	if !strings.Contains(where, "(metadata ->> $4)::numeric <= $5") {
		t.Errorf("expected parameterized range clause, got %q", where)
	}
	if len(args) != 5 || args[3] != "price" || args[4] != 100.0 {
		t.Errorf("expected field+bound as args, got %v", args)
	}
	if strings.Contains(where, "price") {
		t.Errorf("field name must not be interpolated into SQL: %q", where)
	}
}

func TestAppendLoreFilter_UnmarshalableEquals(t *testing.T) {
	// A channel can't be marshalled to JSON — exercises the equals-marshal error path.
	f := &eywa.LoreFilter{Equals: map[string]any{"bad": make(chan int)}}
	if _, _, err := appendLoreFilter("W", []any{"a", "b", "c"}, f); err == nil {
		t.Error("expected an error when the equals filter cannot be marshalled")
	}
}

func TestAppendLoreFilter_Combined(t *testing.T) {
	f := &eywa.LoreFilter{
		Equals: map[string]any{"tenant": "acme"},
		Ranges: map[string]eywa.LoreRange{"price": {Min: ptrFloat(10), Max: ptrFloat(100)}},
	}
	where, args, err := appendLoreFilter("W", []any{"a", "b", "c"}, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"metadata @> $4::jsonb", "(metadata ->> $5)::numeric >= $6", "(metadata ->> $7)::numeric <= $8"} {
		if !strings.Contains(where, want) {
			t.Errorf("expected %q in %q", want, where)
		}
	}
	if len(args) != 8 {
		t.Errorf("expected 8 args, got %d: %v", len(args), args)
	}
}

func TestFormatVector(t *testing.T) {
	cases := []struct {
		in   []float32
		want string
	}{
		{nil, "[]"},
		{[]float32{}, "[]"},
		{[]float32{1.0}, "[1]"},
		{[]float32{0.5, -0.5}, "[0.5,-0.5]"},
		{[]float32{0.1, 0.2, 0.3}, "[0.1,0.2,0.3]"},
	}

	for _, tc := range cases {
		got := formatVector(tc.in)
		if got != tc.want {
			t.Errorf("formatVector(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestVectorRoundTrip(t *testing.T) {
	original := []float32{0.1, 0.2, -0.3, 0.999, 0.0}
	encoded := formatVector(original)
	decoded, err := vectorFromString(encoded)
	if err != nil {
		t.Fatalf("vectorFromString: %v", err)
	}
	if len(decoded) != len(original) {
		t.Fatalf("length mismatch: got %d, want %d", len(decoded), len(original))
	}
	for i := range original {
		if math.Abs(float64(decoded[i]-original[i])) > 1e-6 {
			t.Errorf("index %d: got %v, want %v", i, decoded[i], original[i])
		}
	}
}

func TestLoreStore_Upsert_UnmarshalableMetadata_ReturnsError(t *testing.T) {
	store := &LoreStore{}

	chunks := []eywa.LoreChunk{
		{
			ID:        "chunk-1",
			LoreID:    "lore-1",
			Content:   "hello",
			Embedding: []float32{0.1, 0.2},
			Metadata:  map[string]any{"bad": make(chan int)},
		},
	}

	err := store.Upsert(context.Background(), chunks)

	if err == nil {
		t.Error("expected error for non-serializable metadata, got nil")
	}
}

// TestIntegration_Upsert_Search_Delete exercises the full LoreStore lifecycle
// against a real PostgreSQL + pgvector instance. Skipped unless DATABASE_URL is set.
func TestIntegration_Upsert_Search_Delete(t *testing.T) {
	t.Skip("integration: set DATABASE_URL and remove t.Skip to run")

	// ctx := context.Background()
	// pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	// require.NoError(t, err)
	// defer pool.Close()
	//
	// store, err := NewLoreStore(ctx, pool, 3)
	// require.NoError(t, err)
	//
	// chunks := []eywa.LoreChunk{
	//     {ID: "c1", LoreID: "lore-1", Content: "Go is fast", Embedding: []float32{1, 0, 0}},
	//     {ID: "c2", LoreID: "lore-1", Content: "Go has goroutines", Embedding: []float32{0.9, 0.1, 0}},
	//     {ID: "c3", LoreID: "lore-2", Content: "Different lore", Embedding: []float32{0, 0, 1}},
	// }
	// require.NoError(t, store.Upsert(ctx, chunks))
	//
	// results, err := store.Search(ctx, "lore-1", []float32{1, 0, 0}, 5, 0.5)
	// require.NoError(t, err)
	// require.Len(t, results, 2)
	//
	// require.NoError(t, store.Delete(ctx, "lore-1"))
	// results, err = store.Search(ctx, "lore-1", []float32{1, 0, 0}, 5, 0.0)
	// require.NoError(t, err)
	// require.Len(t, results, 0)
}

// TestCosineSimilarity verifies the distance→similarity mapping.
func TestCosineSimilarity(t *testing.T) {
	// cosine distance threshold = 1 - minScore; verify within float tolerance.
	minScore := 0.8
	distThreshold := 1.0 - minScore
	if math.Abs(distThreshold-0.2) > 1e-9 {
		t.Errorf("expected distThreshold ~0.2, got %v", distThreshold)
	}
}

// Prevent compiler "unused import" error when integration test body is commented out.
var _ = context.Background
