package handlers

import (
	"net/http"
	"strconv"

	"devops-assessment/internal/auth"
	"devops-assessment/internal/models"

	"github.com/gin-gonic/gin"
)

// UserHandler handles user management endpoints
type UserHandler struct {
	userService *models.UserService
	roleService *models.RoleService
	authService *auth.AuthService
}

// NewUserHandler creates a new user handler
func NewUserHandler(
	userService *models.UserService,
	roleService *models.RoleService,
	authService *auth.AuthService,
) *UserHandler {
	return &UserHandler{
		userService: userService,
		roleService: roleService,
		authService: authService,
	}
}

// CreateUserRequest represents a request to create a user
type CreateUserRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	IsActive  bool   `json:"is_active"`
}

// UpdateUserRequest represents a request to update a user
type UpdateUserRequest struct {
	Email     string `json:"email" binding:"omitempty,email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	IsActive  *bool  `json:"is_active"`
}

// ResetPasswordRequest represents a request to reset a user's password
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// AddToTeamRequest represents a request to add a user to a team
type AddToTeamRequest struct {
	TeamID int `json:"team_id" binding:"required"`
	RoleID int `json:"role_id" binding:"required"`
}

// AddToGroupRequest represents a request to add a user to a group
type AddToGroupRequest struct {
	GroupID int `json:"group_id" binding:"required"`
	RoleID  int `json:"role_id" binding:"required"`
}

// ListUsers lists all users with pagination
func (h *UserHandler) ListUsers(c *gin.Context) {
	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	activeOnly := c.Query("active_only") == "true"

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	// Get users
	users, totalCount, err := h.userService.ListUsers(offset, limit, activeOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total_count": totalCount,
			"total_pages": (totalCount + limit - 1) / limit,
		},
	})
}

// GetUser retrieves a specific user
func (h *UserHandler) GetUser(c *gin.Context) {
	// Get user ID from URL
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get user
	user := &models.User{}
	if err := h.userService.GetUserByID(userID, user); err != nil {
		if err == models.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Load user teams and groups
	teams, _ := h.userService.GetUserTeams(userID)
	user.Teams = teams

	groups, _ := h.userService.GetUserGroups(userID)
	user.Groups = groups

	c.JSON(http.StatusOK, user)
}

// CreateUser creates a new user
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create user
	user := &models.User{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		IsActive:  req.IsActive,
	}

	if err := h.userService.CreateUser(user, req.Password); err != nil {
		if err == models.ErrEmailAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Store user ID for audit logging
	c.Set("resourceID", user.ID)

	c.JSON(http.StatusCreated, user)
}

// UpdateUser updates a user
func (h *UserHandler) UpdateUser(c *gin.Context) {
	// Get user ID from URL
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing user
	user := &models.User{}
	if err := h.userService.GetUserByID(userID, user); err != nil {
		if err == models.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Update fields
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	// Save updates
	if err := h.userService.UpdateUser(user); err != nil {
		if err == models.ErrEmailAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	// Store user ID for audit logging
	c.Set("resourceID", user.ID)

	c.JSON(http.StatusOK, user)
}

// DeleteUser deactivates a user
func (h *UserHandler) DeleteUser(c *gin.Context) {
	// Get user ID from URL
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Check if user is trying to delete themselves
	currentUser, _ := auth.GetCurrentUser(c)
	if currentUser != nil && currentUser.ID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete your own account"})
		return
	}

	// Delete (deactivate) user
	if err := h.userService.DeleteUser(userID); err != nil {
		if err == models.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	// Invalidate all user sessions
	h.authService.DeleteUserSessions(userID)

	// Store user ID for audit logging
	c.Set("resourceID", userID)

	c.JSON(http.StatusOK, gin.H{"message": "User deactivated successfully"})
}

// ResetPassword resets a user's password
func (h *UserHandler) ResetPassword(c *gin.Context) {
	// Get user ID from URL
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Reset password
	if err := h.authService.ResetPassword(userID, req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset password"})
		return
	}

	// Store user ID for audit logging
	c.Set("resourceID", userID)

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

// GetUserTeams gets all teams a user belongs to
func (h *UserHandler) GetUserTeams(c *gin.Context) {
	// Get user ID from URL
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get user teams
	teams, err := h.userService.GetUserTeams(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user teams"})
		return
	}

	c.JSON(http.StatusOK, teams)
}

// AddUserToTeam adds a user to a team
func (h *UserHandler) AddUserToTeam(c *gin.Context) {
	// Get user ID from URL
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req AddToTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Add user to team
	if err := h.userService.AddUserToTeam(userID, req.TeamID, req.RoleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add user to team"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User added to team successfully"})
}

// RemoveUserFromTeam removes a user from a team
func (h *UserHandler) RemoveUserFromTeam(c *gin.Context) {
	// Get user ID and team ID from URL
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	teamID, err := strconv.Atoi(c.Param("teamId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	// Remove user from team
	if err := h.userService.RemoveUserFromTeam(userID, teamID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove user from team"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User removed from team successfully"})
}

// GetUserGroups gets all groups a user belongs to
func (h *UserHandler) GetUserGroups(c *gin.Context) {
	// Get user ID from URL
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get user groups
	groups, err := h.userService.GetUserGroups(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user groups"})
		return
	}

	c.JSON(http.StatusOK, groups)
}

// GetRoles gets all available roles
func (h *UserHandler) GetRoles(c *gin.Context) {
	roles, err := h.roleService.ListRoles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get roles"})
		return
	}

	c.JSON(http.StatusOK, roles)
}

// RegisterRoutes registers user management routes
func (h *UserHandler) RegisterRoutes(router *gin.RouterGroup, middleware *auth.Middleware) {
	users := router.Group("/users")
	users.Use(middleware.RequireAuth())
	{
		// User management (admin only)
		admin := users.Group("")
		admin.Use(middleware.RequirePermission(models.ResourceUser, models.ActionRead))
		{
			admin.GET("", h.ListUsers)
			admin.GET("/:id", h.GetUser)
			admin.GET("/:id/teams", h.GetUserTeams)
			admin.GET("/:id/groups", h.GetUserGroups)
		}

		// User modification (admin only)
		adminWrite := users.Group("")
		adminWrite.Use(middleware.RequirePermission(models.ResourceUser, models.ActionCreate))
		{
			adminWrite.POST("", middleware.AuditLog("create_user", "user"), h.CreateUser)
		}

		adminUpdate := users.Group("")
		adminUpdate.Use(middleware.RequirePermission(models.ResourceUser, models.ActionUpdate))
		{
			adminUpdate.PUT("/:id", middleware.AuditLog("update_user", "user"), h.UpdateUser)
			adminUpdate.POST("/:id/reset-password", middleware.AuditLog("reset_password", "user"), h.ResetPassword)
			adminUpdate.POST("/:id/teams", middleware.AuditLog("add_user_to_team", "user"), h.AddUserToTeam)
			adminUpdate.DELETE("/:id/teams/:teamId", middleware.AuditLog("remove_user_from_team", "user"), h.RemoveUserFromTeam)
		}

		adminDelete := users.Group("")
		adminDelete.Use(middleware.RequirePermission(models.ResourceUser, models.ActionDelete))
		{
			adminDelete.DELETE("/:id", middleware.AuditLog("delete_user", "user"), h.DeleteUser)
		}

		// Roles endpoint (available to all authenticated users)
		users.GET("/roles", h.GetRoles)
	}
}
