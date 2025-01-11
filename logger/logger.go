package logger

import (
	"go.uber.org/zap"
)

var log *zap.Logger

func init() {
	var err error
	log, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
}

func Logger() *zap.Logger {
	return log
}

func Sync() error {
	return log.Sync()
}
