package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"devops-assessment/internal/database"

	"golang.org/x/crypto/bcrypt"
)

// User represents a system user
type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Never expose password hash in JSON
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Relationships (loaded separately)
	Teams  []TeamMembership  `json:"teams,omitempty"`
	Groups []GroupMembership `json:"groups,omitempty"`
}

// TeamMembership represents a user's membership in a team
type TeamMembership struct {
	Team     Team      `json:"team"`
	Role     Role      `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// GroupMembership represents a user's membership in a group
type GroupMembership struct {
	Group    Group     `json:"group"`
	Role     Role      `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// UserService handles user-related database operations
type UserService struct {
	db *database.DB
}

// NewUserService creates a new user service
func NewUserService(db *database.DB) *UserService {
	return &UserService{db: db}
}

// Common errors
var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserInactive       = errors.New("user account is inactive")
)

// CreateUser creates a new user
func (s *UserService) CreateUser(user *User, password string) error {
	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	user.PasswordHash = string(hashedPassword)

	// Check if email already exists
	exists, err := s.db.Exists("SELECT 1 FROM users WHERE email = ?", user.Email)
	if err != nil {
		return fmt.Errorf("failed to check email existence: %w", err)
	}
	if exists {
		return ErrEmailAlreadyExists
	}

	// Insert user
	query := `
		INSERT INTO users (email, password_hash, first_name, last_name, is_active)
		VALUES (?, ?, ?, ?, ?)
	`

	id, err := s.db.Insert(query,
		user.Email,
		user.PasswordHash,
		user.FirstName,
		user.LastName,
		user.IsActive,
	)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	user.ID = int(id)

	// Load the created user to get timestamps
	return s.GetUserByID(user.ID, user)
}

// GetUserByID retrieves a user by ID
func (s *UserService) GetUserByID(id int, user *User) error {
	query := `
		SELECT id, email, password_hash, first_name, last_name, 
		       is_active, created_at, updated_at
		FROM users
		WHERE id = ?
	`

	err := s.db.QueryRowContext(context.Background(), query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	return nil
}

// GetUserByEmail retrieves a user by email
func (s *UserService) GetUserByEmail(email string, user *User) error {
	query := `
		SELECT id, email, password_hash, first_name, last_name, 
		       is_active, created_at, updated_at
		FROM users
		WHERE email = ?
	`

	err := s.db.QueryRowContext(context.Background(), query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	return nil
}

// UpdateUser updates user information
func (s *UserService) UpdateUser(user *User) error {
	query := `
		UPDATE users 
		SET email = ?, first_name = ?, last_name = ?, is_active = ?
		WHERE id = ?
	`

	affected, err := s.db.Update(query,
		user.Email,
		user.FirstName,
		user.LastName,
		user.IsActive,
		user.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	if affected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// UpdatePassword updates a user's password
func (s *UserService) UpdatePassword(userID int, newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	query := `UPDATE users SET password_hash = ? WHERE id = ?`

	affected, err := s.db.Update(query, string(hashedPassword), userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	if affected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// DeleteUser soft deletes a user by setting is_active to false
func (s *UserService) DeleteUser(userID int) error {
	query := `UPDATE users SET is_active = false WHERE id = ?`

	affected, err := s.db.Update(query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if affected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// ValidateCredentials validates user credentials
func (s *UserService) ValidateCredentials(email, password string) (*User, error) {
	user := &User{}
	if err := s.GetUserByEmail(email, user); err != nil {
		if err == ErrUserNotFound {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Check if user is active
	if !user.IsActive {
		return nil, ErrUserInactive
	}

	// Verify password
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	return user, nil
}

// GetUserTeams retrieves all teams a user belongs to
func (s *UserService) GetUserTeams(userID int) ([]TeamMembership, error) {
	query := `
		SELECT t.id, t.name, t.description, t.group_id, t.created_at, t.updated_at,
		       r.id, r.name, r.description,
		       ut.joined_at
		FROM user_teams ut
		JOIN teams t ON ut.team_id = t.id
		JOIN roles r ON ut.role_id = r.id
		WHERE ut.user_id = ?
		ORDER BY t.name
	`

	rows, err := s.db.GetMany(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user teams: %w", err)
	}
	defer rows.Close()

	var memberships []TeamMembership
	for rows.Next() {
		var tm TeamMembership
		var groupID sql.NullInt64

		err := rows.Scan(
			&tm.Team.ID,
			&tm.Team.Name,
			&tm.Team.Description,
			&groupID,
			&tm.Team.CreatedAt,
			&tm.Team.UpdatedAt,
			&tm.Role.ID,
			&tm.Role.Name,
			&tm.Role.Description,
			&tm.JoinedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team membership: %w", err)
		}

		if groupID.Valid {
			tm.Team.GroupID = int(groupID.Int64)
		}

		memberships = append(memberships, tm)
	}

	return memberships, nil
}

// GetUserGroups retrieves all groups a user belongs to
func (s *UserService) GetUserGroups(userID int) ([]GroupMembership, error) {
	query := `
		SELECT g.id, g.name, g.description, g.created_at, g.updated_at,
		       r.id, r.name, r.description,
		       ug.joined_at
		FROM user_groups ug
		JOIN groups g ON ug.group_id = g.id
		JOIN roles r ON ug.role_id = r.id
		WHERE ug.user_id = ?
		ORDER BY g.name
	`

	rows, err := s.db.GetMany(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user groups: %w", err)
	}
	defer rows.Close()

	var memberships []GroupMembership
	for rows.Next() {
		var gm GroupMembership

		err := rows.Scan(
			&gm.Group.ID,
			&gm.Group.Name,
			&gm.Group.Description,
			&gm.Group.CreatedAt,
			&gm.Group.UpdatedAt,
			&gm.Role.ID,
			&gm.Role.Name,
			&gm.Role.Description,
			&gm.JoinedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan group membership: %w", err)
		}

		memberships = append(memberships, gm)
	}

	return memberships, nil
}

// AddUserToTeam adds a user to a team with a specific role
func (s *UserService) AddUserToTeam(userID, teamID, roleID int) error {
	query := `
		INSERT INTO user_teams (user_id, team_id, role_id)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE role_id = VALUES(role_id)
	`

	_, err := s.db.Insert(query, userID, teamID, roleID)
	if err != nil {
		return fmt.Errorf("failed to add user to team: %w", err)
	}

	return nil
}

// RemoveUserFromTeam removes a user from a team
func (s *UserService) RemoveUserFromTeam(userID, teamID int) error {
	query := `DELETE FROM user_teams WHERE user_id = ? AND team_id = ?`

	affected, err := s.db.Delete(query, userID, teamID)
	if err != nil {
		return fmt.Errorf("failed to remove user from team: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("user not found in team")
	}

	return nil
}

// ListUsers returns a paginated list of users
func (s *UserService) ListUsers(offset, limit int, activeOnly bool) ([]User, int, error) {
	// Build query
	whereClause := ""
	args := []interface{}{}

	if activeOnly {
		whereClause = "WHERE is_active = true"
	}

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users %s", whereClause)
	var totalCount int
	err := s.db.QueryRowContext(context.Background(), countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user count: %w", err)
	}

	// Get users
	query := fmt.Sprintf(`
		SELECT id, email, password_hash, first_name, last_name, 
		       is_active, created_at, updated_at
		FROM users
		%s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, limit, offset)
	rows, err := s.db.GetMany(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.PasswordHash,
			&user.FirstName,
			&user.LastName,
			&user.IsActive,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	return users, totalCount, nil
}
