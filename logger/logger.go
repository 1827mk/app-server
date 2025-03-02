package logger

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
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

func GetServiceErrorLocation() string {
	stacktrace := make([]uintptr, 50)
	length := runtime.Callers(3, stacktrace[:])
	frames := runtime.CallersFrames(stacktrace[:length])

	var locations []string
	for {
		frame, more := frames.Next()
		if !more {
			break
		}

		// Skip common framework and runtime paths
		if strings.Contains(frame.File, "runtime/") ||
			strings.Contains(frame.File, "/go/pkg/mod/") ||
			strings.Contains(frame.File, "vendor/") {
			continue
		}

		// Get relative path from project root
		if projectPath := strings.Index(frame.File, "super-app"); projectPath != -1 {
			relativePath := frame.File[projectPath:]
			locations = append(locations, fmt.Sprintf("%s:%d", relativePath, frame.Line))
		}
	}

	// Return the most relevant location or unknown
	if len(locations) > 0 {
		return locations[0]
	}
	return "unknown location"
}

func ZapLoggerMiddleware(log *zap.Logger) echo.MiddlewareFunc {
	if log == nil {
		log = Logger()
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("%v", r)
					}

					errorLocation := GetServiceErrorLocation()

					// Log with service-specific location
					log.Error("Service error occurred",
						zap.String("service", "isync-service"),
						zap.String("location", errorLocation),
						zap.String("error", err.Error()),
						zap.String("method", c.Request().Method),
						zap.String("path", c.Request().URL.Path),
					)

					c.JSON(http.StatusInternalServerError, ErrorResponse{
						Success: false,
						Errors: []map[string]string{{
							"code":     "internal_error",
							"message":  "Internal server error",
							"location": errorLocation,
						}},
						Message: "An unexpected error occurred",
					})
				}
			}()

			err := next(c)

			// Log the request
			req := c.Request()
			res := c.Response()

			fields := []zap.Field{
				zap.String("method", req.Method),
				zap.String("path", req.URL.Path),
				zap.Int("status", res.Status),
				zap.Duration("latency", time.Since(start)),
			}

			if err != nil {
				fields = append(fields, zap.Error(err))
				log.Error("Request failed", fields...)
				return err
			}

			log.Info("Request completed", fields...)
			return nil
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
