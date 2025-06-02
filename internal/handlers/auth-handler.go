package handlers

import (
	"net/http"
	"time"

	"devops-assessment/internal/auth"
	"devops-assessment/internal/models"

	"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	authService *auth.AuthService
	userService *models.UserService
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(authService *auth.AuthService, userService *models.UserService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		userService: userService,
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
}

// ChangePasswordRequest represents a password change request
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// Login handles user login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Authenticate user
	session, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		if err == auth.ErrInvalidCredentials {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}
		if err == auth.ErrUserInactive {
			c.JSON(http.StatusForbidden, gin.H{"error": "User account is inactive"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Login failed"})
		return
	}

	// Set session cookie
	c.SetCookie(
		"session_token",
		session.SessionToken,
		int(auth.SessionDuration.Seconds()),
		"/",
		"",   // domain
		true, // secure (HTTPS only)
		true, // httpOnly
	)

	// Load user teams
	teams, _ := h.userService.GetUserTeams(session.User.ID)
	session.User.Teams = teams

	// Load user groups
	groups, _ := h.userService.GetUserGroups(session.User.ID)
	session.User.Groups = groups

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"user":    session.User,
		"session": gin.H{
			"token":      session.SessionToken,
			"expires_at": session.ExpiresAt,
		},
	})
}

// Logout handles user logout
func (h *AuthHandler) Logout(c *gin.Context) {
	// Get session from context
	session, err := auth.GetCurrentSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Delete session
	if err := h.authService.DeleteSession(session.SessionToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Logout failed"})
		return
	}

	// Clear cookie
	c.SetCookie(
		"session_token",
		"",
		-1,
		"/",
		"",
		true,
		true,
	)

	c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
}

// Register handles user registration (admin only)
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create user
	user := &models.User{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		IsActive:  true,
	}

	if err := h.userService.CreateUser(user, req.Password); err != nil {
		if err == models.ErrEmailAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"user":    user,
	})
}

// GetCurrentUser returns the current authenticated user
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Load user teams
	teams, _ := h.userService.GetUserTeams(user.ID)
	user.Teams = teams

	// Load user groups
	groups, _ := h.userService.GetUserGroups(user.ID)
	user.Groups = groups

	c.JSON(http.StatusOK, user)
}

// ChangePassword handles password change for the current user
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get current user
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Change password
	if err := h.authService.ChangePassword(user.ID, req.OldPassword, req.NewPassword); err != nil {
		if err == auth.ErrInvalidCredentials {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid old password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change password"})
		return
	}

	// Clear cookie (user needs to login again)
	c.SetCookie(
		"session_token",
		"",
		-1,
		"/",
		"",
		true,
		true,
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "Password changed successfully. Please login again.",
	})
}

// RefreshSession extends the current session
func (h *AuthHandler) RefreshSession(c *gin.Context) {
	session, err := auth.GetCurrentSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Extend session
	if err := h.authService.ExtendSession(session.SessionToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh session"})
		return
	}

	// Update cookie
	c.SetCookie(
		"session_token",
		session.SessionToken,
		int(auth.SessionDuration.Seconds()),
		"/",
		"",
		true,
		true,
	)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Session refreshed",
		"expires_at": time.Now().Add(auth.SessionDuration),
	})
}

// GetSessions returns all active sessions for the current user
func (h *AuthHandler) GetSessions(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	sessions, err := h.authService.GetUserActiveSessions(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get sessions"})
		return
	}

	c.JSON(http.StatusOK, sessions)
}

// RevokeAllSessions revokes all sessions for the current user
func (h *AuthHandler) RevokeAllSessions(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Delete all user sessions
	if err := h.authService.DeleteUserSessions(user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke sessions"})
		return
	}

	// Clear cookie
	c.SetCookie(
		"session_token",
		"",
		-1,
		"/",
		"",
		true,
		true,
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "All sessions revoked. Please login again.",
	})
}

// RegisterRoutes registers authentication routes
func (h *AuthHandler) RegisterRoutes(router *gin.RouterGroup, middleware *auth.Middleware) {
	auth := router.Group("/auth")
	{
		// Public routes
		auth.POST("/login", h.Login)
		auth.POST("/logout", h.Logout)

		// Protected routes
		protected := auth.Group("")
		protected.Use(middleware.RequireAuth())
		{
			protected.GET("/me", h.GetCurrentUser)
			protected.POST("/change-password", h.ChangePassword)
			protected.POST("/refresh", h.RefreshSession)
			protected.GET("/sessions", h.GetSessions)
			protected.POST("/revoke-all", h.RevokeAllSessions)
		}

		// Admin routes
		admin := auth.Group("")
		admin.Use(middleware.RequireAuth(), middleware.RequireAdmin())
		{
			admin.POST("/register", h.Register)
		}
	}
}
