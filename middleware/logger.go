package middleware

// import (
// 	"time"

// 	"github.com/1827mk/app-server/logger"

// 	"github.com/labstack/echo/v4"
// 	"go.uber.org/zap"
// )

// func Logger(next echo.HandlerFunc) echo.HandlerFunc {
// 	return func(c echo.Context) error {
// 		start := time.Now()
// 		req := c.Request()
// 		res := c.Response()

// 		err := next(c)
// 		stop := time.Since(start).Round(time.Millisecond)

// 		logger.Logger().Info("Request handled",
// 			zap.String("method", req.Method),
// 			zap.String("uri", req.URL.Path),
// 			zap.Int("status", res.Status),
// 			zap.Duration("latency", stop),
// 		)
// 		return err
// 	}
// }
