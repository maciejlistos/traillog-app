package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// RequestIDHeader is the HTTP header used to propagate request identifiers.
	RequestIDHeader = "X-Request-ID"
	requestIDKey    = "request_id"
)

type requestIDContextKey struct{}

var fallbackSequence atomic.Uint64

// RequestIDGenerator creates request identifier values.
type RequestIDGenerator func() string

// RequestID returns middleware that propagates or generates a request identifier.
func RequestID(generator RequestIDGenerator) gin.HandlerFunc {
	if generator == nil {
		generator = NewRequestID
	}

	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(RequestIDHeader))

		if requestID == "" {
			requestID = generator()
		}

		c.Set(requestIDKey, requestID)
		c.Header(RequestIDHeader, requestID)

		requestContext := context.WithValue(
			c.Request.Context(),
			requestIDContextKey{},
			requestID,
		)

		c.Request = c.Request.WithContext(requestContext)

		c.Next()
	}
}

// RequestIDFronContext returns the request identifier stored in ctx.
func RequestIDFronContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

// RequestIDFromGin returns the request identifier stored in c.
func RequestIDFromGin(c *gin.Context) string {
	requestID, _ := c.Get(requestIDKey)
	value, _ := requestID.(string)

	return value
}

// NewRequestID returns a cryptographically random request identifier when possible.
func NewRequestID() string {
	randomBytes := make([]byte, 16)

	if _, err := rand.Read(randomBytes); err == nil {
		return hex.EncodeToString(randomBytes)
	}

	sequence := fallbackSequence.Add(1)

	return fmt.Sprintf(
		"%x-%x",
		time.Now().UnixNano(),
		sequence,
	)
}
