package server

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/1827mk/app-server/logger"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func (s *Server) Run() {
	go func() {
		if err := s.Start(); err != nil && err != echo.ErrServiceUnavailable {
			s.Echo.Logger.Fatalf("shutting down the server: %v", err)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	<-shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.Stop(ctx); err != nil {
		s.Echo.Logger.Fatalf("shutting down the server: %v", err)
	}

	if err := logger.Sync(); err != nil {
		zap.L().Error("Failed to sync zap logger", zap.Error(err))
	}
}
