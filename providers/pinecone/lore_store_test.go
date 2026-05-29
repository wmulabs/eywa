package pinecone

import (
	"testing"

	eywa "github.com/wmulabs/eywa"
)

// Interface compliance — fails at compile time if LoreStore diverges from the port.
var _ eywa.LoreStore = (*LoreStore)(nil)

// TestIntegration exercises the full lifecycle against a live Pinecone index.
// Skipped by default; requires a real Pinecone API key and pre-created index.
func TestIntegration_Upsert_Search_Delete(t *testing.T) {
	t.Skip("integration: set PINECONE_API_KEY + PINECONE_INDEX and remove t.Skip")
	// ctx := context.Background()
	// store, err := NewLoreStore(ctx, Config{
	//     APIKey:    os.Getenv("PINECONE_API_KEY"),
	//     IndexName: os.Getenv("PINECONE_INDEX"),
	// })
	// require.NoError(t, err)
	//
	// chunks := []eywa.LoreChunk{
	//     {ID: "c1", LoreID: "lore-1", Content: "hello world", Embedding: make([]float32, 1536)},
	// }
	// require.NoError(t, store.Upsert(ctx, chunks))
	// ...
}
