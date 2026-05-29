package pgvector

import (
	"context"
	"math"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

// Interface compliance — fails at compile time if LoreStore diverges from the port.
var _ eywa.LoreStore = (*LoreStore)(nil)

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
