package mongo

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newLogger() *zap.SugaredLogger {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.MessageKey = "message"
	cfg.EncoderConfig.LevelKey = "severity"
	cfg.EncoderConfig.TimeKey = "timestamp"
	log, err := cfg.Build()
	if err != nil {
		panic(fmt.Sprintf("mongo: failed to build logger: %v", err))
	}
	return log.Sugar().Named("eywa-mongo")
}
