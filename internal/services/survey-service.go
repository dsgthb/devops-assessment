package services

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/dsgthb/devops-assessment/internal/database"
	"github.com/dsgthb/devops-assessment/internal/models"
)

// SurveyService handles survey-related business logic
type SurveyService struct {
	db                *database.DB
	assessmentService *models.AssessmentService
	questionService   *models.QuestionService
	teamService       *models.TeamService
}

// NewSurveyService creates a new survey service
func NewSurveyService(db *database.DB, questionsFile, adviceFile string) *SurveyService {
	return &SurveyService{
		db:                db,
		assessmentService: models.NewAssessmentService(db),
		questionService:   models.NewQuestionService(questionsFile, adviceFile),
		teamService:       models.NewTeamService(db),
	}
}

// StartAssessment creates a new assessment for a team
func (s *SurveyService) StartAssessment(teamID, userID int) (*models.Assessment, *models.Survey, error) {
	// Create new assessment
	assessment := &models.Assessment{
		TeamID:    teamID,
		CreatedBy: userID,
		Status:    models.StatusInProgress,
	}

	if err := s.assessmentService.CreateAssessment(assessment); err != nil {
		return nil, nil, fmt.Errorf("failed to create assessment: %w", err)
	}

	// Load survey questions
	survey, err := s.questionService.LoadQuestions()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load questions: %w", err)
	}

	return assessment, survey, nil
}

// ContinueAssessment loads an existing assessment
func (s *SurveyService) ContinueAssessment(assessmentID int) (*models.Assessment, *models.Survey, error) {
	// Load assessment
	assessment := &models.Assessment{}
	if err := s.assessmentService.GetAssessmentByID(assessmentID, assessment); err != nil {
		return nil, nil, err
	}

	// Check if assessment is still in progress
	if assessment.Status != models.StatusInProgress {
		return nil, nil, fmt.Errorf("assessment is already completed")
	}

	// Load survey questions
	survey, err := s.questionService.LoadQuestions()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load questions: %w", err)
	}

	// Load existing responses
	responses, err := s.assessmentService.GetAssessmentResponses(assessmentID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load responses: %w", err)
	}

	// Apply responses to survey
	if err := s.questionService.ApplyResponses(survey, responses); err != nil {
		return nil, nil, fmt.Errorf("failed to apply responses: %w", err)
	}

	return assessment, survey, nil
}

// SaveResponses saves responses for a specific section
func (s *SurveyService) SaveResponses(assessmentID int, sectionName string, formData map[string][]string) error {
	// Load survey questions
	survey, err := s.questionService.LoadQuestions()
	if err != nil {
		return fmt.Errorf("failed to load questions: %w", err)
	}

	// Find the section
	section, err := s.questionService.GetSectionByURLName(survey, sectionName)
	if err != nil {
		return err
	}

	// Process responses for each question in the section
	for _, question := range section.Questions {
		if question.Type == "Banner" || question.ID == "" {
			continue
		}

		response := &models.Response{
			AssessmentID: assessmentID,
			QuestionID:   question.ID,
			AnswerIDs:    []string{},
		}

		switch question.Type {
		case "Option":
			// Radio button - single value
			if values, exists := formData[question.ID]; exists && len(values) > 0 {
				response.AnswerIDs = []string{values[0]}
			}

		case "Checkbox":
			// Checkboxes - multiple values
			for _, answer := range question.Answers {
				if _, exists := formData[answer.ID]; exists {
					response.AnswerIDs = append(response.AnswerIDs, answer.ID)
				}
			}
		}

		// Save response if any answers were selected
		if len(response.AnswerIDs) > 0 {
			if err := s.assessmentService.SaveResponse(response); err != nil {
				return fmt.Errorf("failed to save response: %w", err)
			}
		}
	}

	return nil
}

// CalculateResults calculates and saves the assessment results
func (s *SurveyService) CalculateResults(assessmentID int) (*AssessmentResults, error) {
	// Load survey questions
	survey, err := s.questionService.LoadQuestions()
	if err != nil {
		return nil, fmt.Errorf("failed to load questions: %w", err)
	}

	// Load responses
	responses, err := s.assessmentService.GetAssessmentResponses(assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to load responses: %w", err)
	}

	// Apply responses to survey
	if err := s.questionService.ApplyResponses(survey, responses); err != nil {
		return nil, fmt.Errorf("failed to apply responses: %w", err)
	}

	// Calculate section scores
	sectionScores := s.questionService.CalculateSectionScores(survey, assessmentID)

	// Save section scores
	for _, score := range sectionScores {
		if err := s.assessmentService.SaveSectionScore(&score); err != nil {
			return nil, fmt.Errorf("failed to save section score: %w", err)
		}
	}

	// Mark assessment as completed
	if err := s.assessmentService.CompleteAssessment(assessmentID); err != nil {
		return nil, fmt.Errorf("failed to complete assessment: %w", err)
	}

	// Create results structure
	results := &AssessmentResults{
		AssessmentID:  assessmentID,
		SectionScores: sectionScores,
		Survey:        survey,
	}

	// Calculate subcategory scores for sections that have them
	results.SubCategoryScores = make(map[string][]models.SectionScore)
	for _, section := range survey.Sections {
		if section.HasSubCategories {
			subScores := s.questionService.CalculateSubCategoryScores(survey, section.SectionName, assessmentID)
			if len(subScores) > 0 {
				results.SubCategoryScores[section.SectionName] = subScores
			}
		}
	}

	return results, nil
}

// GetAssessmentResults retrieves calculated results for a completed assessment
func (s *SurveyService) GetAssessmentResults(assessmentID int) (*AssessmentResults, error) {
	// Load assessment
	assessment := &models.Assessment{}
	if err := s.assessmentService.GetAssessmentByID(assessmentID, assessment); err != nil {
		return nil, err
	}

	// Check if assessment is completed
	if assessment.Status != models.StatusCompleted {
		return nil, fmt.Errorf("assessment is not completed")
	}

	// Load section scores
	sectionScores, err := s.assessmentService.GetAssessmentScores(assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to load section scores: %w", err)
	}

	// Load survey for subcategory calculation
	survey, err := s.questionService.LoadQuestions()
	if err != nil {
		return nil, fmt.Errorf("failed to load questions: %w", err)
	}

	// Load responses to calculate subcategory scores
	responses, err := s.assessmentService.GetAssessmentResponses(assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to load responses: %w", err)
	}

	// Apply responses to survey
	if err := s.questionService.ApplyResponses(survey, responses); err != nil {
		return nil, fmt.Errorf("failed to apply responses: %w", err)
	}

	// Create results structure
	results := &AssessmentResults{
		AssessmentID:  assessmentID,
		SectionScores: sectionScores,
		Survey:        survey,
	}

	// Calculate subcategory scores
	results.SubCategoryScores = make(map[string][]models.SectionScore)
	for _, section := range survey.Sections {
		if section.HasSubCategories {
			subScores := s.questionService.CalculateSubCategoryScores(survey, section.SectionName, assessmentID)
			if len(subScores) > 0 {
				results.SubCategoryScores[section.SectionName] = subScores
			}
		}
	}

	// Load team information
	team := &models.Team{}
	if err := s.teamService.GetTeamByID(assessment.TeamID, team); err == nil {
		results.Team = team
	}

	return results, nil
}

// ExportAssessmentCSV exports assessment results to CSV format
func (s *SurveyService) ExportAssessmentCSV(assessmentID int, writer io.Writer) error {
	// Get assessment results
	results, err := s.GetAssessmentResults(assessmentID)
	if err != nil {
		return err
	}

	// Create CSV writer
	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	// Write header
	header := []string{
		"Section",
		"Sub Category",
		"Question",
		"Possible Answers",
		"Max Score",
		"Answer(s)",
		"Score",
	}
	if err := csvWriter.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Load responses
	responses, err := s.assessmentService.GetAssessmentResponses(assessmentID)
	if err != nil {
		return fmt.Errorf("failed to load responses: %w", err)
	}

	// Create response map for quick lookup
	responseMap := make(map[string][]string)
	for _, response := range responses {
		responseMap[response.QuestionID] = response.AnswerIDs
	}

	// Write data rows
	for _, section := range results.Survey.Sections {
		for _, question := range section.Questions {
			if len(question.Answers) == 0 {
				continue // Skip questions without answers
			}

			// Build possible answers string
			var possibleAnswers strings.Builder
			switch question.Type {
			case "Option":
				possibleAnswers.WriteString("Choose one of:\n")
			case "Checkbox":
				possibleAnswers.WriteString("Choose all that apply:\n")
			}

			for _, answer := range question.Answers {
				possibleAnswers.WriteString(fmt.Sprintf("%s (%.1f)\n", answer.Answer, answer.Score))
			}

			// Get selected answers
			var selectedAnswers strings.Builder
			if answerIDs, exists := responseMap[question.ID]; exists {
				for _, answerID := range answerIDs {
					for _, answer := range question.Answers {
						if answer.ID == answerID {
							selectedAnswers.WriteString(answer.Answer + "\n")
							break
						}
					}
				}
			}

			// Calculate scores
			maxScore := s.questionService.CalculateQuestionMaxScore(&question)
			score := 0.0
			if answerIDs, exists := responseMap[question.ID]; exists {
				for _, answerID := range answerIDs {
					for _, answer := range question.Answers {
						if answer.ID == answerID && answer.Value == "checked" {
							score += answer.Score
						}
					}
				}
			}

			// Write row
			row := []string{
				section.SectionName,
				question.SubCategory,
				question.QuestionText,
				strings.TrimSpace(possibleAnswers.String()),
				strconv.FormatFloat(maxScore, 'f', 1, 64),
				strings.TrimSpace(selectedAnswers.String()),
				strconv.FormatFloat(score, 'f', 1, 64),
			}

			if err := csvWriter.Write(row); err != nil {
				return fmt.Errorf("failed to write CSV row: %w", err)
			}
		}
	}

	return nil
}

// GetTeamAssessmentHistory gets assessment history for a team
func (s *SurveyService) GetTeamAssessmentHistory(teamID int) ([]AssessmentSummary, error) {
	assessments, err := s.assessmentService.ListTeamAssessments(teamID, false)
	if err != nil {
		return nil, err
	}

	summaries := make([]AssessmentSummary, 0, len(assessments))
	for _, assessment := range assessments {
		if assessment.Status == models.StatusCompleted {
			scores, err := s.assessmentService.GetAssessmentScores(assessment.ID)
			if err != nil {
				continue
			}

			summary := AssessmentSummary{
				Assessment:    assessment,
				SectionScores: scores,
				OverallScore:  calculateOverallScore(scores),
			}

			summaries = append(summaries, summary)
		}
	}

	return summaries, nil
}

// Helper structures

// AssessmentResults contains complete assessment results
type AssessmentResults struct {
	AssessmentID      int                              `json:"assessment_id"`
	Team              *models.Team                     `json:"team,omitempty"`
	SectionScores     []models.SectionScore            `json:"section_scores"`
	SubCategoryScores map[string][]models.SectionScore `json:"subcategory_scores,omitempty"`
	Survey            *models.Survey                   `json:"survey,omitempty"`
}

// AssessmentSummary contains summary information about an assessment
type AssessmentSummary struct {
	Assessment    models.Assessment     `json:"assessment"`
	SectionScores []models.SectionScore `json:"section_scores"`
	OverallScore  float64               `json:"overall_score"`
}

// calculateOverallScore calculates the overall percentage score
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
