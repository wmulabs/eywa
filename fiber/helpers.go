package fiber

import "go.uber.org/zap"

func newLogger() *zap.SugaredLogger {
	l, _ := zap.NewProduction()
	return l.Sugar()
}
