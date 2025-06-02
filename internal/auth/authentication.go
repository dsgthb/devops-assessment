package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/dsgthb/devops-assessment/internal/database"
	"github.com/dsgthb/devops-assessment/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// AuthService handles authentication operations
type AuthService struct {
	db          *database.DB
	userService *models.UserService
}

// Session represents a user session
type Session struct {
	ID           int       `json:"id"`
	UserID       int       `json:"user_id"`
	SessionToken string    `json:"session_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Loaded separately
	User *models.User `json:"user,omitempty"`
}

// NewAuthService creates a new authentication service
func NewAuthService(db *database.DB) *AuthService {
	return &AuthService{
		db:          db,
		userService: models.NewUserService(db),
	}
}

// Common errors
var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserInactive       = errors.New("user account is inactive")
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionExpired     = errors.New("session has expired")
)

// Configuration
const (
	SessionDuration = 24 * time.Hour * 7 // 7 days
	TokenLength     = 32                 // bytes
)

// Login authenticates a user and creates a session
func (s *AuthService) Login(email, password string) (*Session, error) {
	// Validate credentials
	user, err := s.userService.ValidateCredentials(email, password)
	if err != nil {
		return nil, err
	}

	// Create session
	session, err := s.CreateSession(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	session.User = user
	return session, nil
}

// CreateSession creates a new session for a user
func (s *AuthService) CreateSession(userID int) (*Session, error) {
	// Generate session token
	token, err := generateSecureToken(TokenLength)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}

	// Calculate expiration time
	expiresAt := time.Now().Add(SessionDuration)

	// Insert session
	query := `
		INSERT INTO user_sessions (user_id, session_token, expires_at)
		VALUES (?, ?, ?)
	`

	id, err := s.db.Insert(query, userID, token, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	session := &Session{
		ID:           int(id),
		UserID:       userID,
		SessionToken: token,
		ExpiresAt:    expiresAt,
	}

	return session, nil
}

// ValidateSession validates a session token and returns the session
func (s *AuthService) ValidateSession(token string) (*Session, error) {
	query := `
		SELECT id, user_id, session_token, expires_at, created_at, updated_at
		FROM user_sessions
		WHERE session_token = ?
	`

	session := &Session{}
	err := s.db.QueryRowContext(context.Background(), query, token).Scan(
		&session.ID,
		&session.UserID,
		&session.SessionToken,
		&session.ExpiresAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		// Delete expired session
		s.DeleteSession(token)
		return nil, ErrSessionExpired
	}

	// Load user information
	user := &models.User{}
	if err := s.userService.GetUserByID(session.UserID, user); err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	session.User = user

	// Update session last activity
	s.TouchSession(session.ID)

	return session, nil
}

// TouchSession updates the session's last activity time
func (s *AuthService) TouchSession(sessionID int) error {
	query := `
		UPDATE user_sessions 
		SET updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := s.db.Update(query, sessionID)
	return err
}

// ExtendSession extends a session's expiration time
func (s *AuthService) ExtendSession(token string) error {
	newExpiry := time.Now().Add(SessionDuration)

	query := `
		UPDATE user_sessions 
		SET expires_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE session_token = ? AND expires_at > NOW()
	`

	affected, err := s.db.Update(query, newExpiry, token)
	if err != nil {
		return fmt.Errorf("failed to extend session: %w", err)
	}

	if affected == 0 {
		return ErrSessionNotFound
	}

	return nil
}

// DeleteSession deletes a session
func (s *AuthService) DeleteSession(token string) error {
	query := `DELETE FROM user_sessions WHERE session_token = ?`

	_, err := s.db.Delete(query, token)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// DeleteUserSessions deletes all sessions for a user
func (s *AuthService) DeleteUserSessions(userID int) error {
	query := `DELETE FROM user_sessions WHERE user_id = ?`

	_, err := s.db.Delete(query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	return nil
}

// CleanupExpiredSessions removes all expired sessions
func (s *AuthService) CleanupExpiredSessions() error {
	query := `DELETE FROM user_sessions WHERE expires_at < NOW()`

	affected, err := s.db.Delete(query)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}

	if affected > 0 {
		fmt.Printf("Cleaned up %d expired sessions\n", affected)
	}

	return nil
}

// GetUserActiveSessions gets all active sessions for a user
func (s *AuthService) GetUserActiveSessions(userID int) ([]Session, error) {
	query := `
		SELECT id, user_id, session_token, expires_at, created_at, updated_at
		FROM user_sessions
		WHERE user_id = ? AND expires_at > NOW()
		ORDER BY created_at DESC
	`

	rows, err := s.db.GetMany(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var session Session
		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.SessionToken,
			&session.ExpiresAt,
			&session.CreatedAt,
			&session.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// ChangePassword changes a user's password and invalidates all sessions
func (s *AuthService) ChangePassword(userID int, oldPassword, newPassword string) error {
	// Get user to verify old password
	user := &models.User{}
	if err := s.userService.GetUserByID(userID, user); err != nil {
		return err
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrInvalidCredentials
	}

	// Update password
	if err := s.userService.UpdatePassword(userID, newPassword); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Invalidate all user sessions
	if err := s.DeleteUserSessions(userID); err != nil {
		return fmt.Errorf("failed to invalidate sessions: %w", err)
	}

	return nil
}

// ResetPassword resets a user's password (admin action)
func (s *AuthService) ResetPassword(userID int, newPassword string) error {
	// Update password
	if err := s.userService.UpdatePassword(userID, newPassword); err != nil {
		return fmt.Errorf("failed to reset password: %w", err)
	}

	// Invalidate all user sessions
	if err := s.DeleteUserSessions(userID); err != nil {
		return fmt.Errorf("failed to invalidate sessions: %w", err)
	}

	return nil
}

// Helper functions

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
