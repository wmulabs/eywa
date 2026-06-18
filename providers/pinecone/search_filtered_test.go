package pinecone

import (
	"context"
	"errors"
	"testing"

	pc "github.com/pinecone-io/go-pinecone/v3/pinecone"
	eywa "github.com/wmulabs/eywa"
	"google.golang.org/protobuf/types/known/structpb"
)

func ptrF(f float64) *float64 { return &f }

type fakeQuerier struct {
	resp      *pc.QueryVectorsResponse
	err       error
	gotFilter *pc.MetadataFilter
}

func (f *fakeQuerier) QueryByVectorValues(_ context.Context, in *pc.QueryByVectorValuesRequest) (*pc.QueryVectorsResponse, error) {
	f.gotFilter = in.MetadataFilter
	return f.resp, f.err
}

func scoredVector(t *testing.T, id string, score float32) *pc.ScoredVector {
	t.Helper()
	meta, err := structpb.NewStruct(map[string]any{"content": "hello", "loc": "SP"})
	if err != nil {
		t.Fatalf("build metadata: %v", err)
	}
	return &pc.ScoredVector{Score: score, Vector: &pc.Vector{Id: id, Metadata: meta}}
}

func TestPineconeFilter(t *testing.T) {
	if f, err := pineconeFilter(nil); err != nil || f != nil {
		t.Errorf("nil filter → nil, got %v / %v", f, err)
	}
	if f, err := pineconeFilter(&eywa.LoreFilter{}); err != nil || f != nil {
		t.Errorf("empty filter → nil, got %v / %v", f, err)
	}

	f, err := pineconeFilter(&eywa.LoreFilter{
		Equals: map[string]any{"loc": "SP"},
		Ranges: map[string]eywa.LoreRange{"salary": {Min: ptrF(1000), Max: ptrF(5000)}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	eq := f.Fields["loc"].GetStructValue().Fields["$eq"].GetStringValue()
	if eq != "SP" {
		t.Errorf("expected loc $eq SP, got %q", eq)
	}
	salary := f.Fields["salary"].GetStructValue().Fields
	if salary["$gte"].GetNumberValue() != 1000 || salary["$lte"].GetNumberValue() != 5000 {
		t.Errorf("expected salary range [1000,5000], got %v", salary)
	}
}

func TestPineconeFilter_EncodeError(t *testing.T) {
	// a channel can't be encoded to a protobuf value
	_, err := pineconeFilter(&eywa.LoreFilter{Equals: map[string]any{"bad": make(chan int)}})
	if err == nil {
		t.Error("expected an error encoding an unsupported filter value")
	}
}

func TestSearchFiltered_IndexConnError(t *testing.T) {
	// No index host configured → indexConn fails before any network call.
	store := &LoreStore{}
	if _, err := store.SearchFiltered(context.Background(), "lore", []float32{0.1, 0.2}, eywa.LoreSearchOptions{}); err == nil {
		t.Error("expected an error when the index connection cannot be created")
	}
}

func TestQueryVectors_ReturnsScoredChunks(t *testing.T) {
	fake := &fakeQuerier{resp: &pc.QueryVectorsResponse{Matches: []*pc.ScoredVector{scoredVector(t, "c1", 0.9)}}}
	store := &LoreStore{}

	got, err := store.queryVectors(context.Background(), fake, "lore", []float32{0.1, 0.2}, eywa.LoreSearchOptions{
		TopK:   5,
		Filter: &eywa.LoreFilter{Equals: map[string]any{"loc": "SP"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "c1" || got[0].Content != "hello" || got[0].Metadata["loc"] != "SP" {
		t.Errorf("unexpected result: %+v", got)
	}
	if got[0].Score < 0.89 || got[0].Score > 0.91 {
		t.Errorf("expected score ~0.9, got %v", got[0].Score)
	}
	if fake.gotFilter == nil {
		t.Error("expected the metadata filter to be forwarded")
	}
}

func TestQueryVectors_BelowMinScoreSkipped(t *testing.T) {
	fake := &fakeQuerier{resp: &pc.QueryVectorsResponse{Matches: []*pc.ScoredVector{scoredVector(t, "c1", 0.3)}}}
	got, err := (&LoreStore{}).queryVectors(context.Background(), fake, "lore", []float32{0.1}, eywa.LoreSearchOptions{MinScore: 0.5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected the below-threshold match skipped, got %+v", got)
	}
}

func TestQueryVectors_NilVectorSkipped(t *testing.T) {
	fake := &fakeQuerier{resp: &pc.QueryVectorsResponse{Matches: []*pc.ScoredVector{{Score: 0.9, Vector: nil}}}}
	got, err := (&LoreStore{}).queryVectors(context.Background(), fake, "lore", []float32{0.1}, eywa.LoreSearchOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected nil-vector match skipped, got %+v", got)
	}
}

func TestQueryVectors_QueryError(t *testing.T) {
	fake := &fakeQuerier{err: errors.New("pinecone down")}
	if _, err := (&LoreStore{}).queryVectors(context.Background(), fake, "lore", []float32{0.1}, eywa.LoreSearchOptions{}); err == nil {
		t.Error("expected the query error to propagate")
	}
}

func TestQueryVectors_FilterError(t *testing.T) {
	fake := &fakeQuerier{resp: &pc.QueryVectorsResponse{}}
	_, err := (&LoreStore{}).queryVectors(context.Background(), fake, "lore", []float32{0.1}, eywa.LoreSearchOptions{
		Filter: &eywa.LoreFilter{Equals: map[string]any{"bad": make(chan int)}},
	})
	if err == nil {
		t.Error("expected a filter-build error")
	}
}
