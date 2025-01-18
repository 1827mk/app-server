package server

import (
	"context"
	"fmt"
	"time"

	"github.com/1827mk/app-commons/conf"
	"github.com/1827mk/app-server/datastore"
	"github.com/1827mk/app-server/middleware"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
)

type Server struct {
	Echo     *echo.Echo
	Cfg      *conf.Config
	Database *datastore.DBStore
	Redis    *datastore.RedisClient
}

func NewServer(cfg *conf.Config) (*Server, error) {
	e := echo.New()
	db, err := datastore.NewPostgresDB(&datastore.DBConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		Scripts:  cfg.Database.Scripts,
	})
	if err != nil {
		return nil, err
	}

	rdb, err := datastore.NewRedisClient(&datastore.RedisConfig{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		return nil, err
	}

	e.HideBanner = true
	e.Use(middleware.Logger)
	e.Use(middleware.Recover)
	if cfg.Server.RateLimit {
		e.Use(middleware.RateLimiter(rdb.Client, int64(cfg.Server.ReadTimeout), time.Minute))
	}
	e.Use(echojwt.WithConfig(echojwt.Config{
		Skipper: func(c echo.Context) bool {
			return c.Path() == "/login"
		},
		SigningKey: []byte(cfg.JWT.Secret),
	}))

	server := &Server{
		Echo:     e,
		Cfg:      cfg,
		Database: &datastore.DBStore{DB: db},
		Redis:    rdb,
	}

	return server, nil
}

func (s *Server) Start() error {
	return s.Echo.Start(fmt.Sprintf(":%v", s.Cfg.Server.Port))
}

func (s *Server) Stop(ctx context.Context) error {
	return s.Echo.Shutdown(ctx)
}
