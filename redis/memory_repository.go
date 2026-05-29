package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	eywa "github.com/wmulabs/eywa"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/redis/go-redis/v9"
)

type MemoryRepository struct {
	client      *redis.Client
	serviceName string
	environment string
	memoryTTL   time.Duration
	tracer      trace.Tracer
}

func NewMemoryRepository(client *redis.Client, serviceName, environment string, memoryTTLSeconds int, tracer trace.Tracer) eywa.MemoryRepository {
	if tracer == nil {
		tracer = noop.NewTracerProvider().Tracer("eywa")
	}
	return &MemoryRepository{
		client:      client,
		serviceName: serviceName,
		environment: environment,
		memoryTTL:   time.Duration(memoryTTLSeconds) * time.Second,
		tracer:      tracer,
	}
}

// prefixKey namespaces the composite key from MemoryManager.
// Pattern: {service}:{environment}:memory:{composite_key}
// Example: Agentflow:PRD:memory:whatsapp:+5521999999:shipment:12345
func (r *MemoryRepository) prefixKey(key string) string {
	return fmt.Sprintf("%s:%s:memory:%s", r.serviceName, r.environment, key)
}

func (r *MemoryRepository) GetMemory(ctx context.Context, key string) (*eywa.Memory, error) {
	ctx, span := r.tracer.Start(ctx, "Repository/Memory/GetMemory")
	defer span.End()

	log := newLogger()

	redisKey := r.prefixKey(key)
	data, err := r.client.Get(ctx, redisKey).Result()
	if err != nil {
		if err == redis.Nil {
			log.Debugw("memory cache miss, will reconstruct", "key", key)
			return nil, fmt.Errorf("memory not found: %s", key)
		}
		log.Errorw("error getting memory", "error", err, "key", key)
		return nil, fmt.Errorf("get memory: %w", err)
	}

	var memory eywa.Memory
	if err := json.Unmarshal([]byte(data), &memory); err != nil {
		log.Errorw("error unmarshaling memory", "error", err, "key", key)
		return nil, fmt.Errorf("unmarshal memory: %w", err)
	}

	return &memory, nil
}

func (r *MemoryRepository) SaveMemory(ctx context.Context, key string, memory *eywa.Memory) error {
	ctx, span := r.tracer.Start(ctx, "Repository/Memory/SaveMemory")
	defer span.End()

	log := newLogger()

	memory.LastInteraction = eywa.NowUTC()

	data, err := json.Marshal(memory)
	if err != nil {
		log.Errorw("error marshaling memory", "error", err, "key", key)
		return fmt.Errorf("marshal memory: %w", err)
	}

	redisKey := r.prefixKey(key)
	if err := r.client.Set(ctx, redisKey, data, r.memoryTTL).Err(); err != nil {
		log.Errorw("error saving memory", "error", err, "key", key)
		return fmt.Errorf("set memory: %w", err)
	}

	log.Infow("memory saved", "key", key, "ttl", r.memoryTTL)
	return nil
}

func (r *MemoryRepository) DeleteMemory(ctx context.Context, key string) error {
	ctx, span := r.tracer.Start(ctx, "Repository/Memory/DeleteMemory")
	defer span.End()

	log := newLogger()

	redisKey := r.prefixKey(key)
	if err := r.client.Del(ctx, redisKey).Err(); err != nil {
		log.Errorw("error deleting memory", "error", err, "key", key)
		return fmt.Errorf("delete memory: %w", err)
	}

	log.Infow("memory deleted", "key", key)
	return nil
}
