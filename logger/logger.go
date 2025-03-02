package logger

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	custom_response "github.com/1827mk/app-commons/app_response"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger

func init() {
	// Create logs directory if it doesn't exist
	err := os.MkdirAll("logs", 0755)
	if err != nil {
		panic(err)
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.StacktraceKey = "" // This removes stacktrace from output
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	config := zap.Config{
		Encoding:         "json",
		Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
		OutputPaths:      []string{"stdout", "logs/app.log"},
		ErrorOutputPaths: []string{"stderr", "logs/error.log"},
		EncoderConfig:    encoderConfig,
		InitialFields: map[string]interface{}{
			"app": "super-app",
		},
		DisableStacktrace: true, // This also helps remove stacktrace
	}

	log, err = config.Build(
		zap.AddCaller(),
		zap.AddCallerSkip(1),
	)
	if err != nil {
		panic(err)
	}
}

// Logger returns the global logger instance
func Logger() *zap.Logger {
	return log
}

func ZapLoggerMiddleware(log *zap.Logger) echo.MiddlewareFunc {
	// Check if logger is nil and use default logger if it is
	if log == nil {
		log = Logger()
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			req := c.Request()
			res := c.Response()

			fields := []zap.Field{
				zap.String("method", req.Method),
				zap.String("path", req.URL.Path),
				zap.String("remote_ip", c.RealIP()),
				zap.Int("status", res.Status),
				zap.Int64("size", res.Size),
				zap.String("user_agent", req.UserAgent()),
				zap.Duration("latency", time.Since(start)),
			}

			id := req.Header.Get(echo.HeaderXRequestID)
			if id == "" {
				id = res.Header().Get(echo.HeaderXRequestID)
			}
			if id != "" {
				fields = append(fields, zap.String("request_id", id))
			}

			n := res.Status
			switch {
			case n >= 500:
				log.Error("Server error", fields...)
			case n >= 400:
				log.Warn("Client error", fields...)
			case n >= 300:
				log.Info("Redirection", fields...)
			default:
				log.Info("Success", fields...)
			}

			return err
		}
	}
}

type ErrorResponse struct {
	Success bool                `json:"success"`
	Errors  []map[string]string `json:"errors,omitempty"`
	Message string              `json:"message"`
}

func Error(msg string, fields ...zap.Field) error {
	log.Error(msg, fields...)

	for _, field := range fields {
		if field.Key == "error" {
			// Handle binding errors that contain validation errors
			if he, ok := field.Interface.(*echo.HTTPError); ok {
				if errMap, ok := he.Message.(map[string]interface{}); ok {
					if validationErrs, ok := errMap["errors"].(string); ok {
						// Parse the validation errors from string
						var errs custom_response.Errors
						if err := json.Unmarshal([]byte(validationErrs), &errs); err == nil {
							errors := make([]map[string]string, len(errs))
							for i, err := range errs {
								errors[i] = map[string]string{
									"code":    err.Code,
									"message": err.Message,
								}
							}
							return echo.NewHTTPError(http.StatusBadRequest, ErrorResponse{
								Success: false,
								Errors:  errors,
								Message: msg,
							})
						}
					}
				}
			}

			// Handle custom validation errors directly
			if cerrs, ok := field.Interface.(*custom_response.Errors); ok {
				errors := make([]map[string]string, len(*cerrs))
				for i, err := range *cerrs {
					errors[i] = map[string]string{
						"code":    err.Code,
						"message": err.Message,
					}
				}
				return echo.NewHTTPError(http.StatusBadRequest, ErrorResponse{
					Success: false,
					Errors:  errors,
					Message: msg,
				})
			}

			// Handle regular errors
			errStr := field.Interface.(error).Error()
			return echo.NewHTTPError(http.StatusBadRequest, ErrorResponse{
				Success: false,
				Errors: []map[string]string{{
					"code":    "validation_error",
					"message": errStr,
				}},
				Message: msg,
			})
		}
	}

	return echo.NewHTTPError(http.StatusBadRequest, ErrorResponse{
		Success: false,
		Message: msg,
	})
}

// Add this helper function for consistent error field creation
func WithError(err error) zap.Field {
	return zap.Error(err)
}
