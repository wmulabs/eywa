package orchestrator

import (
	"context"
	"fmt"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type MessageManager interface {
	Append(ctx context.Context, message entities.Echo) error
}

type DefaultMessageManager struct {
	messageRepo ports.EchoRepository
	logger      *zap.SugaredLogger
	tracer      trace.Tracer
}

func NewMessageManager(
	messageRepo ports.EchoRepository,
	logger *zap.SugaredLogger,
	tracer trace.Tracer,
) *DefaultMessageManager {
	return &DefaultMessageManager{
		messageRepo: messageRepo,
		logger:      logger,
		tracer:      tracer,
	}
}

func (m *DefaultMessageManager) Append(ctx context.Context, message entities.Echo) error {
	ctx, span := m.tracer.Start(ctx, "MessageManager/Append")
	defer span.End()

	if err := m.messageRepo.Append(ctx, message); err != nil {
		return fmt.Errorf("append message: %w", err)
	}
	return nil
}
