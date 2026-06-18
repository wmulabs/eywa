package pgvector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	eywa "github.com/wmulabs/eywa"
)

// anyArgs returns n positional AnyArg matchers (query args: vector, loreID, distThreshold, [filter…], topK).
func anyArgs(n int) []any {
	a := make([]any, n)
	for i := range a {
		a[i] = pgxmock.AnyArg()
	}
	return a
}

func TestSearchFiltered_ReturnsScoredChunks(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("new mock pool: %v", err)
	}
	defer mock.Close()

	store := &LoreStore{pool: mock, dim: 2}
	rows := pgxmock.NewRows([]string{"id", "lore_id", "content", "metadata", "created_at", "score"}).
		AddRow("c1", "lore", "hello", []byte(`{"loc":"SP"}`), time.Now(), 0.88)
	mock.ExpectQuery("SELECT id, lore_id, content").WithArgs(anyArgs(5)...).WillReturnRows(rows)

	got, err := store.SearchFiltered(context.Background(), "lore", []float32{0.1, 0.2}, eywa.LoreSearchOptions{
		TopK:   5,
		Filter: &eywa.LoreFilter{Equals: map[string]any{"loc": "SP"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "c1" || got[0].Score != 0.88 || got[0].Metadata["loc"] != "SP" {
		t.Errorf("unexpected result: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSearchFiltered_EmptyQuery(t *testing.T) {
	store := &LoreStore{pool: nil, dim: 2}
	if _, err := store.SearchFiltered(context.Background(), "lore", nil, eywa.LoreSearchOptions{}); err == nil {
		t.Error("expected error for empty query vector")
	}
}

func TestSearchFiltered_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("new mock pool: %v", err)
	}
	defer mock.Close()

	store := &LoreStore{pool: mock, dim: 2}
	mock.ExpectQuery("SELECT").WithArgs(anyArgs(4)...).WillReturnError(errors.New("db down"))

	if _, err := store.SearchFiltered(context.Background(), "lore", []float32{0.1, 0.2}, eywa.LoreSearchOptions{}); err == nil {
		t.Error("expected the query error to propagate")
	}
}

func TestSearchFiltered_FilterError(t *testing.T) {
	store := &LoreStore{pool: nil, dim: 2}
	_, err := store.SearchFiltered(context.Background(), "lore", []float32{0.1, 0.2}, eywa.LoreSearchOptions{
		Filter: &eywa.LoreFilter{Equals: map[string]any{"bad": make(chan int)}},
	})
	if err == nil {
		t.Error("expected an error when the filter cannot be built")
	}
}

func TestSearch_DelegatesToFiltered(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("new mock pool: %v", err)
	}
	defer mock.Close()

	store := &LoreStore{pool: mock, dim: 2}
	rows := pgxmock.NewRows([]string{"id", "lore_id", "content", "metadata", "created_at", "score"}).
		AddRow("c1", "lore", "hi", []byte(nil), time.Now(), 0.5)
	mock.ExpectQuery("SELECT id, lore_id, content").WithArgs(anyArgs(4)...).WillReturnRows(rows)

	got, err := store.Search(context.Background(), "lore", []float32{0.1, 0.2}, 3, 0.4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "c1" {
		t.Errorf("unexpected result: %+v", got)
	}
}
