package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func Recover(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		defer func() {
			if r := recover(); r != nil {
				c.Error(echo.NewHTTPError(http.StatusInternalServerError, "Internal server error"))
			}
		}()
		return next(c)
	}
}
