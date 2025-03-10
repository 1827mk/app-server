package server

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
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
}
