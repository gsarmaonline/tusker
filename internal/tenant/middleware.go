package tenant

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gsarma/tusker/internal/store"
)

const ctxKey = "tenant"

// AuthMiddleware validates the Bearer API key and sets the tenant in context.
func (s *Service) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing API key"})
			return
		}
		rawKey := strings.TrimPrefix(header, "Bearer ")

		t, err := s.GetByAPIKey(c.Request.Context(), rawKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}

		c.Set(ctxKey, t)
		c.Next()
	}
}

// FromContext retrieves the authenticated tenant from the Gin context.
func FromContext(c *gin.Context) *store.Tenant {
	t, _ := c.Get(ctxKey)
	tenant, _ := t.(*store.Tenant)
	return tenant
}
