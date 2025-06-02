package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dsgthb/devops-assessment/internal/auth"
	"github.com/dsgthb/devops-assessment/internal/config"
	"github.com/dsgthb/devops-assessment/internal/database"
	"github.com/dsgthb/devops-assessment/internal/handlers"
	"github.com/dsgthb/devops-assessment/internal/models"
	"github.com/dsgthb/devops-assessment/internal/services"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Connect to database
	db, err := database.NewConnection(database.Config{
		Host:         cfg.Database.Host,
		Port:         cfg.Database.Port,
		User:         cfg.Database.User,
		Password:     cfg.Database.Password,
		Database:     cfg.Database.Database,
		MaxOpenConns: cfg.Database.MaxOpenConns,
		MaxIdleConns: cfg.Database.MaxIdleConns,
		MaxLifetime:  cfg.Database.MaxLifetime,
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize services
	userService := models.NewUserService(db)
	teamService := models.NewTeamService(db)
	groupService := models.NewGroupService(db)
	roleService := models.NewRoleService(db)
	rbacService := models.NewRBACService(db)
	assessmentService := models.NewAssessmentService(db)
	questionService := models.NewQuestionService(cfg.Files.QuestionsPath, cfg.Files.AdvicePath)
	surveyService := services.NewSurveyService(db, cfg.Files.QuestionsPath, cfg.Files.AdvicePath)
	authService := auth.NewAuthService(db)

	// Load templates
	templates, err := loadTemplates(cfg.Files.TemplatesPath)
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	// Initialize middleware
	authMiddleware := auth.NewMiddleware(authService, rbacService)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService, userService)
	userHandler := handlers.NewUserHandler(userService, roleService, authService)
	teamHandler := handlers.NewTeamHandler(teamService, groupService)
	surveyHandler := handlers.NewSurveyHandler(surveyService, questionService, assessmentService, rbacService)
	resultsHandler := handlers.NewResultsHandler(surveyService, questionService, assessmentService, rbacService, templates)

	// Setup router
	router := setupRouter(cfg, templates, authMiddleware, authHandler, userHandler, teamHandler, surveyHandler, resultsHandler)

	// Start background tasks
	go startBackgroundTasks(authService)

	// Create default admin user if none exists
	if err := createDefaultAdmin(userService, teamService, roleService); err != nil {
		log.Printf("Warning: Failed to create default admin: %v", err)
	}

	// Setup graceful shutdown
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}

// setupRouter configures all routes
func setupRouter(
	cfg *config.Config,
	templates *template.Template,
	authMiddleware *auth.Middleware,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	teamHandler *handlers.TeamHandler,
	surveyHandler *handlers.SurveyHandler,
	resultsHandler *handlers.ResultsHandler,
) *gin.Engine {
	router := gin.New()

	// Global middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(authMiddleware.CORS())

	// Trust proxies if configured
	if len(cfg.Security.TrustedProxies) > 0 {
		router.SetTrustedProxies(cfg.Security.TrustedProxies)
	}

	// Set HTML templates
	router.SetHTMLTemplate(templates)

	// Static files
	router.Static("/static", cfg.Files.StaticPath)
	router.Static("/css", filepath.Join(cfg.Files.StaticPath, "css"))
	router.Static("/js", filepath.Join(cfg.Files.StaticPath, "js"))
	router.Static("/fontawesome", filepath.Join(cfg.Files.StaticPath, "fontawesome"))
	router.StaticFile("/favicon.ico", filepath.Join(cfg.Files.StaticPath, "favicon.ico"))

	// HTML routes
	htmlRouter := router.Group("")
	{
		// Public pages
		htmlRouter.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, "/login")
		})
		htmlRouter.GET("/login", renderLogin)
		htmlRouter.GET("/about", renderAbout)

		// Results and resources (optional auth)
		resultsHandler.RegisterRoutes(htmlRouter, authMiddleware)

		// Protected pages
		protected := htmlRouter.Group("")
		protected.Use(authMiddleware.RequireAuth())
		{
			protected.GET("/survey/*section", renderSurvey)
		}
	}

	// API routes
	api := router.Group("/api/v1")
	{
		// Register all API handlers
		authHandler.RegisterRoutes(api, authMiddleware)
		userHandler.RegisterRoutes(api, authMiddleware)
		teamHandler.RegisterRoutes(api, authMiddleware)
		surveyHandler.RegisterRoutes(api, authMiddleware)
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
		})
	})

	// 404 handler
	router.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"Title":      "Page Not Found",
			"StatusCode": 404,
		})
	})

	return router
}

// loadTemplates loads all HTML templates
func loadTemplates(templatesPath string) (*template.Template, error) {
	// Define template functions
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b float64) float64 { return a * b },
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"json": func(v interface{}) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("null")
			}
			return template.JS(b)
		},
		"sectionNameToURL": models.SectionNameToURLName,
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
	}

	// Load all templates
	pattern := filepath.Join(templatesPath, "*.html")
	tmpl, err := template.New("").Funcs(funcMap).ParseGlob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	// Also load partials if they exist
	partialPattern := filepath.Join(templatesPath, "partials", "*.html")
	if _, err := os.Stat(filepath.Join(templatesPath, "partials")); err == nil {
		tmpl, err = tmpl.ParseGlob(partialPattern)
		if err != nil {
			return nil, fmt.Errorf("failed to parse partial templates: %w", err)
		}
	}

	return tmpl, nil
}

// startBackgroundTasks starts background maintenance tasks
func startBackgroundTasks(authService *auth.AuthService) {
	// Clean up expired sessions every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := authService.CleanupExpiredSessions(); err != nil {
				log.Printf("Error cleaning up sessions: %v", err)
			}
		}
	}
}

// createDefaultAdmin creates a default admin user if none exists
func createDefaultAdmin(userService *models.UserService, teamService *models.TeamService, roleService *models.RoleService) error {
	// Check if any users exist
	users, _, err := userService.ListUsers(0, 1, false)
	if err != nil {
		return err
	}

	// If users exist, don't create default admin
	if len(users) > 0 {
		return nil
	}

	// Get default admin credentials from environment or use defaults
	adminEmail := os.Getenv("DEFAULT_ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@example.com"
	}

	adminPassword := os.Getenv("DEFAULT_ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "changeme123"
	}

	// Create default admin
	log.Println("Creating default admin user...")
	admin := &models.User{
		Email:     adminEmail,
		FirstName: "Admin",
		LastName:  "User",
		IsActive:  true,
	}

	if err := userService.CreateUser(admin, adminPassword); err != nil {
		return fmt.Errorf("failed to create default admin: %w", err)
	}

	// Create a default team
	defaultTeam := &models.Team{
		Name:        "Admin Team",
		Description: "Default administrative team",
	}

	if err := teamService.CreateTeam(defaultTeam); err != nil {
		log.Printf("Warning: Failed to create default team: %v", err)
	} else {
		// Get admin role
		adminRole, err := roleService.GetRoleByName(models.RoleAdmin)
		if err != nil {
			log.Printf("Warning: Failed to get admin role: %v", err)
		} else {
			// Add admin to team with admin role
			if err := userService.AddUserToTeam(admin.ID, defaultTeam.ID, adminRole.ID); err != nil {
				log.Printf("Warning: Failed to add admin to team: %v", err)
			}
		}
	}

	log.Printf("Default admin created. Email: %s, Password: %s", adminEmail, adminPassword)
	log.Println("IMPORTANT: Please change the default password immediately!")

	return nil
}

// Page rendering functions

func renderLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"Title": "Login - DevOps Assessment",
	})
}

func renderAbout(c *gin.Context) {
	c.HTML(http.StatusOK, "about.html", gin.H{
		"Title":      "About - DevOps Assessment",
		"ActivePage": "About",
	})
}

func renderSurvey(c *gin.Context) {
	section := c.Param("section")
	if section == "/" || section == "" {
		section = "/section-introduction"
	}

	// Remove leading slash
	section = strings.TrimPrefix(section, "/")

	// Get current user
	user, _ := auth.GetCurrentUser(c)

	c.HTML(http.StatusOK, "survey.html", gin.H{
		"Title":      "Survey - DevOps Assessment",
		"ActivePage": "Questionnaire",
		"User":       user,
		"Section":    section,
	})
}
