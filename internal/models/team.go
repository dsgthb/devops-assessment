package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dsgthb/devops-assessment/internal/database"
)

// Team represents an organizational team
type Team struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	GroupID     int       `json:"group_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relationships (loaded separately)
	Group       *Group       `json:"group,omitempty"`
	Members     []TeamMember `json:"members,omitempty"`
	Assessments []Assessment `json:"assessments,omitempty"`
}

// TeamMember represents a member of a team
type TeamMember struct {
	User     User      `json:"user"`
	Role     Role      `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// Group represents an organizational group (collection of teams)
type Group struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relationships (loaded separately)
	Teams   []Team        `json:"teams,omitempty"`
	Members []GroupMember `json:"members,omitempty"`
}

// GroupMember represents a member of a group
type GroupMember struct {
	User     User      `json:"user"`
	Role     Role      `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// TeamService handles team-related database operations
type TeamService struct {
	db *database.DB
}

// GroupService handles group-related database operations
type GroupService struct {
	db *database.DB
}

// NewTeamService creates a new team service
func NewTeamService(db *database.DB) *TeamService {
	return &TeamService{db: db}
}

// NewGroupService creates a new group service
func NewGroupService(db *database.DB) *GroupService {
	return &GroupService{db: db}
}

// Common errors
var (
	ErrTeamNotFound    = errors.New("team not found")
	ErrGroupNotFound   = errors.New("group not found")
	ErrTeamNameExists  = errors.New("team name already exists")
	ErrGroupNameExists = errors.New("group name already exists")
)

// Team Service Methods

// CreateTeam creates a new team
func (s *TeamService) CreateTeam(team *Team) error {
	// Check if team name already exists
	exists, err := s.db.Exists("SELECT 1 FROM teams WHERE name = ?", team.Name)
	if err != nil {
		return fmt.Errorf("failed to check team name existence: %w", err)
	}
	if exists {
		return ErrTeamNameExists
	}

	// Insert team
	query := `
		INSERT INTO teams (name, description, group_id)
		VALUES (?, ?, ?)
	`

	var groupID interface{}
	if team.GroupID > 0 {
		groupID = team.GroupID
	} else {
		groupID = nil
	}

	id, err := s.db.Insert(query, team.Name, team.Description, groupID)
	if err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}

	team.ID = int(id)

	// Load the created team to get timestamps
	return s.GetTeamByID(team.ID, team)
}

// GetTeamByID retrieves a team by ID
func (s *TeamService) GetTeamByID(id int, team *Team) error {
	query := `
		SELECT id, name, description, group_id, created_at, updated_at
		FROM teams
		WHERE id = ?
	`

	var groupID sql.NullInt64
	err := s.db.QueryRowContext(context.Background(), query, id).Scan(
		&team.ID,
		&team.Name,
		&team.Description,
		&groupID,
		&team.CreatedAt,
		&team.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return ErrTeamNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to get team: %w", err)
	}

	if groupID.Valid {
		team.GroupID = int(groupID.Int64)
	}

	return nil
}

// UpdateTeam updates team information
func (s *TeamService) UpdateTeam(team *Team) error {
	query := `
		UPDATE teams 
		SET name = ?, description = ?, group_id = ?
		WHERE id = ?
	`

	var groupID interface{}
	if team.GroupID > 0 {
		groupID = team.GroupID
	} else {
		groupID = nil
	}

	affected, err := s.db.Update(query, team.Name, team.Description, groupID, team.ID)
	if err != nil {
		return fmt.Errorf("failed to update team: %w", err)
	}

	if affected == 0 {
		return ErrTeamNotFound
	}

	return nil
}

// DeleteTeam deletes a team
func (s *TeamService) DeleteTeam(teamID int) error {
	// Check if team has assessments
	hasAssessments, err := s.db.Exists("SELECT 1 FROM assessments WHERE team_id = ?", teamID)
	if err != nil {
		return fmt.Errorf("failed to check team assessments: %w", err)
	}
	if hasAssessments {
		return fmt.Errorf("cannot delete team with existing assessments")
	}

	query := `DELETE FROM teams WHERE id = ?`

	affected, err := s.db.Delete(query, teamID)
	if err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}

	if affected == 0 {
		return ErrTeamNotFound
	}

	return nil
}

// GetTeamMembers retrieves all members of a team
func (s *TeamService) GetTeamMembers(teamID int) ([]TeamMember, error) {
	query := `
		SELECT u.id, u.email, u.first_name, u.last_name, u.is_active,
		       u.created_at, u.updated_at,
		       r.id, r.name, r.description,
		       ut.joined_at
		FROM user_teams ut
		JOIN users u ON ut.user_id = u.id
		JOIN roles r ON ut.role_id = r.id
		WHERE ut.team_id = ?
		ORDER BY u.last_name, u.first_name
	`

	rows, err := s.db.GetMany(query, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}
	defer rows.Close()

	var members []TeamMember
	for rows.Next() {
		var member TeamMember

		err := rows.Scan(
			&member.User.ID,
			&member.User.Email,
			&member.User.FirstName,
			&member.User.LastName,
			&member.User.IsActive,
			&member.User.CreatedAt,
			&member.User.UpdatedAt,
			&member.Role.ID,
			&member.Role.Name,
			&member.Role.Description,
			&member.JoinedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team member: %w", err)
		}

		members = append(members, member)
	}

	return members, nil
}

// ListTeams returns a paginated list of teams
func (s *TeamService) ListTeams(offset, limit int, groupID *int) ([]Team, int, error) {
	// Build query
	whereClause := ""
	args := []interface{}{}

	if groupID != nil {
		whereClause = "WHERE group_id = ?"
		args = append(args, *groupID)
	}

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM teams %s", whereClause)
	var totalCount int
	err := s.db.QueryRowContext(context.Background(), countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get team count: %w", err)
	}

	// Get teams
	query := fmt.Sprintf(`
		SELECT id, name, description, group_id, created_at, updated_at
		FROM teams
		%s
		ORDER BY name
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, limit, offset)
	rows, err := s.db.GetMany(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list teams: %w", err)
	}
	defer rows.Close()

	var teams []Team
	for rows.Next() {
		var team Team
		var groupID sql.NullInt64

		err := rows.Scan(
			&team.ID,
			&team.Name,
			&team.Description,
			&groupID,
			&team.CreatedAt,
			&team.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan team: %w", err)
		}

		if groupID.Valid {
			team.GroupID = int(groupID.Int64)
		}

		teams = append(teams, team)
	}

	return teams, totalCount, nil
}

// Group Service Methods

// CreateGroup creates a new group
func (s *GroupService) CreateGroup(group *Group) error {
	// Check if group name already exists
	exists, err := s.db.Exists("SELECT 1 FROM groups WHERE name = ?", group.Name)
	if err != nil {
		return fmt.Errorf("failed to check group name existence: %w", err)
	}
	if exists {
		return ErrGroupNameExists
	}

	// Insert group
	query := `
		INSERT INTO groups (name, description)
		VALUES (?, ?)
	`

	id, err := s.db.Insert(query, group.Name, group.Description)
	if err != nil {
		return fmt.Errorf("failed to create group: %w", err)
	}

	group.ID = int(id)

	// Load the created group to get timestamps
	return s.GetGroupByID(group.ID, group)
}

// GetGroupByID retrieves a group by ID
func (s *GroupService) GetGroupByID(id int, group *Group) error {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM groups
		WHERE id = ?
	`

	err := s.db.QueryRowContext(context.Background(), query, id).Scan(
		&group.ID,
		&group.Name,
		&group.Description,
		&group.CreatedAt,
		&group.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return ErrGroupNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to get group: %w", err)
	}

	return nil
}

// UpdateGroup updates group information
func (s *GroupService) UpdateGroup(group *Group) error {
	query := `
		UPDATE groups 
		SET name = ?, description = ?
		WHERE id = ?
	`

	affected, err := s.db.Update(query, group.Name, group.Description, group.ID)
	if err != nil {
		return fmt.Errorf("failed to update group: %w", err)
	}

	if affected == 0 {
		return ErrGroupNotFound
	}

	return nil
}

// DeleteGroup deletes a group
func (s *GroupService) DeleteGroup(groupID int) error {
	// Check if group has teams
	hasTeams, err := s.db.Exists("SELECT 1 FROM teams WHERE group_id = ?", groupID)
	if err != nil {
		return fmt.Errorf("failed to check group teams: %w", err)
	}
	if hasTeams {
		return fmt.Errorf("cannot delete group with existing teams")
	}

	query := `DELETE FROM groups WHERE id = ?`

	affected, err := s.db.Delete(query, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}

	if affected == 0 {
		return ErrGroupNotFound
	}

	return nil
}

// GetGroupMembers retrieves all members of a group
func (s *GroupService) GetGroupMembers(groupID int) ([]GroupMember, error) {
	query := `
		SELECT u.id, u.email, u.first_name, u.last_name, u.is_active,
		       u.created_at, u.updated_at,
		       r.id, r.name, r.description,
		       ug.joined_at
		FROM user_groups ug
		JOIN users u ON ug.user_id = u.id
		JOIN roles r ON ug.role_id = r.id
		WHERE ug.group_id = ?
		ORDER BY u.last_name, u.first_name
	`

	rows, err := s.db.GetMany(query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group members: %w", err)
	}
	defer rows.Close()

	var members []GroupMember
	for rows.Next() {
		var member GroupMember

		err := rows.Scan(
			&member.User.ID,
			&member.User.Email,
			&member.User.FirstName,
			&member.User.LastName,
			&member.User.IsActive,
			&member.User.CreatedAt,
			&member.User.UpdatedAt,
			&member.Role.ID,
			&member.Role.Name,
			&member.Role.Description,
			&member.JoinedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan group member: %w", err)
		}

		members = append(members, member)
	}

	return members, nil
}

// GetGroupTeams retrieves all teams in a group
func (s *GroupService) GetGroupTeams(groupID int) ([]Team, error) {
	query := `
		SELECT id, name, description, group_id, created_at, updated_at
		FROM teams
		WHERE group_id = ?
		ORDER BY name
	`

	rows, err := s.db.GetMany(query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group teams: %w", err)
	}
	defer rows.Close()

	var teams []Team
	for rows.Next() {
		var team Team
		var gID sql.NullInt64

		err := rows.Scan(
			&team.ID,
			&team.Name,
			&team.Description,
			&gID,
			&team.CreatedAt,
			&team.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team: %w", err)
		}

		if gID.Valid {
			team.GroupID = int(gID.Int64)
		}

		teams = append(teams, team)
	}

	return teams, nil
}

// ListGroups returns a paginated list of groups
func (s *GroupService) ListGroups(offset, limit int) ([]Group, int, error) {
	// Get total count
	countQuery := "SELECT COUNT(*) FROM groups"
	var totalCount int
	err := s.db.QueryRowContext(context.Background(), countQuery).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get group count: %w", err)
	}

	// Get groups
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM groups
		ORDER BY name
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.GetMany(query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list groups: %w", err)
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var group Group

		err := rows.Scan(
			&group.ID,
			&group.Name,
			&group.Description,
			&group.CreatedAt,
			&group.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan group: %w", err)
		}

		groups = append(groups, group)
	}

	return groups, totalCount, nil
}
