package server

import (
	"context"
	"fmt"
	"time"

	"github.com/1827mk/app-commons/conf"
	"github.com/1827mk/app-server/datastore"
	mid "github.com/1827mk/app-server/middleware"
	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

type Server struct {
	Echo     *echo.Echo
	Cfg      *conf.Config
	Database *datastore.DBStore
	Redis    *datastore.RedisClient
}

func NewServer(cfg *conf.Config) (*Server, error) {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Initialize database
	db, err := datastore.NewPostgresDB(&datastore.DBConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		Scripts:  cfg.Database.Scripts,
	})
	if err != nil {
		return nil, fmt.Errorf("database initialization failed: %v", err)
	}

	// Initialize Redis
	rdb, err := datastore.NewRedisClient(&datastore.RedisConfig{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		return nil, fmt.Errorf("redis initialization failed: %v", err)
	}

	// Basic middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(middleware.RequestID())

	// Configure JWT middleware
	jwtConfig := echojwt.Config{
		SigningKey:    []byte(cfg.JWT.Secret),
		TokenLookup:   "header:Authorization:Bearer",
		SigningMethod: "HS256",
		ContextKey:    "user",
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return new(mid.JWTCustomClaims)
		},
		Skipper: func(c echo.Context) bool {
			publicPaths := map[string]struct{}{
				"/api/v1/auth/login":           {},
				"/api/v1/auth/register":        {},
				"/api/v1/auth/refresh":         {},
				"/api/v1/auth/forgot-password": {},
				"/api/v1/auth/reset-password":  {},
				"/health":                      {},
				"/metrics":                     {},
			}

			path := c.Path()
			_, ok := publicPaths[path]
			if ok {
				c.Logger().Infof("Skipping JWT authentication for: %s", path)
			}
			return ok
		},
		ErrorHandler: func(c echo.Context, err error) error {
			if err.Error() == "Missing or malformed JWT" {
				return c.JSON(401, map[string]interface{}{
					"code":    401,
					"message": "missing or malformed token",
					"error":   "authorization header is required",
				})
			}
			return c.JSON(401, map[string]interface{}{
				"code":    401,
				"message": "invalid or expired token",
				"error":   err.Error(),
			})
		},
	}

	// Apply JWT middleware
	e.Use(echojwt.WithConfig(jwtConfig))

	// Configure rate limiting if enabled
	if cfg.Server.RateLimit {
		e.Use(middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
			Store: middleware.NewRateLimiterMemoryStoreWithConfig(
				middleware.RateLimiterMemoryStoreConfig{
					Rate:      rate.Limit(cfg.Auth.RateLimit.LoginAttempts),
					Burst:     int(cfg.Auth.RateLimit.LoginAttempts),
					ExpiresIn: time.Duration(cfg.Auth.RateLimit.WindowMinutes) * time.Minute,
				},
			),
			IdentifierExtractor: func(ctx echo.Context) (string, error) {
				return ctx.RealIP(), nil
			},
			ErrorHandler: func(ctx echo.Context, err error) error {
				return ctx.JSON(429, map[string]interface{}{
					"code":    429,
					"message": "too many requests",
					"error":   err.Error(),
				})
			},
		}))
	}

	// Security headers middleware
	e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		HSTSMaxAge:            31536000,
		HSTSExcludeSubdomains: false,
		ContentSecurityPolicy: "default-src 'self'",
	}))

	// Configure server timeouts
	e.Server.ReadTimeout = time.Duration(cfg.Server.ReadTimeout) * time.Second
	e.Server.WriteTimeout = time.Duration(cfg.Server.WriteTimeout) * time.Second

	// Initialize server with all components
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
