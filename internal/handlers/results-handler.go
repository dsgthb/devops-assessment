package handlers

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/dsgthb/devops-assessment/internal/auth"
	"github.com/dsgthb/devops-assessment/internal/models"
	"github.com/dsgthb/devops-assessment/internal/services"
	"github.com/gin-gonic/gin"
)

// ResultsHandler handles results viewing and resources pages
type ResultsHandler struct {
	surveyService     *services.SurveyService
	questionService   *models.QuestionService
	assessmentService *models.AssessmentService
	rbacService       *models.RBACService
	templates         *template.Template
}

// NewResultsHandler creates a new results handler
func NewResultsHandler(
	surveyService *services.SurveyService,
	questionService *models.QuestionService,
	assessmentService *models.AssessmentService,
	rbacService *models.RBACService,
	templates *template.Template,
) *ResultsHandler {
	return &ResultsHandler{
		surveyService:     surveyService,
		questionService:   questionService,
		assessmentService: assessmentService,
		rbacService:       rbacService,
		templates:         templates,
	}
}

// PageData represents common data for all pages
type PageData struct {
	Title      string
	User       *models.User
	ActivePage string
	Survey     *models.Survey
	NavBar     map[string]NavItem
}

// NavItem represents a navigation menu item
type NavItem struct {
	Type  string
	URL   string
	Items map[string]NavItem
}

// ResultsPageData represents data for the results page
type ResultsPageData struct {
	PageData
	Assessment *models.Assessment
	Results    *services.AssessmentResults
	Advice     map[string]models.Advice
	ChartData  ChartData
}

// ChartData represents data for the chart visualization
type ChartData struct {
	Labels []string
	Data   []float64
	Title  string
}

// ResourcesPageData represents data for the resources page
type ResourcesPageData struct {
	PageData
	Advice map[string]models.Advice
}

// DashboardPageData represents data for the dashboard
type DashboardPageData struct {
	PageData
	Teams       []models.Team
	Assessments []AssessmentSummary
	Statistics  DashboardStats
}

// AssessmentSummary represents a summary of an assessment for display
type AssessmentSummary struct {
	Assessment   models.Assessment
	TeamName     string
	OverallScore float64
}

// DashboardStats represents dashboard statistics
type DashboardStats struct {
	TotalAssessments   int
	CompletedThisMonth int
	AverageScore       float64
	TeamsAssessed      int
}

// Dashboard shows the main dashboard
func (h *ResultsHandler) Dashboard(c *gin.Context) {
	user, _ := auth.GetCurrentUser(c)

	// Get user teams
	teams, _ := user.Teams, user.Groups

	// Get recent assessments
	var assessments []AssessmentSummary
	for _, team := range user.Teams {
		teamAssessments, _ := h.assessmentService.ListTeamAssessments(team.Team.ID, false)
		for _, assessment := range teamAssessments {
			if assessment.Status == models.StatusCompleted {
				scores, _ := h.assessmentService.GetAssessmentScores(assessment.ID)
				summary := AssessmentSummary{
					Assessment:   assessment,
					TeamName:     team.Team.Name,
					OverallScore: calculateOverallScore(scores),
				}
				assessments = append(assessments, summary)
			}
		}
	}

	// Calculate statistics
	stats := h.calculateDashboardStats(assessments)

	data := DashboardPageData{
		PageData:    h.getPageData("Dashboard", user, "Dashboard"),
		Teams:       extractTeams(user.Teams),
		Assessments: assessments,
		Statistics:  stats,
	}

	c.HTML(http.StatusOK, "dashboard.html", data)
}

// ViewResults shows the results page for an assessment
func (h *ResultsHandler) ViewResults(c *gin.Context) {
	// Get assessment ID from URL or query
	assessmentIDStr := c.Param("id")
	if assessmentIDStr == "" {
		assessmentIDStr = c.Query("assessment_id")
	}

	var assessment *models.Assessment
	var results *services.AssessmentResults

	if assessmentIDStr != "" {
		// Load specific assessment
		assessmentID, err := strconv.Atoi(assessmentIDStr)
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Invalid assessment ID"})
			return
		}

		assessment = &models.Assessment{}
		if err := h.assessmentService.GetAssessmentByID(assessmentID, assessment); err != nil {
			c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Assessment not found"})
			return
		}

		// Check permissions
		user, _ := auth.GetCurrentUser(c)
		if user != nil {
			hasPermission, _ := h.rbacService.CheckTeamPermission(
				user.ID, assessment.TeamID, models.ResourceAssessment, models.ActionRead,
			)
			if !hasPermission {
				c.HTML(http.StatusForbidden, "error.html", gin.H{"error": "Access denied"})
				return
			}
		}

		// Get results
		results, err = h.surveyService.GetAssessmentResults(assessmentID)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}
	}

	// Load advice
	advice, _ := h.questionService.LoadAdvice()

	// Prepare chart data
	chartData := h.prepareChartData(results)

	// Get current user
	user, _ := auth.GetCurrentUser(c)

	data := ResultsPageData{
		PageData:   h.getPageData("Results", user, "Results"),
		Assessment: assessment,
		Results:    results,
		Advice:     advice,
		ChartData:  chartData,
	}

	c.HTML(http.StatusOK, "results.html", data)
}

// ViewDetailedResults shows detailed results for a specific section
func (h *ResultsHandler) ViewDetailedResults(c *gin.Context) {
	// Get section name from URL
	sectionName := c.Param("section")
	if sectionName == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Section name required"})
		return
	}

	// Get assessment ID
	assessmentIDStr := c.Query("assessment_id")
	if assessmentIDStr == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Assessment ID required"})
		return
	}

	assessmentID, err := strconv.Atoi(assessmentIDStr)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Invalid assessment ID"})
		return
	}

	// Load assessment
	assessment := &models.Assessment{}
	if err := h.assessmentService.GetAssessmentByID(assessmentID, assessment); err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Assessment not found"})
		return
	}

	// Check permissions
	user, _ := auth.GetCurrentUser(c)
	if user != nil {
		hasPermission, _ := h.rbacService.CheckTeamPermission(
			user.ID, assessment.TeamID, models.ResourceAssessment, models.ActionRead,
		)
		if !hasPermission {
			c.HTML(http.StatusForbidden, "error.html", gin.H{"error": "Access denied"})
			return
		}
	}

	// Get results
	results, err := h.surveyService.GetAssessmentResults(assessmentID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
		return
	}

	// Load advice
	advice, _ := h.questionService.LoadAdvice()

	// Prepare chart data for subcategories
	chartData := h.prepareSubCategoryChartData(results, sectionName)

	data := ResultsPageData{
		PageData:   h.getPageData("Detailed Results - "+sectionName, user, "Detailed Reports"),
		Assessment: assessment,
		Results:    results,
		Advice:     advice,
		ChartData:  chartData,
	}

	c.HTML(http.StatusOK, "detailed-results.html", data)
}

// ViewResources shows the resources page
func (h *ResultsHandler) ViewResources(c *gin.Context) {
	// Load advice
	advice, err := h.questionService.LoadAdvice()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to load resources"})
		return
	}

	// Get current user
	user, _ := auth.GetCurrentUser(c)

	data := ResourcesPageData{
		PageData: h.getPageData("Resources", user, "Resources"),
		Advice:   advice,
	}

	c.HTML(http.StatusOK, "resources.html", data)
}

// Helper methods

// getPageData returns common page data
func (h *ResultsHandler) getPageData(title string, user *models.User, activePage string) PageData {
	// Load survey for navigation
	survey, _ := h.questionService.LoadQuestions()

	// Build navigation
	navBar := h.buildNavigation(survey)

	return PageData{
		Title:      title,
		User:       user,
		ActivePage: activePage,
		Survey:     survey,
		NavBar:     navBar,
	}
}

// buildNavigation builds the navigation menu structure
func (h *ResultsHandler) buildNavigation(survey *models.Survey) map[string]NavItem {
	navBar := make(map[string]NavItem)

	// Questionnaire
	if survey != nil && len(survey.Sections) > 0 {
		navBar["Questionnaire"] = NavItem{
			Type: "Standard",
			URL:  "/survey/section-" + models.SectionNameToURLName(survey.Sections[0].SectionName),
		}
	}

	// Sections dropdown
	sectionsItems := make(map[string]NavItem)
	if survey != nil {
		for _, section := range survey.Sections {
			sectionsItems[section.SectionName] = NavItem{
				Type: "Standard",
				URL:  "/survey/section-" + models.SectionNameToURLName(section.SectionName),
			}
		}
	}
	navBar["Sections"] = NavItem{
		Type:  "Dropdown",
		Items: sectionsItems,
	}

	// Results
	navBar["Results"] = NavItem{
		Type: "Standard",
		URL:  "/results",
	}

	// Detailed Reports dropdown
	detailedItems := make(map[string]NavItem)
	detailedItems["Download CSV"] = NavItem{
		Type: "Standard",
		URL:  "#", // Will be handled by JavaScript
	}

	if survey != nil {
		for _, section := range survey.Sections {
			if section.HasSubCategories {
				detailedItems[section.SectionName] = NavItem{
					Type: "Standard",
					URL:  "/results/" + models.SectionNameToURLName(section.SectionName),
				}
			}
		}
	}
	navBar["Detailed Reports"] = NavItem{
		Type:  "Dropdown",
		Items: detailedItems,
	}

	// Resources
	navBar["Resources"] = NavItem{
		Type: "Standard",
		URL:  "/resources",
	}

	// About
	navBar["About"] = NavItem{
		Type: "Standard",
		URL:  "/about",
	}

	return navBar
}

// prepareChartData prepares data for the radar chart
func (h *ResultsHandler) prepareChartData(results *services.AssessmentResults) ChartData {
	if results == nil {
		return ChartData{Title: "DevOps Maturity by Area"}
	}

	var labels []string
	var data []float64

	// Sort by spider position for consistent display
	for _, score := range results.SectionScores {
		labels = append(labels, score.SectionName)
		data = append(data, score.Percentage)
	}

	return ChartData{
		Labels: labels,
		Data:   data,
		Title:  "DevOps Maturity by Area",
	}
}

// prepareSubCategoryChartData prepares data for subcategory chart
func (h *ResultsHandler) prepareSubCategoryChartData(results *services.AssessmentResults, sectionName string) ChartData {
	if results == nil || results.SubCategoryScores == nil {
		return ChartData{Title: "Breakdown for " + sectionName}
	}

	var labels []string
	var data []float64

	if scores, exists := results.SubCategoryScores[sectionName]; exists {
		for _, score := range scores {
			labels = append(labels, score.SectionName)
			data = append(data, score.Percentage)
		}
	}

	return ChartData{
		Labels: labels,
		Data:   data,
		Title:  "Breakdown for " + sectionName,
	}
}

// calculateDashboardStats calculates dashboard statistics
func (h *ResultsHandler) calculateDashboardStats(assessments []AssessmentSummary) DashboardStats {
	stats := DashboardStats{}

	stats.TotalAssessments = len(assessments)

	// Calculate other stats
	totalScore := 0.0
	teamsMap := make(map[int]bool)

	for _, assessment := range assessments {
		totalScore += assessment.OverallScore
		teamsMap[assessment.Assessment.TeamID] = true

		// Check if completed this month
		if assessment.Assessment.CompletedAt != nil {
			// Simple month check - you might want to improve this
			stats.CompletedThisMonth++
		}
	}

	stats.TeamsAssessed = len(teamsMap)
	if stats.TotalAssessments > 0 {
		stats.AverageScore = totalScore / float64(stats.TotalAssessments)
	}

	return stats
}

// Helper functions

func calculateOverallScore(scores []models.SectionScore) float64 {
	if len(scores) == 0 {
		return 0
	}

	totalScore := 0.0
	totalMaxScore := 0.0

	for _, score := range scores {
		totalScore += score.Score
		totalMaxScore += score.MaxScore
	}

	if totalMaxScore == 0 {
		return 0
	}

	return (totalScore / totalMaxScore) * 100
}

func extractTeams(memberships []models.TeamMembership) []models.Team {
	teams := make([]models.Team, len(memberships))
	for i, membership := range memberships {
		teams[i] = membership.Team
	}
	return teams
}

// RegisterRoutes registers results and resources routes
func (h *ResultsHandler) RegisterRoutes(router *gin.RouterGroup, middleware *auth.Middleware) {
	// Public routes (optional auth for anonymous results viewing)
	public := router.Group("")
	public.Use(middleware.OptionalAuth())
	{
		public.GET("/results", h.ViewResults)
		public.GET("/results/:section", h.ViewDetailedResults)
		public.GET("/resources", h.ViewResources)
	}

	// Protected routes
	protected := router.Group("")
	protected.Use(middleware.RequireAuth())
	{
		protected.GET("/dashboard", h.Dashboard)
	}
}
