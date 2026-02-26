package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// userContextKey is the gin context key for storing the authenticated user context.
const userContextKey = "user_context"

// AuthMiddleware extracts user identity from request headers and injects
// a UserContext into the gin context. Required headers: X-User-ID, X-Tenant-ID.
// X-User-Roles is optional and expects a comma-separated list of roles.
//
// NOTE: This middleware trusts the provided headers without cryptographic
// verification. In production, these headers should be set exclusively by a
// trusted upstream component (e.g., API gateway or auth service) that has
// already authenticated the request.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		tenantID := c.GetHeader("X-Tenant-ID")

		if userID == "" || tenantID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "X-User-ID and X-Tenant-ID headers are required",
				},
			})
			c.Abort()
			return
		}

		roles := parseRoles(c.GetHeader("X-User-Roles"))

		uc := &UserContext{
			UserID:   userID,
			TenantID: tenantID,
			Roles:    roles,
		}

		c.Set(userContextKey, uc)
		c.Next()
	}
}

// GetUserContext retrieves the UserContext stored in the gin context.
func GetUserContext(c *gin.Context) (*UserContext, bool) {
	val, exists := c.Get(userContextKey)
	if !exists {
		return nil, false
	}
	uc, ok := val.(*UserContext)
	return uc, ok
}

// parseRoles converts a comma-separated role string into a slice of Role values.
func parseRoles(header string) []Role {
	if header == "" {
		return nil
	}
	parts := strings.Split(header, ",")
	roles := make([]Role, 0, len(parts))
	for _, p := range parts {
		r := Role(strings.TrimSpace(p))
		if r != "" {
			roles = append(roles, r)
		}
	}
	return roles
}
