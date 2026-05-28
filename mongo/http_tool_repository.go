package mongo

import (
	"context"
	"time"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

var _ eywa.HTTPToolRepository = (*HTTPToolRepository)(nil)

type HTTPToolRepository struct {
	collection *mongodriver.Collection
	logger     *zap.SugaredLogger
}

func NewHTTPToolRepository(database *mongodriver.Database) *HTTPToolRepository {
	return &HTTPToolRepository{
		collection: database.Collection("http_tools"),
		logger:     newLogger(),
	}
}

type httpToolDocument struct {
	ID           string                  `bson:"_id"`
	Name         string                  `bson:"name"`
	Description  string                  `bson:"description"`
	Method       string                  `bson:"method"`
	URL          string                  `bson:"url"`
	Headers      map[string]string       `bson:"headers"`
	BodyTemplate string                  `bson:"body_template"`
	Parameters   []httpToolParamDocument `bson:"parameters"`
	TimeoutMS    int                     `bson:"timeout_ms"`
	IsCritical   bool                    `bson:"is_critical"`
	SpiritIDs    []string                `bson:"spirit_ids"`
	UpdatedAt    time.Time               `bson:"updated_at"`
}

type httpToolParamDocument struct {
	Name        string `bson:"name"`
	Type        string `bson:"type"`
	Description string `bson:"description"`
	Required    bool   `bson:"required"`
}

func httpToolToDocument(tool eywa.HTTPTool) httpToolDocument {
	params := make([]httpToolParamDocument, len(tool.Parameters))
	for i, p := range tool.Parameters {
		params[i] = httpToolParamDocument{
			Name:        p.Name,
			Type:        p.Type,
			Description: p.Description,
			Required:    p.Required,
		}
	}
	headers := tool.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	spiritIDs := tool.SpiritIDs
	if spiritIDs == nil {
		spiritIDs = []string{}
	}
	return httpToolDocument{
		ID:           tool.ID,
		Name:         tool.Name,
		Description:  tool.Description,
		Method:       tool.Method,
		URL:          tool.URL,
		Headers:      headers,
		BodyTemplate: tool.BodyTemplate,
		Parameters:   params,
		TimeoutMS:    tool.TimeoutMS,
		IsCritical:   tool.IsCritical,
		SpiritIDs:    spiritIDs,
		UpdatedAt:    time.Now().UTC(),
	}
}

func httpDocumentToTool(doc httpToolDocument) eywa.HTTPTool {
	params := make([]eywa.HTTPToolParam, len(doc.Parameters))
	for i, p := range doc.Parameters {
		params[i] = eywa.HTTPToolParam{
			Name:        p.Name,
			Type:        p.Type,
			Description: p.Description,
			Required:    p.Required,
		}
	}
	return eywa.HTTPTool{
		ID:           doc.ID,
		Name:         doc.Name,
		Description:  doc.Description,
		Method:       doc.Method,
		URL:          doc.URL,
		Headers:      doc.Headers,
		BodyTemplate: doc.BodyTemplate,
		Parameters:   params,
		TimeoutMS:    doc.TimeoutMS,
		IsCritical:   doc.IsCritical,
		SpiritIDs:    doc.SpiritIDs,
	}
}

func (r *HTTPToolRepository) List(ctx context.Context) ([]eywa.HTTPTool, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []httpToolDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	tools := make([]eywa.HTTPTool, len(docs))
	for i, doc := range docs {
		tools[i] = httpDocumentToTool(doc)
	}
	return tools, nil
}

func (r *HTTPToolRepository) FindBySpiritID(ctx context.Context, spiritID string) ([]eywa.HTTPTool, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"spirit_ids": spiritID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []httpToolDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	tools := make([]eywa.HTTPTool, len(docs))
	for i, doc := range docs {
		tools[i] = httpDocumentToTool(doc)
	}
	return tools, nil
}

func (r *HTTPToolRepository) FindByID(ctx context.Context, id string) (*eywa.HTTPTool, error) {
	var doc httpToolDocument
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, eywa.ErrNotFound
		}
		return nil, err
	}
	tool := httpDocumentToTool(doc)
	return &tool, nil
}

func (r *HTTPToolRepository) Save(ctx context.Context, tool eywa.HTTPTool) error {
	doc := httpToolToDocument(tool)
	_, err := r.collection.InsertOne(ctx, doc)
	return err
}

func (r *HTTPToolRepository) Update(ctx context.Context, tool eywa.HTTPTool) error {
	doc := httpToolToDocument(tool)
	result, err := r.collection.ReplaceOne(ctx, bson.M{"_id": tool.ID}, doc, options.Replace().SetUpsert(false))
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return eywa.ErrNotFound
	}
	return nil
}

func (r *HTTPToolRepository) Delete(ctx context.Context, id string) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return eywa.ErrNotFound
	}
	return nil
}
