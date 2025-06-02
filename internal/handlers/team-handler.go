package handlers

import (
	"net/http"
	"strconv"

	"devops-assessment/internal/auth"
	"devops-assessment/internal/models"

	"github.com/gin-gonic/gin"
)

// TeamHandler handles team management endpoints
type TeamHandler struct {
	teamService  *models.TeamService
	groupService *models.GroupService
}

// NewTeamHandler creates a new team handler
func NewTeamHandler(teamService *models.TeamService, groupService *models.GroupService) *TeamHandler {
	return &TeamHandler{
		teamService:  teamService,
		groupService: groupService,
	}
}

// CreateTeamRequest represents a request to create a team
type CreateTeamRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	GroupID     int    `json:"group_id"`
}

// UpdateTeamRequest represents a request to update a team
type UpdateTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	GroupID     *int   `json:"group_id"`
}

// CreateGroupRequest represents a request to create a group
type CreateGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateGroupRequest represents a request to update a group
type UpdateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Team endpoints

// ListTeams lists all teams with optional filtering
func (h *TeamHandler) ListTeams(c *gin.Context) {
	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	groupIDStr := c.Query("group_id")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	// Parse group ID filter
	var groupID *int
	if groupIDStr != "" {
		if gid, err := strconv.Atoi(groupIDStr); err == nil {
			groupID = &gid
		}
	}

	// Get teams
	teams, totalCount, err := h.teamService.ListTeams(offset, limit, groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list teams"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"teams": teams,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total_count": totalCount,
			"total_pages": (totalCount + limit - 1) / limit,
		},
	})
}

// GetTeam retrieves a specific team
func (h *TeamHandler) GetTeam(c *gin.Context) {
	// Get team ID from URL
	teamID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	// Get team
	team := &models.Team{}
	if err := h.teamService.GetTeamByID(teamID, team); err != nil {
		if err == models.ErrTeamNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get team"})
		return
	}

	// Load team members
	members, _ := h.teamService.GetTeamMembers(teamID)
	team.Members = members

	c.JSON(http.StatusOK, team)
}

// CreateTeam creates a new team
func (h *TeamHandler) CreateTeam(c *gin.Context) {
	var req CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create team
	team := &models.Team{
		Name:        req.Name,
		Description: req.Description,
		GroupID:     req.GroupID,
	}

	if err := h.teamService.CreateTeam(team); err != nil {
		if err == models.ErrTeamNameExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Team name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create team"})
		return
	}

	// Store team ID for audit logging
	c.Set("resourceID", team.ID)

	c.JSON(http.StatusCreated, team)
}

// UpdateTeam updates a team
func (h *TeamHandler) UpdateTeam(c *gin.Context) {
	// Get team ID from URL
	teamID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	var req UpdateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing team
	team := &models.Team{}
	if err := h.teamService.GetTeamByID(teamID, team); err != nil {
		if err == models.ErrTeamNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get team"})
		return
	}

	// Update fields
	if req.Name != "" {
		team.Name = req.Name
	}
	if req.Description != "" {
		team.Description = req.Description
	}
	if req.GroupID != nil {
		team.GroupID = *req.GroupID
	}

	// Save updates
	if err := h.teamService.UpdateTeam(team); err != nil {
		if err == models.ErrTeamNameExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Team name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update team"})
		return
	}

	// Store team ID for audit logging
	c.Set("resourceID", team.ID)

	c.JSON(http.StatusOK, team)
}

// DeleteTeam deletes a team
func (h *TeamHandler) DeleteTeam(c *gin.Context) {
	// Get team ID from URL
	teamID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	// Delete team
	if err := h.teamService.DeleteTeam(teamID); err != nil {
		if err == models.ErrTeamNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Store team ID for audit logging
	c.Set("resourceID", teamID)

	c.JSON(http.StatusOK, gin.H{"message": "Team deleted successfully"})
}

// GetTeamMembers gets all members of a team
func (h *TeamHandler) GetTeamMembers(c *gin.Context) {
	// Get team ID from URL
	teamID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	// Get team members
	members, err := h.teamService.GetTeamMembers(teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get team members"})
		return
	}

	c.JSON(http.StatusOK, members)
}

// Group endpoints

// ListGroups lists all groups
func (h *TeamHandler) ListGroups(c *gin.Context) {
	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	// Get groups
	groups, totalCount, err := h.groupService.ListGroups(offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list groups"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total_count": totalCount,
			"total_pages": (totalCount + limit - 1) / limit,
		},
	})
}

// GetGroup retrieves a specific group
func (h *TeamHandler) GetGroup(c *gin.Context) {
	// Get group ID from URL
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	// Get group
	group := &models.Group{}
	if err := h.groupService.GetGroupByID(groupID, group); err != nil {
		if err == models.ErrGroupNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get group"})
		return
	}

	// Load group teams
	teams, _ := h.groupService.GetGroupTeams(groupID)
	group.Teams = teams

	// Load group members
	members, _ := h.groupService.GetGroupMembers(groupID)
	group.Members = members

	c.JSON(http.StatusOK, group)
}

// CreateGroup creates a new group
func (h *TeamHandler) CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create group
	group := &models.Group{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := h.groupService.CreateGroup(group); err != nil {
		if err == models.ErrGroupNameExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Group name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
		return
	}

	// Store group ID for audit logging
	c.Set("resourceID", group.ID)

	c.JSON(http.StatusCreated, group)
}

// UpdateGroup updates a group
func (h *TeamHandler) UpdateGroup(c *gin.Context) {
	// Get group ID from URL
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	var req UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing group
	group := &models.Group{}
	if err := h.groupService.GetGroupByID(groupID, group); err != nil {
		if err == models.ErrGroupNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get group"})
		return
	}

	// Update fields
	if req.Name != "" {
		group.Name = req.Name
	}
	if req.Description != "" {
		group.Description = req.Description
	}

	// Save updates
	if err := h.groupService.UpdateGroup(group); err != nil {
		if err == models.ErrGroupNameExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Group name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update group"})
		return
	}

	// Store group ID for audit logging
	c.Set("resourceID", group.ID)

	c.JSON(http.StatusOK, group)
}

// DeleteGroup deletes a group
func (h *TeamHandler) DeleteGroup(c *gin.Context) {
	// Get group ID from URL
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	// Delete group
	if err := h.groupService.DeleteGroup(groupID); err != nil {
		if err == models.ErrGroupNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Store group ID for audit logging
	c.Set("resourceID", groupID)

	c.JSON(http.StatusOK, gin.H{"message": "Group deleted successfully"})
}

// GetGroupMembers gets all members of a group
func (h *TeamHandler) GetGroupMembers(c *gin.Context) {
	// Get group ID from URL
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	// Get group members
	members, err := h.groupService.GetGroupMembers(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get group members"})
		return
	}

	c.JSON(http.StatusOK, members)
}

// GetGroupTeams gets all teams in a group
func (h *TeamHandler) GetGroupTeams(c *gin.Context) {
	// Get group ID from URL
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	// Get group teams
	teams, err := h.groupService.GetGroupTeams(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get group teams"})
		return
	}

	c.JSON(http.StatusOK, teams)
}

// RegisterRoutes registers team and group management routes
func (h *TeamHandler) RegisterRoutes(router *gin.RouterGroup, middleware *auth.Middleware) {
	// Team routes
	teams := router.Group("/teams")
	teams.Use(middleware.RequireAuth())
	{
		// Read operations
		read := teams.Group("")
		read.Use(middleware.RequirePermission(models.ResourceTeam, models.ActionRead))
		{
			read.GET("", h.ListTeams)
			read.GET("/:id", h.GetTeam)
			read.GET("/:id/members", h.GetTeamMembers)
		}

		// Write operations
		write := teams.Group("")
		write.Use(middleware.RequirePermission(models.ResourceTeam, models.ActionCreate))
		{
			write.POST("", middleware.AuditLog("create_team", "team"), h.CreateTeam)
		}

		update := teams.Group("")
		update.Use(middleware.RequirePermission(models.ResourceTeam, models.ActionUpdate))
		{
			update.PUT("/:id", middleware.AuditLog("update_team", "team"), h.UpdateTeam)
		}

		del := teams.Group("")
		del.Use(middleware.RequirePermission(models.ResourceTeam, models.ActionDelete))
		{
			del.DELETE("/:id", middleware.AuditLog("delete_team", "team"), h.DeleteTeam)
		}
	}

	// Group routes
	groups := router.Group("/groups")
	groups.Use(middleware.RequireAuth())
	{
		// Read operations
		read := groups.Group("")
		read.Use(middleware.RequirePermission(models.ResourceGroup, models.ActionRead))
		{
			read.GET("", h.ListGroups)
			read.GET("/:id", h.GetGroup)
			read.GET("/:id/members", h.GetGroupMembers)
			read.GET("/:id/teams", h.GetGroupTeams)
		}

		// Write operations
		write := groups.Group("")
		write.Use(middleware.RequirePermission(models.ResourceGroup, models.ActionCreate))
		{
			write.POST("", middleware.AuditLog("create_group", "group"), h.CreateGroup)
		}

		update := groups.Group("")
		update.Use(middleware.RequirePermission(models.ResourceGroup, models.ActionUpdate))
		{
			update.PUT("/:id", middleware.AuditLog("update_group", "group"), h.UpdateGroup)
		}

		del := groups.Group("")
		del.Use(middleware.RequirePermission(models.ResourceGroup, models.ActionDelete))
		{
			del.DELETE("/:id", middleware.AuditLog("delete_group", "group"), h.DeleteGroup)
		}
	}
}
