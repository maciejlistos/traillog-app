package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery converts panics into internal-server-error responses and logs their stacks.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			logger.ErrorContext(
				c.Request.Context(),
				"panic recovered",
				"request_id",
				RequestIDFromGin(c),
				"method",
				c.Request.Method,
				"path",
				c.Request.URL.Path,
				"panic",
				fmt.Sprint(recovered),
				"stack",
				string(debug.Stack()),
			)

			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				gin.H{
					"error":      "internal server error",
					"request_id": RequestIDFromGin(c),
				},
			)
		}()

		c.Next()
	}
}
