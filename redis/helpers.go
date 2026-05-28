package redis

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newLogger() *zap.SugaredLogger {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.MessageKey = "message"
	cfg.EncoderConfig.LevelKey = "severity"
	cfg.EncoderConfig.TimeKey = "timestamp"
	log, _ := cfg.Build()
	return log.Sugar().Named("eywa-redis")
}
