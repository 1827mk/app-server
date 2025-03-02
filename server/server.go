package server

import (
	"context"
	"fmt"
	"time"

	"github.com/1827mk/app-commons/conf"
	"github.com/1827mk/app-server/datastore"
	"github.com/1827mk/app-server/logger"
	appMiddleware "github.com/1827mk/app-server/middleware"
	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type Server struct {
	Echo     *echo.Echo
	Cfg      *conf.Config
	Database *datastore.DBStore
	Redis    *datastore.RedisClient
}

// JWTClaims defines the structure for JWT token claims
type JWTClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Pre-configured logger
var log *zap.Logger

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
	//e.Use(middleware.Logger())
	e.Use(logger.ZapLoggerMiddleware(log))
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(middleware.RequestID())

	// Make logger available in context
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("logger", log)
			return next(c)
		}
	})

	// Configure JWT middleware
	configureJWTMiddleware(e, cfg)

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
	e.Binder = &appMiddleware.CustomBinder{DefaultBinder: &echo.DefaultBinder{}}

	// Initialize server with all components
	server := &Server{
		Echo:     e,
		Cfg:      cfg,
		Database: &datastore.DBStore{DB: db},
		Redis:    rdb,
	}

	return server, nil
}

// configureJWTMiddleware sets up the JWT middleware
func configureJWTMiddleware(e *echo.Echo, cfg *conf.Config) {
	// Create a JWT middleware group for protected routes
	jwtGroup := e.Group("/api")

	// Configure JWT middleware
	jwtConfig := echojwt.Config{
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return new(JWTClaims)
		},
		SigningKey:    []byte(cfg.JWT.Secret),
		SigningMethod: "HS256",
		TokenLookup:   "header:Authorization:Bearer ",
		ErrorHandler: func(c echo.Context, err error) error {
			return c.JSON(401, map[string]interface{}{
				"code":    401,
				"message": "unauthorized",
				"error":   err.Error(),
			})
		},
	}

	// Apply JWT middleware to protected routes
	jwtGroup.Use(echojwt.WithConfig(jwtConfig))
}

// GenerateJWTToken creates a new JWT token for a user
func (s *Server) GenerateJWTToken(userID uint, username, role string) (string, error) {
	// Set expiry time based on configuration
	expiryTime := time.Now().Add(time.Duration(s.Cfg.JWT.AccessExpiry) * time.Minute)

	// Create claims
	claims := &JWTClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiryTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    s.Cfg.JWT.Issuer,
			Subject:   username,
			Audience:  []string{s.Cfg.JWT.Audience},
		},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate encoded token
	tokenString, err := token.SignedString([]byte(s.Cfg.JWT.Secret))
	if err != nil {
		return "", fmt.Errorf("failed to generate JWT token: %w", err)
	}

	return tokenString, nil
}

// GenerateRefreshToken creates a new refresh token
func (s *Server) GenerateRefreshToken(userID uint) (string, error) {
	// Generate a unique refresh token
	refreshToken := jwt.New(jwt.SigningMethodHS256)

	// Set claims
	claims := refreshToken.Claims.(jwt.MapClaims)
	claims["user_id"] = userID
	claims["exp"] = time.Now().Add(time.Duration(s.Cfg.JWT.RefreshExpiry) * 24 * time.Hour).Unix()
	claims["token_type"] = "refresh"

	// Generate encoded token
	tokenString, err := refreshToken.SignedString([]byte(s.Cfg.JWT.Secret))
	if err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Store refresh token in Redis with expiry
	ctx := context.Background()
	err = s.Redis.Client.Set(
		ctx,
		fmt.Sprintf("refresh_token:%d", userID),
		tokenString,
		time.Duration(s.Cfg.JWT.RefreshExpiry)*24*time.Hour,
	).Err()
	if err != nil {
		return "", fmt.Errorf("failed to store refresh token: %w", err)
	}

	return tokenString, nil
}

// ValidateRefreshToken validates a refresh token
func (s *Server) ValidateRefreshToken(tokenString string) (uint, error) {
	// Parse the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.Cfg.JWT.Secret), nil
	})

	if err != nil {
		return 0, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Verify token is valid
	if !token.Valid {
		return 0, fmt.Errorf("invalid refresh token")
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, fmt.Errorf("invalid token claims")
	}

	// Check token type
	tokenType, ok := claims["token_type"].(string)
	if !ok || tokenType != "refresh" {
		return 0, fmt.Errorf("invalid token type")
	}

	// Get user ID from claims
	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid user ID in token")
	}
	userID := uint(userIDFloat)

	// Verify against stored token in Redis
	ctx := context.Background()
	storedToken, err := s.Redis.Client.Get(ctx, fmt.Sprintf("refresh_token:%d", userID)).Result()
	if err != nil {
		return 0, fmt.Errorf("refresh token not found: %w", err)
	}

	if storedToken != tokenString {
		return 0, fmt.Errorf("refresh token has been revoked")
	}

	return userID, nil
}

// RevokeRefreshToken invalidates a refresh token
func (s *Server) RevokeRefreshToken(userID uint) error {
	ctx := context.Background()
	err := s.Redis.Client.Del(ctx, fmt.Sprintf("refresh_token:%d", userID)).Err()
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

// type ServiceValidator struct {
// 	validator *validator.Validate
// }

// func (cv *ServiceValidator) Validate(i interface{}) error {
// 	if err := cv.validator.Struct(i); err != nil {
// 		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
// 	}
// 	return nil
// }

func (s *Server) Start() error {
	return s.Echo.Start(fmt.Sprintf(":%v", s.Cfg.Server.Port))
}

func (s *Server) Stop(ctx context.Context) error {
	return s.Echo.Shutdown(ctx)
}
