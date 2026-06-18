package middleware

import (
	"subber/internal/requestid"

	"github.com/gin-gonic/gin"
)

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := requestid.Normalize(c.GetHeader(requestid.Header))
		c.Writer.Header().Set(requestid.Header, id)
		c.Request = c.Request.WithContext(requestid.WithContext(c.Request.Context(), id))
		c.Next()
	}
}
