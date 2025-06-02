package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/dsgthb/devops-assessment/internal/models"
	"github.com/gin-gonic/gin"
)

// ContextKey type for context keys
type ContextKey string

const (
	// UserContextKey is the key for storing user in context
	UserContextKey ContextKey = "user"
	// SessionContextKey is the key for storing session in context
	SessionContextKey ContextKey = "session"
)

// Middleware handles authentication and authorization
type Middleware struct {
	authService *AuthService
	rbacService *models.RBACService
}

// NewMiddleware creates a new authentication middleware
func NewMiddleware(authService *AuthService, rbacService *models.RBACService) *Middleware {
	return &Middleware{
		authService: authService,
		rbacService: rbacService,
	}
}

// RequireAuth ensures the user is authenticated
func (m *Middleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from cookie or header
		token := m.getToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		// Validate session
		session, err := m.authService.ValidateSession(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session"})
			c.Abort()
			return
		}

		// Store user and session in context
		c.Set(string(UserContextKey), session.User)
		c.Set(string(SessionContextKey), session)

		c.Next()
	}
}

// RequirePermission ensures the user has a specific permission
func (m *Middleware) RequirePermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		user, exists := c.Get(string(UserContextKey))
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		userModel := user.(*models.User)

		// Check permission
		hasPermission, err := m.rbacService.CheckUserPermission(userModel.ID, resource, action)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
			c.Abort()
			return
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireRole ensures the user has a specific role
func (m *Middleware) RequireRole(roleName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		user, exists := c.Get(string(UserContextKey))
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		userModel := user.(*models.User)

		// Get user roles
		roles, err := m.rbacService.GetUserRoles(userModel.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check roles"})
			c.Abort()
			return
		}

		// Check if user has the required role
		hasRole := false
		for _, role := range roles {
			if role.Name == roleName {
				hasRole = true
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAdmin ensures the user has admin role
func (m *Middleware) RequireAdmin() gin.HandlerFunc {
	return m.RequireRole(models.RoleAdmin)
}

// RequireTeamAccess ensures the user has access to a specific team
func (m *Middleware) RequireTeamAccess(teamParamName string, requiredPermission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		user, exists := c.Get(string(UserContextKey))
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		userModel := user.(*models.User)

		// Get team ID from URL parameter
		teamIDStr := c.Param(teamParamName)
		var teamID int
		if _, err := fmt.Sscanf(teamIDStr, "%d", &teamID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
			c.Abort()
			return
		}

		// Parse the required permission
		parts := strings.Split(requiredPermission, ":")
		if len(parts) != 2 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid permission format"})
			c.Abort()
			return
		}
		resource, action := parts[0], parts[1]

		// Check if user has permission for this team
		hasPermission, err := m.rbacService.CheckTeamPermission(userModel.ID, teamID, resource, action)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check team permissions"})
			c.Abort()
			return
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions for this team"})
			c.Abort()
			return
		}

		// Store team ID in context for later use
		c.Set("teamID", teamID)

		c.Next()
	}
}

// OptionalAuth checks for authentication but doesn't require it
func (m *Middleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from cookie or header
		token := m.getToken(c)
		if token != "" {
			// Validate session
			session, err := m.authService.ValidateSession(token)
			if err == nil {
				// Store user and session in context
				c.Set(string(UserContextKey), session.User)
				c.Set(string(SessionContextKey), session)
			}
		}

		c.Next()
	}
}

// CORS middleware for handling Cross-Origin Resource Sharing
func (m *Middleware) CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// Helper functions

// getToken retrieves the session token from cookie or Authorization header
func (m *Middleware) getToken(c *gin.Context) string {
	// Try to get token from cookie first
	if cookie, err := c.Cookie("session_token"); err == nil {
		return cookie
	}

	// Try Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		// Expected format: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	return ""
}

// GetCurrentUser retrieves the current user from context
func GetCurrentUser(c *gin.Context) (*models.User, error) {
	user, exists := c.Get(string(UserContextKey))
	if !exists {
		return nil, fmt.Errorf("user not found in context")
	}

	userModel, ok := user.(*models.User)
	if !ok {
		return nil, fmt.Errorf("invalid user type in context")
	}

	return userModel, nil
}

// GetCurrentSession retrieves the current session from context
func GetCurrentSession(c *gin.Context) (*Session, error) {
	session, exists := c.Get(string(SessionContextKey))
	if !exists {
		return nil, fmt.Errorf("session not found in context")
	}

	sessionModel, ok := session.(*Session)
	if !ok {
		return nil, fmt.Errorf("invalid session type in context")
	}

	return sessionModel, nil
}

// AuditLog logs user actions for audit trail
func (m *Middleware) AuditLog(action string, resourceType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Execute the handler first
		c.Next()

		// Only log if the request was successful
		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			user, _ := GetCurrentUser(c)
			if user != nil {
				// Get resource ID from context or params
				var resourceID int
				if id, exists := c.Get("resourceID"); exists {
					resourceID = id.(int)
				}

				// Create audit log entry
				details := gin.H{
					"method":     c.Request.Method,
					"path":       c.Request.URL.Path,
					"status":     c.Writer.Status(),
					"user_agent": c.Request.UserAgent(),
				}

				detailsJSON, _ := json.Marshal(details)

				query := `
					INSERT INTO audit_logs (user_id, action, resource_type, resource_id, details, ip_address)
					VALUES (?, ?, ?, ?, ?, ?)
				`

				m.authService.db.Insert(query,
					user.ID,
					action,
					resourceType,
					resourceID,
					string(detailsJSON),
					c.ClientIP(),
				)
			}
		}
	}
}
