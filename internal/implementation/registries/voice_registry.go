package registries

import (
	"fmt"
	"sync"

	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/infrastructure/driven/dbg"
	"go.uber.org/zap"
)

type VoiceRegistry struct {
	channels map[string]ports.Voice
	mu       sync.RWMutex
	logger   *zap.SugaredLogger
}

func NewVoiceRegistry() ports.VoiceRegistry {
	return &VoiceRegistry{
		channels: make(map[string]ports.Voice),
		logger:   dbg.GetLogger(),
	}
}

// Register adds a Voice channel; returns an error if one with the same name already exists.
func (r *VoiceRegistry) Register(channel ports.Voice) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := channel.GetName()
	if _, exists := r.channels[name]; exists {
		r.logger.Warnw("Voice channel already registered", "channel_name", name)
		return fmt.Errorf("voice channel already registered: %s", name)
	}

	r.channels[name] = channel
	r.logger.Infow("Voice channel registered", "channel_name", name)
	return nil
}

// RegisterMultiple registers multiple Voice channels; returns on the first duplicate.
func (r *VoiceRegistry) RegisterMultiple(channels ...ports.Voice) error {
	for _, channel := range channels {
		if err := r.Register(channel); err != nil {
			return err
		}
	}
	return nil
}

func (r *VoiceRegistry) Get(name string) (ports.Voice, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	channel, exists := r.channels[name]
	if !exists {
		return nil, fmt.Errorf("voice channel not found: %s", name)
	}

	return channel, nil
}

func (r *VoiceRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.channels[name]
	return exists
}

func (r *VoiceRegistry) List() []ports.Voice {
	r.mu.RLock()
	defer r.mu.RUnlock()

	channels := make([]ports.Voice, 0, len(r.channels))
	for _, channel := range r.channels {
		channels = append(channels, channel)
	}

	return channels
}
