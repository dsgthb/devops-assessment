package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"devops-assessment/internal/database"
)

// Assessment represents a DevOps maturity assessment
type Assessment struct {
	ID          int        `json:"id"`
	TeamID      int        `json:"team_id"`
	CreatedBy   int        `json:"created_by"`
	SessionID   string     `json:"session_id"`
	Status      string     `json:"status"` // 'in_progress' or 'completed'
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Relationships (loaded separately)
	Team          *Team          `json:"team,omitempty"`
	Creator       *User          `json:"creator,omitempty"`
	Responses     []Response     `json:"responses,omitempty"`
	SectionScores []SectionScore `json:"section_scores,omitempty"`
}

// Response represents an answer to a survey question
type Response struct {
	ID           int       `json:"id"`
	AssessmentID int       `json:"assessment_id"`
	QuestionID   string    `json:"question_id"` // e.g., 'S1-Q1'
	AnswerIDs    []string  `json:"answer_ids"`  // Array of answer IDs
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SectionScore represents the score for a section
type SectionScore struct {
	ID           int       `json:"id"`
	AssessmentID int       `json:"assessment_id"`
	SectionName  string    `json:"section_name"`
	Score        float64   `json:"score"`
	MaxScore     float64   `json:"max_score"`
	Percentage   float64   `json:"percentage"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AssessmentService handles assessment-related database operations
type AssessmentService struct {
	db *database.DB
}

// NewAssessmentService creates a new assessment service
func NewAssessmentService(db *database.DB) *AssessmentService {
	return &AssessmentService{db: db}
}

// Assessment status constants
const (
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
)

// Common errors
var (
	ErrAssessmentNotFound = errors.New("assessment not found")
	ErrInvalidStatus      = errors.New("invalid assessment status")
)

// CreateAssessment creates a new assessment
func (s *AssessmentService) CreateAssessment(assessment *Assessment) error {
	// Validate status
	if assessment.Status == "" {
		assessment.Status = StatusInProgress
	}

	if assessment.Status != StatusInProgress && assessment.Status != StatusCompleted {
		return ErrInvalidStatus
	}

	// Generate session ID if not provided
	if assessment.SessionID == "" {
		assessment.SessionID = generateSessionID()
	}

	// Insert assessment
	query := `
		INSERT INTO assessments (team_id, created_by, session_id, status)
		VALUES (?, ?, ?, ?)
	`

	id, err := s.db.Insert(query,
		assessment.TeamID,
		assessment.CreatedBy,
		assessment.SessionID,
		assessment.Status,
	)
	if err != nil {
		return fmt.Errorf("failed to create assessment: %w", err)
	}

	assessment.ID = int(id)

	// Load the created assessment to get timestamps
	return s.GetAssessmentByID(assessment.ID, assessment)
}

// GetAssessmentByID retrieves an assessment by ID
func (s *AssessmentService) GetAssessmentByID(id int, assessment *Assessment) error {
	query := `
		SELECT id, team_id, created_by, session_id, status, 
		       created_at, completed_at
		FROM assessments
		WHERE id = ?
	`

	var completedAt sql.NullTime
	err := s.db.QueryRowContext(context.Background(), query, id).Scan(
		&assessment.ID,
		&assessment.TeamID,
		&assessment.CreatedBy,
		&assessment.SessionID,
		&assessment.Status,
		&assessment.CreatedAt,
		&completedAt,
	)

	if err == sql.ErrNoRows {
		return ErrAssessmentNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	if completedAt.Valid {
		assessment.CompletedAt = &completedAt.Time
	}

	return nil
}

// GetAssessmentBySessionID retrieves an assessment by session ID
func (s *AssessmentService) GetAssessmentBySessionID(sessionID string, assessment *Assessment) error {
	query := `
		SELECT id, team_id, created_by, session_id, status, 
		       created_at, completed_at
		FROM assessments
		WHERE session_id = ?
	`

	var completedAt sql.NullTime
	err := s.db.QueryRowContext(context.Background(), query, sessionID).Scan(
		&assessment.ID,
		&assessment.TeamID,
		&assessment.CreatedBy,
		&assessment.SessionID,
		&assessment.Status,
		&assessment.CreatedAt,
		&completedAt,
	)

	if err == sql.ErrNoRows {
		return ErrAssessmentNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	if completedAt.Valid {
		assessment.CompletedAt = &completedAt.Time
	}

	return nil
}

// SaveResponse saves or updates a response for an assessment
func (s *AssessmentService) SaveResponse(response *Response) error {
	// Convert answer IDs to JSON
	answerJSON, err := json.Marshal(response.AnswerIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal answer IDs: %w", err)
	}

	// Use INSERT ... ON DUPLICATE KEY UPDATE
	query := `
		INSERT INTO responses (assessment_id, question_id, answer_ids)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE 
			answer_ids = VALUES(answer_ids),
			updated_at = CURRENT_TIMESTAMP
	`

	_, err = s.db.Insert(query, response.AssessmentID, response.QuestionID, string(answerJSON))
	if err != nil {
		return fmt.Errorf("failed to save response: %w", err)
	}

	return nil
}

// GetAssessmentResponses retrieves all responses for an assessment
func (s *AssessmentService) GetAssessmentResponses(assessmentID int) ([]Response, error) {
	query := `
		SELECT id, assessment_id, question_id, answer_ids, created_at, updated_at
		FROM responses
		WHERE assessment_id = ?
		ORDER BY question_id
	`

	rows, err := s.db.GetMany(query, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get responses: %w", err)
	}
	defer rows.Close()

	var responses []Response
	for rows.Next() {
		var response Response
		var answerJSON string

		err := rows.Scan(
			&response.ID,
			&response.AssessmentID,
			&response.QuestionID,
			&answerJSON,
			&response.CreatedAt,
			&response.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan response: %w", err)
		}

		// Parse answer IDs from JSON
		if err := json.Unmarshal([]byte(answerJSON), &response.AnswerIDs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal answer IDs: %w", err)
		}

		responses = append(responses, response)
	}

	return responses, nil
}

// SaveSectionScore saves or updates a section score
func (s *AssessmentService) SaveSectionScore(score *SectionScore) error {
	query := `
		INSERT INTO section_scores (assessment_id, section_name, score, max_score, percentage)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE 
			score = VALUES(score),
			max_score = VALUES(max_score),
			percentage = VALUES(percentage),
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := s.db.Insert(query,
		score.AssessmentID,
		score.SectionName,
		score.Score,
		score.MaxScore,
		score.Percentage,
	)
	if err != nil {
		return fmt.Errorf("failed to save section score: %w", err)
	}

	return nil
}

// GetAssessmentScores retrieves all section scores for an assessment
func (s *AssessmentService) GetAssessmentScores(assessmentID int) ([]SectionScore, error) {
	query := `
		SELECT id, assessment_id, section_name, score, max_score, percentage,
		       created_at, updated_at
		FROM section_scores
		WHERE assessment_id = ?
		ORDER BY section_name
	`

	rows, err := s.db.GetMany(query, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get section scores: %w", err)
	}
	defer rows.Close()

	var scores []SectionScore
	for rows.Next() {
		var score SectionScore

		err := rows.Scan(
			&score.ID,
			&score.AssessmentID,
			&score.SectionName,
			&score.Score,
			&score.MaxScore,
			&score.Percentage,
			&score.CreatedAt,
			&score.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan section score: %w", err)
		}

		scores = append(scores, score)
	}

	return scores, nil
}

// CompleteAssessment marks an assessment as completed
func (s *AssessmentService) CompleteAssessment(assessmentID int) error {
	query := `
		UPDATE assessments 
		SET status = ?, completed_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = ?
	`

	affected, err := s.db.Update(query, StatusCompleted, assessmentID, StatusInProgress)
	if err != nil {
		return fmt.Errorf("failed to complete assessment: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("assessment not found or already completed")
	}

	return nil
}

// DeleteAssessment deletes an assessment and all related data
func (s *AssessmentService) DeleteAssessment(assessmentID int) error {
	// Foreign key constraints will handle cascade deletion
	query := `DELETE FROM assessments WHERE id = ?`

	affected, err := s.db.Delete(query, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to delete assessment: %w", err)
	}

	if affected == 0 {
		return ErrAssessmentNotFound
	}

	return nil
}

// ListTeamAssessments returns assessments for a specific team
func (s *AssessmentService) ListTeamAssessments(teamID int, includeInProgress bool) ([]Assessment, error) {
	query := `
		SELECT id, team_id, created_by, session_id, status, 
		       created_at, completed_at
		FROM assessments
		WHERE team_id = ?
	`

	args := []interface{}{teamID}

	if !includeInProgress {
		query += " AND status = ?"
		args = append(args, StatusCompleted)
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.db.GetMany(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list team assessments: %w", err)
	}
	defer rows.Close()

	var assessments []Assessment
	for rows.Next() {
		var assessment Assessment
		var completedAt sql.NullTime

		err := rows.Scan(
			&assessment.ID,
			&assessment.TeamID,
			&assessment.CreatedBy,
			&assessment.SessionID,
			&assessment.Status,
			&assessment.CreatedAt,
			&completedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan assessment: %w", err)
		}

		if completedAt.Valid {
			assessment.CompletedAt = &completedAt.Time
		}

		assessments = append(assessments, assessment)
	}

	return assessments, nil
}

// ListUserAssessments returns assessments created by a specific user
func (s *AssessmentService) ListUserAssessments(userID int, offset, limit int) ([]Assessment, int, error) {
	// Get total count
	countQuery := "SELECT COUNT(*) FROM assessments WHERE created_by = ?"
	var totalCount int
	err := s.db.QueryRowContext(context.Background(), countQuery, userID).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get assessment count: %w", err)
	}

	// Get assessments
	query := `
		SELECT id, team_id, created_by, session_id, status, 
		       created_at, completed_at
		FROM assessments
		WHERE created_by = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.GetMany(query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list user assessments: %w", err)
	}
	defer rows.Close()

	var assessments []Assessment
	for rows.Next() {
		var assessment Assessment
		var completedAt sql.NullTime

		err := rows.Scan(
			&assessment.ID,
			&assessment.TeamID,
			&assessment.CreatedBy,
			&assessment.SessionID,
			&assessment.Status,
			&assessment.CreatedAt,
			&completedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan assessment: %w", err)
		}

		if completedAt.Valid {
			assessment.CompletedAt = &completedAt.Time
		}

		assessments = append(assessments, assessment)
	}

	return assessments, totalCount, nil
}

// GetLatestTeamAssessment gets the most recent completed assessment for a team
func (s *AssessmentService) GetLatestTeamAssessment(teamID int) (*Assessment, error) {
	query := `
		SELECT id, team_id, created_by, session_id, status, 
		       created_at, completed_at
		FROM assessments
		WHERE team_id = ? AND status = ?
		ORDER BY completed_at DESC
		LIMIT 1
	`

	assessment := &Assessment{}
	var completedAt sql.NullTime

	err := s.db.QueryRowContext(context.Background(), query, teamID, StatusCompleted).Scan(
		&assessment.ID,
		&assessment.TeamID,
		&assessment.CreatedBy,
		&assessment.SessionID,
		&assessment.Status,
		&assessment.CreatedAt,
		&completedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrAssessmentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest assessment: %w", err)
	}

	if completedAt.Valid {
		assessment.CompletedAt = &completedAt.Time
	}

	return assessment, nil
}
