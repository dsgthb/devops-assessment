package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"

	"github.com/dsgthb/devops-assessment/internal/auth"
	"github.com/dsgthb/devops-assessment/internal/models"
	"github.com/dsgthb/devops-assessment/internal/services"
	"github.com/gin-gonic/gin"
)

// SurveyHandler handles survey-related endpoints
type SurveyHandler struct {
	surveyService     *services.SurveyService
	questionService   *models.QuestionService
	assessmentService *models.AssessmentService
	rbacService       *models.RBACService
}

// NewSurveyHandler creates a new survey handler
func NewSurveyHandler(
	surveyService *services.SurveyService,
	questionService *models.QuestionService,
	assessmentService *models.AssessmentService,
	rbacService *models.RBACService,
) *SurveyHandler {
	return &SurveyHandler{
		surveyService:     surveyService,
		questionService:   questionService,
		assessmentService: assessmentService,
		rbacService:       rbacService,
	}
}

// StartAssessmentRequest represents a request to start a new assessment
type StartAssessmentRequest struct {
	TeamID int `json:"team_id" binding:"required"`
}

// SaveResponsesRequest represents a request to save responses
type SaveResponsesRequest struct {
	Responses map[string][]string `json:"responses"`
}

// StartAssessment starts a new assessment
func (h *SurveyHandler) StartAssessment(c *gin.Context) {
	var req StartAssessmentRequest
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

	// Check if user has permission to create assessments for this team
	hasPermission, err := h.rbacService.CheckTeamPermission(
		user.ID, req.TeamID, models.ResourceAssessment, models.ActionCreate,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return
	}
	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	// Start assessment
	assessment, survey, err := h.surveyService.StartAssessment(req.TeamID, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Store assessment ID for audit logging
	c.Set("resourceID", assessment.ID)

	c.JSON(http.StatusCreated, gin.H{
		"assessment": assessment,
		"survey":     survey,
	})
}

// GetAssessment retrieves an assessment with its current state
func (h *SurveyHandler) GetAssessment(c *gin.Context) {
	// Get assessment ID from URL
	assessmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assessment ID"})
		return
	}

	// Get current user
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Load assessment
	assessment := &models.Assessment{}
	if err := h.assessmentService.GetAssessmentByID(assessmentID, assessment); err != nil {
		if err == models.ErrAssessmentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Assessment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load assessment"})
		return
	}

	// Check if user has permission to view this assessment
	hasPermission, err := h.rbacService.CheckTeamPermission(
		user.ID, assessment.TeamID, models.ResourceAssessment, models.ActionRead,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return
	}
	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	// Continue assessment
	assessment, survey, err := h.surveyService.ContinueAssessment(assessmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"assessment": assessment,
		"survey":     survey,
	})
}

// SaveResponses saves responses for a section
func (h *SurveyHandler) SaveResponses(c *gin.Context) {
	// Get assessment ID from URL
	assessmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assessment ID"})
		return
	}

	// Get section name from URL
	sectionName := c.Param("section")
	if sectionName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Section name required"})
		return
	}

	// Get current user
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Load assessment
	assessment := &models.Assessment{}
	if err := h.assessmentService.GetAssessmentByID(assessmentID, assessment); err != nil {
		if err == models.ErrAssessmentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Assessment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load assessment"})
		return
	}

	// Check if user has permission to update this assessment
	hasPermission, err := h.rbacService.CheckTeamPermission(
		user.ID, assessment.TeamID, models.ResourceAssessment, models.ActionUpdate,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return
	}
	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	// Check if assessment is still in progress
	if assessment.Status != models.StatusInProgress {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Assessment is already completed"})
		return
	}

	// Parse form data
	var req SaveResponsesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Save responses
	if err := h.surveyService.SaveResponses(assessmentID, sectionName, req.Responses); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Responses saved successfully"})
}

// CompleteAssessment completes an assessment and calculates results
func (h *SurveyHandler) CompleteAssessment(c *gin.Context) {
	// Get assessment ID from URL
	assessmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assessment ID"})
		return
	}

	// Get current user
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Load assessment
	assessment := &models.Assessment{}
	if err := h.assessmentService.GetAssessmentByID(assessmentID, assessment); err != nil {
		if err == models.ErrAssessmentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Assessment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load assessment"})
		return
	}

	// Check if user has permission to update this assessment
	hasPermission, err := h.rbacService.CheckTeamPermission(
		user.ID, assessment.TeamID, models.ResourceAssessment, models.ActionUpdate,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return
	}
	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	// Calculate and save results
	results, err := h.surveyService.CalculateResults(assessmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Load advice
	advice, err := h.questionService.LoadAdvice()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load advice"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Assessment completed successfully",
		"results": results,
		"advice":  advice,
	})
}

// GetResults retrieves results for a completed assessment
func (h *SurveyHandler) GetResults(c *gin.Context) {
	// Get assessment ID from URL
	assessmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assessment ID"})
		return
	}

	// Get current user
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Load assessment
	assessment := &models.Assessment{}
	if err := h.assessmentService.GetAssessmentByID(assessmentID, assessment); err != nil {
		if err == models.ErrAssessmentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Assessment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load assessment"})
		return
	}

	// Check if user has permission to view this assessment
	hasPermission, err := h.rbacService.CheckTeamPermission(
		user.ID, assessment.TeamID, models.ResourceAssessment, models.ActionRead,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return
	}
	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	// Get results
	results, err := h.surveyService.GetAssessmentResults(assessmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Load advice
	advice, err := h.questionService.LoadAdvice()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load advice"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"assessment": assessment,
		"results":    results,
		"advice":     advice,
	})
}

// ExportCSV exports assessment results as CSV
func (h *SurveyHandler) ExportCSV(c *gin.Context) {
	// Get assessment ID from URL
	assessmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assessment ID"})
		return
	}

	// Get current user
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Load assessment
	assessment := &models.Assessment{}
	if err := h.assessmentService.GetAssessmentByID(assessmentID, assessment); err != nil {
		if err == models.ErrAssessmentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Assessment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load assessment"})
		return
	}

	// Check if user has permission to export this assessment
	hasPermission, err := h.rbacService.CheckTeamPermission(
		user.ID, assessment.TeamID, models.ResourceReport, models.ActionExport,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return
	}
	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	// Create CSV buffer
	var buf bytes.Buffer
	if err := h.surveyService.ExportAssessmentCSV(assessmentID, &buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Set headers for CSV download
	filename := fmt.Sprintf("devops-assessment-%d.csv", assessmentID)
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "text/csv", buf.Bytes())
}

// GetTeamAssessments gets all assessments for a team
func (h *SurveyHandler) GetTeamAssessments(c *gin.Context) {
	// Get team ID from URL
	teamID, err := strconv.Atoi(c.Param("teamId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	// Get current user
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Check if user has permission to view team assessments
	hasPermission, err := h.rbacService.CheckTeamPermission(
		user.ID, teamID, models.ResourceAssessment, models.ActionRead,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return
	}
	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	// Get assessment history
	history, err := h.surveyService.GetTeamAssessmentHistory(teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, history)
}

// RegisterRoutes registers survey routes
func (h *SurveyHandler) RegisterRoutes(router *gin.RouterGroup, middleware *auth.Middleware) {
	survey := router.Group("/assessments")
	survey.Use(middleware.RequireAuth())
	{
		// Assessment operations
		survey.POST("/start", middleware.AuditLog("create_assessment", "assessment"), h.StartAssessment)
		survey.GET("/:id", h.GetAssessment)
		survey.POST("/:id/sections/:section", h.SaveResponses)
		survey.POST("/:id/complete", middleware.AuditLog("complete_assessment", "assessment"), h.CompleteAssessment)
		survey.GET("/:id/results", h.GetResults)
		survey.GET("/:id/export/csv", middleware.AuditLog("export_assessment", "assessment"), h.ExportCSV)

		// Team assessments
		survey.GET("/teams/:teamId", h.GetTeamAssessments)
	}
}
