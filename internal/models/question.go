package models

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
)

// Survey represents the entire survey structure
type Survey struct {
	Sections []Section `json:"sections"`
}

// Section represents a section of the survey
type Section struct {
	SectionName       string     `json:"SectionName"`
	SpiderPos         int        `json:"SpiderPos,omitempty"`
	Questions         []Question `json:"Questions"`
	HasSubCategories  bool       `json:"HasSubCategories,omitempty"`
}

// Question represents a survey question
type Question struct {
	ID           string   `json:"ID,omitempty"`          // Generated ID like S1-Q1
	Type         string   `json:"Type"`                  // "Option", "Checkbox", "Banner"
	SubCategory  string   `json:"SubCategory,omitempty"`
	QuestionText string   `json:"QuestionText"`
	Answers      []Answer `json:"Answers,omitempty"`
}

// Answer represents a possible answer to a question
type Answer struct {
	ID     string  `json:"ID,omitempty"`     // Generated ID like S1-Q1-A1
	Answer string  `json:"Answer"`
	Score  float64 `json:"Score"`
	Value  string  `json:"Value,omitempty"`  // "checked" or empty
}

// QuestionService handles question-related operations
type QuestionService struct {
	questionsFile string
	adviceFile    string
}

// NewQuestionService creates a new question service
func NewQuestionService(questionsFile, adviceFile string) *QuestionService {
	return &QuestionService{
		questionsFile: questionsFile,
		adviceFile:    adviceFile,
	}
}

// LoadQuestions loads questions from the JSON file
func (s *QuestionService) LoadQuestions() (*Survey, error) {
	data, err := ioutil.ReadFile(s.questionsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read questions file: %w", err)
	}

	// Parse JSON into slice of sections
	var sections []Section
	if err := json.Unmarshal(data, &sections); err != nil {
		return nil, fmt.Errorf("failed to parse questions JSON: %w", err)
	}

	// Process sections and assign IDs
	survey := &Survey{Sections: sections}
	s.assignQuestionIDs(survey)
	s.detectSubCategories(survey)

	return survey, nil
}

// assignQuestionIDs assigns unique IDs to questions and answers
func (s *QuestionService) assignQuestionIDs(survey *Survey) {
	for sectionIndex, section := range survey.Sections {
		for questionIndex, question := range section.Questions {
			if question.Type != "Banner" {
				// Assign question ID
				question.ID = fmt.Sprintf("S%d-Q%d", sectionIndex+1, questionIndex+1)
				
				// Add default yes/no answers if not specified
				if len(question.Answers) == 0 {
					question.Answers = []Answer{
						{Answer: "Yes", Score: 1},
						{Answer: "No", Score: 0},
					}
				}
				
				// Assign answer IDs
				for answerIndex := range question.Answers {
					question.Answers[answerIndex].ID = fmt.Sprintf("S%d-Q%d-A%d", 
						sectionIndex+1, questionIndex+1, answerIndex+1)
					
					// Initialize value if not set
					if question.Answers[answerIndex].Value == "" {
						question.Answers[answerIndex].Value = ""
					}
				}
				
				// Update the question in the survey
				survey.Sections[sectionIndex].Questions[questionIndex] = question
			}
		}
	}
}

// detectSubCategories detects if sections have subcategories
func (s *QuestionService) detectSubCategories(survey *Survey) {
	for i, section := range survey.Sections {
		hasSubCategories := false
		for _, question := range section.Questions {
			if question.SubCategory != "" {
				hasSubCategories = true
				break
			}
		}
		survey.Sections[i].HasSubCategories = hasSubCategories
	}
}

// GetSectionByName returns a section by its name
func (s *QuestionService) GetSectionByName(survey *Survey, name string) (*Section, error) {
	for i := range survey.Sections {
		if survey.Sections[i].SectionName == name {
			return &survey.Sections[i], nil
		}
	}
	return nil, fmt.Errorf("section not found: %s", name)
}

// GetSectionByURLName returns a section by its URL-friendly name
func (s *QuestionService) GetSectionByURLName(survey *Survey, urlName string) (*Section, error) {
	for i := range survey.Sections {
		if SectionNameToURLName(survey.Sections[i].SectionName) == urlName {
			return &survey.Sections[i], nil
		}
	}
	return nil, fmt.Errorf("section not found: %s", urlName)
}

// GetQuestionByID returns a question by its ID
func (s *QuestionService) GetQuestionByID(survey *Survey, questionID string) (*Question, error) {
	for _, section := range survey.Sections {
		for i := range section.Questions {
			if section.Questions[i].ID == questionID {
				return &section.Questions[i], nil
			}
		}
	}
	return nil, fmt.Errorf("question not found: %s", questionID)
}

// CalculateQuestionScore calculates the score for a question based on selected answers
func (s *QuestionService) CalculateQuestionScore(question *Question) float64 {
	score := 0.0
	
	if question.Type == "Banner" {
		return 0
	}
	
	for _, answer := range question.Answers {
		if answer.Value == "checked" {
			score += answer.Score
		}
	}
	
	return score
}

// CalculateQuestionMaxScore calculates the maximum possible score for a question
func (s *QuestionService) CalculateQuestionMaxScore(question *Question) float64 {
	maxScore := 0.0
	
	if question.Type == "Banner" {
		return 0
	}
	
	switch question.Type {
	case "Option":
		// For radio buttons, find the highest score
		for _, answer := range question.Answers {
			if answer.Score > maxScore {
				maxScore = answer.Score
			}
		}
	case "Checkbox":
		// For checkboxes, sum all scores
		for _, answer := range question.Answers {
			maxScore += answer.Score
		}
	}
	
	return maxScore
}

// ApplyResponses applies saved responses to the survey structure
func (s *QuestionService) ApplyResponses(survey *Survey, responses []Response) error {
	// Create a map for quick lookup
	responseMap := make(map[string][]string)
	for _, response := range responses {
		responseMap[response.QuestionID] = response.AnswerIDs
	}
	
	// Apply responses to questions
	for sectionIndex := range survey.Sections {
		for questionIndex := range survey.Sections[sectionIndex].Questions {
			question := &survey.Sections[sectionIndex].Questions[questionIndex]
			
			if answerIDs, exists := responseMap[question.ID]; exists {
				// Reset all answers first
				for answerIndex := range question.Answers {
					question.Answers[answerIndex].Value = ""
				}
				
				// Mark selected answers
				for _, answerID := range answerIDs {
					for answerIndex := range question.Answers {
						if question.Answers[answerIndex].ID == answerID {
							question.Answers[answerIndex].Value = "checked"
						}
					}
				}
			}
		}
	}
	
	return nil
}

// ExtractResponses extracts the current responses from the survey
func (s *QuestionService) ExtractResponses(survey *Survey, assessmentID int) []Response {
	var responses []Response
	
	for _, section := range survey.Sections {
		for _, question := range section.Questions {
			if question.Type != "Banner" && question.ID != "" {
				var answerIDs []string
				
				for _, answer := range question.Answers {
					if answer.Value == "checked" {
						answerIDs = append(answerIDs, answer.ID)
					}
				}
				
				if len(answerIDs) > 0 {
					responses = append(responses, Response{
						AssessmentID: assessmentID,
						QuestionID:   question.ID,
						AnswerIDs:    answerIDs,
					})
				}
			}
		}
	}
	
	return responses
}

// CalculateSectionScores calculates scores for all sections
func (s *QuestionService) CalculateSectionScores(survey *Survey, assessmentID int) []SectionScore {
	var scores []SectionScore
	
	for _, section := range survey.Sections {
		score := 0.0
		maxScore := 0.0
		
		// Calculate scores for all questions in the section
		for _, question := range section.Questions {
			score += s.CalculateQuestionScore(&question)
			maxScore += s.CalculateQuestionMaxScore(&question)
		}
		
		// Only include sections that have scoreable questions
		if maxScore > 0 {
			percentage := (score / maxScore) * 100
			
			scores = append(scores, SectionScore{
				AssessmentID: assessmentID,
				SectionName:  section.SectionName,
				Score:        score,
				MaxScore:     maxScore,
				Percentage:   percentage,
			})
		}
	}
	
	return scores
}

// CalculateSubCategoryScores calculates scores for subcategories within a section
func (s *QuestionService) CalculateSubCategoryScores(survey *Survey, sectionName string, assessmentID int) []SectionScore {
	var scores []SectionScore
	subCategoryScores := make(map[string]*SectionScore)
	
	// Find the section
	var targetSection *Section
	for i := range survey.Sections {
		if survey.Sections[i].SectionName == sectionName {
			targetSection = &survey.Sections[i]
			break
		}
	}
	
	if targetSection == nil {
		return scores
	}
	
	// Calculate scores by subcategory
	for _, question := range targetSection.Questions {
		if question.SubCategory != "" {
			if _, exists := subCategoryScores[question.SubCategory]; !exists {
				subCategoryScores[question.SubCategory] = &SectionScore{
					AssessmentID: assessmentID,
					SectionName:  question.SubCategory,
					Score:        0,
					MaxScore:     0,
				}
			}
			
			subCategoryScores[question.SubCategory].Score += s.CalculateQuestionScore(&question)
			subCategoryScores[question.SubCategory].MaxScore += s.CalculateQuestionMaxScore(&question)
		}
	}
	
	// Calculate percentages and build result slice
	for _, score := range subCategoryScores {
		if score.MaxScore > 0 {
			score.Percentage = (score.Score / score.MaxScore) * 100
			scores = append(scores, *score)
		}
	}
	
	return scores
}

// Advice represents improvement advice for a section
type Advice struct {
	SectionName string             `json:"section_name"`
	Advice      string             `json:"advice"`
	ReadMore    string             `json:"read_more,omitempty"`
	Links       []AdviceLink       `json:"links"`
}

// AdviceLink represents a resource link
type AdviceLink struct {
	Type string `json:"Type"`
	Text string `json:"Text"`
	Href string `json:"Href"`
	Paid string `json:"Paid,omitempty"`
}

// LoadAdvice loads advice from the JSON file
func (s *QuestionService) LoadAdvice() (map[string]Advice, error) {
	data, err := ioutil.ReadFile(s.adviceFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read advice file: %w", err)
	}

	var rawAdvice map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawAdvice); err != nil {
		return nil, fmt.Errorf("failed to parse advice JSON: %w", err)
	}

	advice := make(map[string]Advice)
	for key, value := range rawAdvice {
		if key == "//" {
			continue // Skip comments
		}
		
		var sectionAdvice struct {
			Advice   string       `json:"Advice"`
			ReadMore string       `json:"ReadMore,omitempty"`
			Links    []AdviceLink `json:"Links"`
		}
		
		if err := json.Unmarshal(value, &sectionAdvice); err != nil {
			return nil, fmt.Errorf("failed to parse advice for section %s: %w", key, err)
		}
		
		advice[key] = Advice{
			SectionName: key,
			Advice:      sectionAdvice.Advice,
			ReadMore:    sectionAdvice.ReadMore,
			Links:       sectionAdvice.Links,
		}
	}
	
	return advice, nil
}

// Helper function to convert section names to URL-friendly format
func SectionNameToURLName(sectionName string) string {
	// Remove commas and replace spaces with hyphens
	urlName := strings.ReplaceAll(sectionName, ",", "")
	urlName = strings.ReplaceAll(urlName, " ", "-")
	return strings.ToLower(urlName)
}