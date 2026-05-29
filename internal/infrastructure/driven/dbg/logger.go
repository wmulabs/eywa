package dbg

import (
	"fmt"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	mu           sync.RWMutex
	globalLogger *zap.SugaredLogger
	loggerName   string
)

// InitializeLogger sets the default logger name. Call once at startup before the first GetLogger call.
func InitializeLogger(name string) {
	mu.Lock()
	loggerName = name
	globalLogger = nil
	mu.Unlock()
}

// CreateLogger builds a new production zap logger with the given name.
func CreateLogger(name string) *zap.SugaredLogger {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeCaller = zapcore.FullCallerEncoder
	cfg.EncoderConfig.MessageKey = "message"
	cfg.EncoderConfig.LevelKey = "severity"
	cfg.EncoderConfig.TimeKey = "timestamp"
	log, err := cfg.Build()
	if err != nil {
		panic(fmt.Sprintf("dbg: failed to build logger: %v", err))
	}
	return log.Sugar().Named(name)
}

// GetLogger returns the global logger, creating it on first call.
func GetLogger() *zap.SugaredLogger {
	mu.RLock()
	l := globalLogger
	mu.RUnlock()
	if l != nil {
		return l
	}
	mu.Lock()
	defer mu.Unlock()
	if globalLogger == nil {
		name := loggerName
		if name == "" {
			name = "eywa"
		}
		globalLogger = CreateLogger(name)
	}
	return globalLogger
}

// SetLogger replaces the global logger. Called by WeaveBuilder.Build so that
// actions and other components share the Weave's configured logger.
func SetLogger(l *zap.SugaredLogger) {
	mu.Lock()
	globalLogger = l
	mu.Unlock()
}
