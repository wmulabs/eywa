package mongo

import (
	"context"
	"fmt"
	"time"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var _ eywa.LinkRepository = (*LinkRepository)(nil)

type LinkRepository struct {
	collection *mongodriver.Collection
}

func NewLinkRepository(database *mongodriver.Database) *LinkRepository {
	return &LinkRepository{
		collection: database.Collection("event_configurations"),
	}
}

type linkDocument struct {
	EventType            string          `bson:"_id"`
	InboundConverterName string          `bson:"inbound_converter_name"`
	RequireScouts        []string        `bson:"require_scouts"`
	PathfinderName       string          `bson:"pathfinder_name"`
	AllowedSpirits       []string        `bson:"allowed_spirits"`
	DefaultSpirit        string          `bson:"default_spirit"`
	VoiceName            string          `bson:"voice_name"`
	ChannelName          string          `bson:"channel_name"`
	IngestionTimeoutNs   int64           `bson:"ingestion_timeout_ns"`
	ProcessingTimeoutNs  int64           `bson:"processing_timeout_ns"`
	Guards               []guardDocument `bson:"guards"`
	Metadata             map[string]any  `bson:"metadata"`
	UpdatedAt            time.Time       `bson:"updated_at"`
}

type guardDocument struct {
	Field     string   `bson:"field"`
	BlockList []string `bson:"block_list"`
	AllowList []string `bson:"allow_list"`
}

func linkToDocument(link *eywa.Link) linkDocument {
	guards := make([]guardDocument, len(link.Guards))
	for i, g := range link.Guards {
		guards[i] = guardDocument{
			Field:     g.Field,
			BlockList: g.BlockList,
			AllowList: g.AllowList,
		}
	}
	return linkDocument{
		EventType:            link.EventType,
		InboundConverterName: link.InboundConverterName,
		RequireScouts:        link.RequireScouts,
		PathfinderName:       link.PathfinderName,
		AllowedSpirits:       link.AllowedSpirits,
		DefaultSpirit:        link.DefaultSpirit,
		VoiceName:            link.VoiceName,
		ChannelName:          link.ChannelName,
		IngestionTimeoutNs:   link.IngestionTimeout.Nanoseconds(),
		ProcessingTimeoutNs:  link.ProcessingTimeout.Nanoseconds(),
		Guards:               guards,
		Metadata:             link.Metadata,
		UpdatedAt:            time.Now().UTC(),
	}
}

func documentToLink(doc linkDocument) *eywa.Link {
	guards := make([]eywa.Guard, len(doc.Guards))
	for i, g := range doc.Guards {
		guards[i] = eywa.Guard{
			Field:     g.Field,
			BlockList: g.BlockList,
			AllowList: g.AllowList,
		}
	}
	link := eywa.NewLink(doc.EventType).
		WithInboundConverter(doc.InboundConverterName).
		WithScouts(doc.RequireScouts...).
		WithPathfinder(doc.PathfinderName).
		WithSpirits(doc.AllowedSpirits...).
		WithDefaultSpirit(doc.DefaultSpirit).
		WithVoice(doc.VoiceName).
		WithMetadata(doc.Metadata).
		WithGuards(guards...).
		Build()
	link.ChannelName = doc.ChannelName
	link.IngestionTimeout = time.Duration(doc.IngestionTimeoutNs)
	link.ProcessingTimeout = time.Duration(doc.ProcessingTimeoutNs)
	return link
}

func (r *LinkRepository) FindAll(ctx context.Context) ([]*eywa.Link, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("find all links: %w", err)
	}
	defer cursor.Close(ctx) //nolint:errcheck
	var docs []linkDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode links: %w", err)
	}
	result := make([]*eywa.Link, len(docs))
	for i, d := range docs {
		result[i] = documentToLink(d)
	}
	return result, nil
}

func (r *LinkRepository) FindByKey(ctx context.Context, eventType string) (*eywa.Link, error) {
	var doc linkDocument
	err := r.collection.FindOne(ctx, bson.M{"_id": eventType}).Decode(&doc)
	if err == mongodriver.ErrNoDocuments {
		return nil, eywa.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("decode link: %w", err)
	}
	return documentToLink(doc), nil
}

func (r *LinkRepository) Save(ctx context.Context, link *eywa.Link) error {
	doc := linkToDocument(link)
	opts := options.Replace().SetUpsert(true)
	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": link.EventType}, doc, opts)
	if err != nil {
		return fmt.Errorf("replace link: %w", err)
	}
	return nil
}

func (r *LinkRepository) Delete(ctx context.Context, eventType string) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": eventType})
	if err != nil {
		return fmt.Errorf("delete link: %w", err)
	}
	return nil
}
