package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dsgthb/devops-assessment/internal/database"
)

// Role represents a system role
type Role struct {
	ID          int          `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	CreatedAt   time.Time    `json:"created_at"`
	Permissions []Permission `json:"permissions,omitempty"`
}

// Permission represents a system permission
type Permission struct {
	ID          int       `json:"id"`
	Resource    string    `json:"resource"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// RoleService handles role-related database operations
type RoleService struct {
	db *database.DB
}

// RBACService handles role-based access control operations
type RBACService struct {
	db          *database.DB
	roleService *RoleService
}

// NewRoleService creates a new role service
func NewRoleService(db *database.DB) *RoleService {
	return &RoleService{db: db}
}

// NewRBACService creates a new RBAC service
func NewRBACService(db *database.DB) *RBACService {
	return &RBACService{
		db:          db,
		roleService: NewRoleService(db),
	}
}

// Common errors
var (
	ErrRoleNotFound       = errors.New("role not found")
	ErrPermissionNotFound = errors.New("permission not found")
	ErrAccessDenied       = errors.New("access denied")
)

// Predefined role constants
const (
	RoleAdmin  = "admin"
	RoleEditor = "editor"
	RoleViewer = "viewer"
)

// Permission resources and actions
const (
	ResourceUser       = "user"
	ResourceTeam       = "team"
	ResourceGroup      = "group"
	ResourceAssessment = "assessment"
	ResourceReport     = "report"
	ResourceSystem     = "system"
	ResourceAudit      = "audit"

	ActionCreate = "create"
	ActionRead   = "read"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionExport = "export"
	ActionManage = "manage"
)

// Role Service Methods

// GetRoleByID retrieves a role by ID
func (s *RoleService) GetRoleByID(id int) (*Role, error) {
	query := `
		SELECT id, name, description, created_at
		FROM roles
		WHERE id = ?
	`

	role := &Role{}
	err := s.db.QueryRowContext(context.Background(), query, id).Scan(
		&role.ID,
		&role.Name,
		&role.Description,
		&role.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrRoleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	// Load permissions
	permissions, err := s.GetRolePermissions(role.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role permissions: %w", err)
	}
	role.Permissions = permissions

	return role, nil
}

// GetRoleByName retrieves a role by name
func (s *RoleService) GetRoleByName(name string) (*Role, error) {
	query := `
		SELECT id, name, description, created_at
		FROM roles
		WHERE name = ?
	`

	role := &Role{}
	err := s.db.QueryRowContext(context.Background(), query, name).Scan(
		&role.ID,
		&role.Name,
		&role.Description,
		&role.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrRoleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	// Load permissions
	permissions, err := s.GetRolePermissions(role.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role permissions: %w", err)
	}
	role.Permissions = permissions

	return role, nil
}

// GetRolePermissions retrieves all permissions for a role
func (s *RoleService) GetRolePermissions(roleID int) ([]Permission, error) {
	query := `
		SELECT p.id, p.resource, p.action, p.description, p.created_at
		FROM permissions p
		JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = ?
		ORDER BY p.resource, p.action
	`

	rows, err := s.db.GetMany(query, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role permissions: %w", err)
	}
	defer rows.Close()

	var permissions []Permission
	for rows.Next() {
		var perm Permission
		err := rows.Scan(
			&perm.ID,
			&perm.Resource,
			&perm.Action,
			&perm.Description,
			&perm.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions = append(permissions, perm)
	}

	return permissions, nil
}

// ListRoles returns all available roles
func (s *RoleService) ListRoles() ([]Role, error) {
	query := `
		SELECT id, name, description, created_at
		FROM roles
		ORDER BY name
	`

	rows, err := s.db.GetMany(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var role Role
		err := rows.Scan(
			&role.ID,
			&role.Name,
			&role.Description,
			&role.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}

		// Load permissions for each role
		permissions, err := s.GetRolePermissions(role.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get permissions for role %d: %w", role.ID, err)
		}
		role.Permissions = permissions

		roles = append(roles, role)
	}

	return roles, nil
}

// RBAC Service Methods

// CheckUserPermission checks if a user has a specific permission
func (s *RBACService) CheckUserPermission(userID int, resource, action string) (bool, error) {
	query := `
		SELECT COUNT(*) > 0
		FROM (
			-- Check team permissions
			SELECT p.id
			FROM user_teams ut
			JOIN role_permissions rp ON ut.role_id = rp.role_id
			JOIN permissions p ON rp.permission_id = p.id
			WHERE ut.user_id = ? AND p.resource = ? AND p.action = ?
			
			UNION
			
			-- Check group permissions
			SELECT p.id
			FROM user_groups ug
			JOIN role_permissions rp ON ug.role_id = rp.role_id
			JOIN permissions p ON rp.permission_id = p.id
			WHERE ug.user_id = ? AND p.resource = ? AND p.action = ?
		) AS combined_permissions
	`

	var hasPermission bool
	err := s.db.QueryRowContext(
		context.Background(),
		query,
		userID, resource, action,
		userID, resource, action,
	).Scan(&hasPermission)

	if err != nil {
		return false, fmt.Errorf("failed to check user permission: %w", err)
	}

	return hasPermission, nil
}

// GetUserPermissions retrieves all permissions for a user
func (s *RBACService) GetUserPermissions(userID int) ([]Permission, error) {
	query := `
		SELECT DISTINCT p.id, p.resource, p.action, p.description, p.created_at
		FROM (
			-- Get permissions from team roles
			SELECT p.*
			FROM user_teams ut
			JOIN role_permissions rp ON ut.role_id = rp.role_id
			JOIN permissions p ON rp.permission_id = p.id
			WHERE ut.user_id = ?
			
			UNION
			
			-- Get permissions from group roles
			SELECT p.*
			FROM user_groups ug
			JOIN role_permissions rp ON ug.role_id = rp.role_id
			JOIN permissions p ON rp.permission_id = p.id
			WHERE ug.user_id = ?
		) AS p
		ORDER BY p.resource, p.action
	`

	rows, err := s.db.GetMany(query, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user permissions: %w", err)
	}
	defer rows.Close()

	var permissions []Permission
	for rows.Next() {
		var perm Permission
		err := rows.Scan(
			&perm.ID,
			&perm.Resource,
			&perm.Action,
			&perm.Description,
			&perm.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions = append(permissions, perm)
	}

	return permissions, nil
}

// GetUserRoles retrieves all roles assigned to a user (from teams and groups)
func (s *RBACService) GetUserRoles(userID int) ([]Role, error) {
	query := `
		SELECT DISTINCT r.id, r.name, r.description, r.created_at
		FROM (
			-- Get roles from teams
			SELECT r.*
			FROM user_teams ut
			JOIN roles r ON ut.role_id = r.id
			WHERE ut.user_id = ?
			
			UNION
			
			-- Get roles from groups
			SELECT r.*
			FROM user_groups ug
			JOIN roles r ON ug.role_id = r.id
			WHERE ug.user_id = ?
		) AS r
		ORDER BY r.name
	`

	rows, err := s.db.GetMany(query, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var role Role
		err := rows.Scan(
			&role.ID,
			&role.Name,
			&role.Description,
			&role.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}

		// Load permissions for each role
		permissions, err := s.roleService.GetRolePermissions(role.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get permissions for role %d: %w", role.ID, err)
		}
		role.Permissions = permissions

		roles = append(roles, role)
	}

	return roles, nil
}

// IsUserAdmin checks if a user has admin role in any team or group
func (s *RBACService) IsUserAdmin(userID int) (bool, error) {
	query := `
		SELECT COUNT(*) > 0
		FROM (
			SELECT r.id
			FROM user_teams ut
			JOIN roles r ON ut.role_id = r.id
			WHERE ut.user_id = ? AND r.name = ?
			
			UNION
			
			SELECT r.id
			FROM user_groups ug
			JOIN roles r ON ug.role_id = r.id
			WHERE ug.user_id = ? AND r.name = ?
		) AS admin_roles
	`

	var isAdmin bool
	err := s.db.QueryRowContext(
		context.Background(),
		query,
		userID, RoleAdmin,
		userID, RoleAdmin,
	).Scan(&isAdmin)

	if err != nil {
		return false, fmt.Errorf("failed to check admin status: %w", err)
	}

	return isAdmin, nil
}

// CheckTeamPermission checks if a user has a specific permission for a team
func (s *RBACService) CheckTeamPermission(userID, teamID int, resource, action string) (bool, error) {
	// First check if user is a member of the team
	query := `
		SELECT COUNT(*) > 0
		FROM user_teams ut
		JOIN role_permissions rp ON ut.role_id = rp.role_id
		JOIN permissions p ON rp.permission_id = p.id
		WHERE ut.user_id = ? AND ut.team_id = ? 
		AND p.resource = ? AND p.action = ?
	`

	var hasDirectPermission bool
	err := s.db.QueryRowContext(
		context.Background(),
		query,
		userID, teamID, resource, action,
	).Scan(&hasDirectPermission)

	if err != nil {
		return false, fmt.Errorf("failed to check team permission: %w", err)
	}

	if hasDirectPermission {
		return true, nil
	}

	// Check if user has permission through group membership
	query = `
		SELECT COUNT(*) > 0
		FROM teams t
		JOIN user_groups ug ON t.group_id = ug.group_id
		JOIN role_permissions rp ON ug.role_id = rp.role_id
		JOIN permissions p ON rp.permission_id = p.id
		WHERE t.id = ? AND ug.user_id = ?
		AND p.resource = ? AND p.action = ?
	`

	var hasGroupPermission bool
	err = s.db.QueryRowContext(
		context.Background(),
		query,
		teamID, userID, resource, action,
	).Scan(&hasGroupPermission)

	if err != nil {
		return false, fmt.Errorf("failed to check group permission: %w", err)
	}

	return hasGroupPermission, nil
}

// GetUserTeamRole gets the user's role in a specific team
func (s *RBACService) GetUserTeamRole(userID, teamID int) (*Role, error) {
	query := `
		SELECT r.id, r.name, r.description, r.created_at
		FROM user_teams ut
		JOIN roles r ON ut.role_id = r.id
		WHERE ut.user_id = ? AND ut.team_id = ?
	`

	role := &Role{}
	err := s.db.QueryRowContext(context.Background(), query, userID, teamID).Scan(
		&role.ID,
		&role.Name,
		&role.Description,
		&role.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found in team")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user team role: %w", err)
	}

	// Load permissions
	permissions, err := s.roleService.GetRolePermissions(role.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role permissions: %w", err)
	}
	role.Permissions = permissions

	return role, nil
}
