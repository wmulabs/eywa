package qdrant

import (
	"context"
	"errors"
	"testing"

	qclient "github.com/qdrant/go-client/qdrant"
	eywa "github.com/wmulabs/eywa"
)

var _ eywa.FilterableLoreStore = (*LoreStore)(nil)

func ptrF(f float64) *float64 { return &f }

type fakeQdrantClient struct {
	points    []*qclient.ScoredPoint
	err       error
	gotFilter *qclient.Filter
}

func (f *fakeQdrantClient) Query(_ context.Context, req *qclient.QueryPoints) ([]*qclient.ScoredPoint, error) {
	f.gotFilter = req.Filter
	return f.points, f.err
}
func (f *fakeQdrantClient) Upsert(_ context.Context, _ *qclient.UpsertPoints) (*qclient.UpdateResult, error) {
	return nil, nil
}
func (f *fakeQdrantClient) Delete(_ context.Context, _ *qclient.DeletePoints) (*qclient.UpdateResult, error) {
	return nil, nil
}
func (f *fakeQdrantClient) CollectionExists(_ context.Context, _ string) (bool, error) {
	return true, nil
}
func (f *fakeQdrantClient) CreateCollection(_ context.Context, _ *qclient.CreateCollection) error {
	return nil
}

func TestQdrantConditions(t *testing.T) {
	// nil filter → only the lore_id condition
	if got := qdrantConditions("lore", nil); len(got) != 1 {
		t.Errorf("nil filter: expected 1 condition, got %d", len(got))
	}

	// lore_id + 5 typed equals (string/bool/int/int64/float) + 2 ranges = 8
	filter := &eywa.LoreFilter{
		Equals: map[string]any{"s": "x", "b": true, "i": 3, "i64": int64(7), "f": 4.0},
		Ranges: map[string]eywa.LoreRange{"price": {Min: ptrF(1)}, "age": {Max: ptrF(9)}},
	}
	if got := qdrantConditions("lore", filter); len(got) != 8 {
		t.Errorf("expected 8 conditions (lore_id + 5 equals + 2 ranges), got %d", len(got))
	}
}

func TestSearchFiltered_SkipsUnparsablePayload(t *testing.T) {
	// one valid point and one without a chunk_id (skipped)
	bad := &qclient.ScoredPoint{Payload: map[string]*qclient.Value{"lore_id": qclient.NewValueString("lore")}}
	fake := &fakeQdrantClient{points: []*qclient.ScoredPoint{scoredPoint("c1", 0.9), bad}}
	store := newFakeStore(fake)

	got, err := store.SearchFiltered(context.Background(), "lore", []float32{0.1, 0.2}, eywa.LoreSearchOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "c1" {
		t.Errorf("expected the unparsable point skipped, got %+v", got)
	}
}

func newFakeStore(c *fakeQdrantClient) *LoreStore {
	return &LoreStore{client: c, collection: "lore", dim: 2}
}

func scoredPoint(id string, score float32) *qclient.ScoredPoint {
	return &qclient.ScoredPoint{
		Score: score,
		Payload: map[string]*qclient.Value{
			"chunk_id": qclient.NewValueString(id),
			"lore_id":  qclient.NewValueString("lore"),
			"content":  qclient.NewValueString("hello"),
		},
	}
}

func TestSearchFiltered_ReturnsScoredChunks(t *testing.T) {
	fake := &fakeQdrantClient{points: []*qclient.ScoredPoint{scoredPoint("c1", 0.9)}}
	store := newFakeStore(fake)

	got, err := store.SearchFiltered(context.Background(), "lore", []float32{0.1, 0.2}, eywa.LoreSearchOptions{
		TopK:   5,
		Filter: &eywa.LoreFilter{Equals: map[string]any{"loc": "SP"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "c1" || got[0].Score < 0.89 || got[0].Score > 0.91 {
		t.Errorf("unexpected result: %+v", got)
	}
	// lore_id + the one equals condition
	if fake.gotFilter == nil || len(fake.gotFilter.Must) != 2 {
		t.Errorf("expected 2 filter conditions forwarded, got %+v", fake.gotFilter)
	}
}

func TestSearchFiltered_QueryError(t *testing.T) {
	store := newFakeStore(&fakeQdrantClient{err: errors.New("grpc down")})
	if _, err := store.SearchFiltered(context.Background(), "lore", []float32{0.1, 0.2}, eywa.LoreSearchOptions{}); err == nil {
		t.Error("expected the query error to propagate")
	}
}

func TestSearch_DelegatesToFiltered(t *testing.T) {
	fake := &fakeQdrantClient{points: []*qclient.ScoredPoint{scoredPoint("c1", 0.5)}}
	store := newFakeStore(fake)

	got, err := store.Search(context.Background(), "lore", []float32{0.1, 0.2}, 3, 0.4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "c1" {
		t.Errorf("unexpected result: %+v", got)
	}
}
