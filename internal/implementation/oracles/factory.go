package oracles

import (
	"fmt"
	"strings"
	"sync"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

type Factory struct {
	providers       map[string]ports.Oracle
	defaultProvider string
	mu              sync.RWMutex
}

func NewFactory() ports.OracleFactory {
	return &Factory{
		providers: make(map[string]ports.Oracle),
	}
}

func (f *Factory) RegisterProvider(name string, provider ports.Oracle) error {
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	f.providers[name] = provider

	// First registered provider becomes the default automatically.
	if f.defaultProvider == "" {
		f.defaultProvider = name
	}

	return nil
}

func (f *Factory) GetProvider(name string) (ports.Oracle, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	provider, ok := f.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return provider, nil
}

func (f *Factory) GetDefaultProvider() (ports.Oracle, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.defaultProvider == "" {
		return nil, fmt.Errorf("no default provider configured")
	}

	provider, ok := f.providers[f.defaultProvider]
	if !ok {
		return nil, fmt.Errorf("default provider %s not found", f.defaultProvider)
	}

	return provider, nil
}

func (f *Factory) SetDefaultProvider(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.providers[name]; !ok {
		return fmt.Errorf("provider %s not registered", name)
	}

	f.defaultProvider = name
	return nil
}

func (f *Factory) ListProviders() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	names := make([]string, 0, len(f.providers))
	for name := range f.providers {
		names = append(names, name)
	}
	return names
}

func (f *Factory) ListAvailableProviders() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	names := make([]string, 0, len(f.providers))
	for name, provider := range f.providers {
		if provider.IsAvailable() {
			names = append(names, name)
		}
	}
	return names
}

// GetProviderForModel returns the first provider that lists the model, then falls back to
// prefix-based matching (gpt-*/o1-* → openai, claude-* → anthropic, gemini-* → gemini),
// and finally the default provider.
func (f *Factory) GetProviderForModel(model string) (ports.Oracle, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	modelLower := strings.ToLower(model)

	for _, provider := range f.providers {
		for _, availableModel := range provider.GetAvailableModels() {
			if strings.ToLower(availableModel) == modelLower {
				return provider, nil
			}
		}
	}

	if strings.HasPrefix(modelLower, "gpt-") || strings.HasPrefix(modelLower, "o1-") {
		if provider, ok := f.providers["openai"]; ok {
			return provider, nil
		}
	}
	if strings.HasPrefix(modelLower, "claude-") {
		if provider, ok := f.providers["anthropic"]; ok {
			return provider, nil
		}
	}
	if strings.HasPrefix(modelLower, "gemini-") {
		if provider, ok := f.providers["gemini"]; ok {
			return provider, nil
		}
	}

	if f.defaultProvider != "" {
		if provider, ok := f.providers[f.defaultProvider]; ok {
			return provider, nil
		}
	}

	return nil, fmt.Errorf("no provider found for model: %s", model)
}
