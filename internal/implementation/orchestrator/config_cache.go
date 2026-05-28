package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

const configReloadChannel = "eywa:config:reload"

type ConfigCache struct {
	mu       sync.RWMutex
	links    map[string]*entities.Link
	linkRepo ports.LinkRepository // nil = no persistence
	pubSub   ports.PubSub         // nil = no pub/sub
	logger   *zap.SugaredLogger
}

func NewConfigCache(linkRepo ports.LinkRepository, pubSub ports.PubSub, logger *zap.SugaredLogger) *ConfigCache {
	return &ConfigCache{
		links:    make(map[string]*entities.Link),
		linkRepo: linkRepo,
		pubSub:   pubSub,
		logger:   logger,
	}
}

// RegisterLink adds a link directly to the in-memory cache without persisting to MongoDB.
// Use this for static registrations in tests or when no repository is configured.
func (c *ConfigCache) RegisterLink(link *entities.Link) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.links[link.EventType] = link
}

func (c *ConfigCache) GetLink(eventType string) (*entities.Link, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	link, ok := c.links[eventType]
	return link, ok
}

func (c *ConfigCache) GetAllLinks() []*entities.Link {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*entities.Link, 0, len(c.links))
	for _, l := range c.links {
		result = append(result, l)
	}
	return result
}

// LoadAll loads all Links from the repository into the in-memory cache.
// No-op if linkRepo is nil.
func (c *ConfigCache) LoadAll(ctx context.Context) error {
	if c.linkRepo == nil {
		return nil
	}
	links, err := c.linkRepo.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("load links: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, l := range links {
		c.links[l.EventType] = l
	}
	return nil
}

// SaveLink persists link to MongoDB, updates the local cache, and notifies other instances.
func (c *ConfigCache) SaveLink(ctx context.Context, link *entities.Link) error {
	if c.linkRepo != nil {
		if err := c.linkRepo.Save(ctx, link); err != nil {
			return fmt.Errorf("save link: %w", err)
		}
	}
	c.mu.Lock()
	c.links[link.EventType] = link
	c.mu.Unlock()
	if c.pubSub != nil {
		if err := c.pubSub.Publish(ctx, configReloadChannel, "link:"+link.EventType); err != nil && c.logger != nil {
			c.logger.Errorw("config cache pubsub publish failed", "channel", configReloadChannel, "error", err)
		}
	}
	return nil
}

// DeleteLink removes a link from MongoDB and the local cache, and notifies other instances.
func (c *ConfigCache) DeleteLink(ctx context.Context, eventType string) error {
	if c.linkRepo != nil {
		if err := c.linkRepo.Delete(ctx, eventType); err != nil {
			return fmt.Errorf("delete link: %w", err)
		}
	}
	c.mu.Lock()
	delete(c.links, eventType)
	c.mu.Unlock()
	if c.pubSub != nil {
		if err := c.pubSub.Publish(ctx, configReloadChannel, "link:"+eventType); err != nil && c.logger != nil {
			c.logger.Errorw("config cache pubsub publish failed", "channel", configReloadChannel, "error", err)
		}
	}
	return nil
}

// ForceReload reloads all Links from MongoDB and notifies other instances.
func (c *ConfigCache) ForceReload(ctx context.Context) error {
	if err := c.LoadAll(ctx); err != nil {
		return err
	}
	if c.pubSub != nil {
		if err := c.pubSub.Publish(ctx, configReloadChannel, "link:*"); err != nil && c.logger != nil {
			c.logger.Errorw("config cache pubsub publish failed", "channel", configReloadChannel, "error", err)
		}
	}
	return nil
}

// Subscribe listens for reload notifications and reloads the affected Link from MongoDB.
// Blocks until ctx is cancelled. Call in a goroutine.
func (c *ConfigCache) Subscribe(ctx context.Context) {
	if c.pubSub == nil {
		return
	}
	err := c.pubSub.Subscribe(ctx, configReloadChannel, func(msg string) {
		if msg == "link:*" {
			if err := c.LoadAll(ctx); err != nil && c.logger != nil {
				c.logger.Errorw("config cache reload all failed", "error", err)
			}
			return
		}
		if strings.HasPrefix(msg, "link:") {
			eventType := msg[5:]
			if c.linkRepo == nil {
				return
			}
			link, err := c.linkRepo.FindByKey(ctx, eventType)
			if err != nil {
				c.mu.Lock()
				delete(c.links, eventType)
				c.mu.Unlock()
				return
			}
			c.mu.Lock()
			c.links[link.EventType] = link
			c.mu.Unlock()
		}
	})
	if err != nil && ctx.Err() == nil && c.logger != nil {
		c.logger.Errorw("config cache subscription ended unexpectedly", "error", err)
	}
}
