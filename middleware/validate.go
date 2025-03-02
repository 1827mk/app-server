package middleware

import (
	"net/http"

	custom_response "github.com/1827mk/app-commons/app_response"
	"github.com/labstack/echo/v4"
)

// CustomBinder wraps Echo's default binder to add automatic validation
type CustomBinder struct {
	DefaultBinder echo.Binder
}

// Bind implements the Echo Binder interface with added validation
func (cb *CustomBinder) Bind(i interface{}, c echo.Context) error {
	// Call the default binder
	if err := cb.DefaultBinder.Bind(i, c); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]interface{}{
			"message": "Invalid request format",
			"error":   err.Error(),
		})
	}

	// Run Echo's validator if configured
	if c.Echo().Validator != nil {
		if err := c.Echo().Validator.Validate(i); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, map[string]interface{}{
				"message": "Validation failed",
				"error":   err.Error(),
			})
		}
	}

	// Check if the struct has a Validate method and call it
	if validator, ok := i.(interface{ Validate() error }); ok {
		if err := validator.Validate(); err != nil {
			// Handle custom_response.Errors properly
			if errs, ok := err.(*custom_response.Errors); ok {
				return echo.NewHTTPError(http.StatusBadRequest, map[string]interface{}{
					"message": "Validation failed",
					"errors":  errs.Error(),
				})
			}
			return echo.NewHTTPError(http.StatusBadRequest, map[string]interface{}{
				"message": "Validation failed",
				"error":   err.Error(),
			})
		}
	}

	return nil
}
