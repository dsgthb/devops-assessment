package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Session  SessionConfig
	Files    FileConfig
	Security SecurityConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Host         string
	Port         int
	Mode         string // "debug", "release", "test"
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	Database     string
	MaxOpenConns int
	MaxIdleConns int
	MaxLifetime  time.Duration
}

// SessionConfig holds session configuration
type SessionConfig struct {
	Secret   string
	Duration time.Duration
}

// FileConfig holds file paths configuration
type FileConfig struct {
	QuestionsPath  string
	AdvicePath     string
	TemplatesPath  string
	StaticPath     string
	UploadsPath    string
}

// SecurityConfig holds security configuration
type SecurityConfig struct {
	BCryptCost     int
	CSRFSecret     string
	AllowedOrigins []string
	TrustedProxies []string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// It's okay if .env doesn't exist
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("error loading .env file: %w", err)
		}
	}

	cfg := &Config{
		Server: ServerConfig{
			Host:         getEnvString("SERVER_HOST", "0.0.0.0"),
			Port:         getEnvInt("SERVER_PORT", 8080),
			Mode:         getEnvString("SERVER_MODE", "release"),
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 10*time.Second),
		},
		Database: DatabaseConfig{
			Host:         getEnvString("DB_HOST", "localhost"),
			Port:         getEnvInt("DB_PORT", 3306),
			User:         getEnvString("DB_USER", "root"),
			Password:     getEnvString("DB_PASSWORD", ""),
			Database:     getEnvString("DB_NAME", "devops_assessment"),
			MaxOpenConns: getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns: getEnvInt("DB_MAX_IDLE_CONNS", 5),
			MaxLifetime:  getEnvDuration("DB_MAX_LIFETIME", 5*time.Minute),
		},
		Session: SessionConfig{
			Secret:   getEnvString("SESSION_SECRET", generateDefaultSecret()),
			Duration: getEnvDuration("SESSION_DURATION", 7*24*time.Hour),
		},
		Files: FileConfig{
			QuestionsPath: getEnvString("QUESTIONS_FILE", "configs/questions.json"),
			AdvicePath:    getEnvString("ADVICE_FILE", "configs/advice.json"),
			TemplatesPath: getEnvString("TEMPLATES_PATH", "web/templates"),
			StaticPath:    getEnvString("STATIC_PATH", "web/static"),
			UploadsPath:   getEnvString("UPLOADS_PATH", "uploads"),
		},
		Security: SecurityConfig{
			BCryptCost:     getEnvInt("BCRYPT_COST", 10),
			CSRFSecret:     getEnvString("CSRF_SECRET", generateDefaultSecret()),
			AllowedOrigins: getEnvStringSlice("ALLOWED_ORIGINS", []string{"*"}),
			TrustedProxies: getEnvStringSlice("TRUSTED_PROXIES", []string{}),
		},
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Server validation
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	// Database validation
	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("database user is required")
	}
	if c.Database.Database == "" {
		return fmt.Errorf("database name is required")
	}

	// Session validation
	if c.Session.Secret == "" {
		return fmt.Errorf("session secret is required")
	}
	if len(c.Session.Secret) < 32 {
		return fmt.Errorf("session secret must be at least 32 characters")
	}

	// File validation
	if c.Files.QuestionsPath == "" {
		return fmt.Errorf("questions file path is required")
	}
	if c.Files.AdvicePath == "" {
		return fmt.Errorf("advice file path is required")
	}

	// Check if files exist
	if _, err := os.Stat(c.Files.QuestionsPath); os.IsNotExist(err) {
		return fmt.Errorf("questions file not found: %s", c.Files.QuestionsPath)
	}
	if _, err := os.Stat(c.Files.AdvicePath); os.IsNotExist(err) {
		return fmt.Errorf("advice file not found: %s", c.Files.AdvicePath)
	}

	// Create directories if they don't exist
	dirs := []string{c.Files.TemplatesPath, c.Files.StaticPath, c.Files.UploadsPath}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// GetDSN returns the database connection string
func (c *Config) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.Database,
	)
}

// Helper functions

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Simple comma-separated parsing
		var result []string
		for _, v := range strings.Split(value, ",") {
			v = strings.TrimSpace(v)
			if v != "" {
				result = append(result, v)
			}
		}
		return result
	}
	return defaultValue
}

func generateDefaultSecret() string {
	// In production, this should be set via environment variable
	// This is just a fallback for development
	return "CHANGE_THIS_SECRET_IN_PRODUCTION_" + strconv.FormatInt(time.Now().UnixNano(), 36)
}