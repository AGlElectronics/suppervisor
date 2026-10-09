package domain

import "time"

type Profile struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Dislikes    []string           `json:"dislikes"`
	Likes       []string           `json:"likes"`
	Preferences map[string]float64 `json:"preferences"`
}

type PlanRequest struct {
	SchemaVersion    int      `json:"schemaVersion"`
	Text             string   `json:"text"`
	Diners           []string `json:"diners"`
	Servings         int      `json:"servings"`
	LunchPortions    int      `json:"lunchPortions"`
	MaxMinutes       int      `json:"maxMinutes"`
	MaxCostPence     int      `json:"maxCostPence"`
	Available        []string `json:"available"`
	Avoid            []string `json:"avoid"`
	Equipment        []string `json:"equipment"`
	RequireReheating bool     `json:"requireReheating"`
	Assumptions      []string `json:"assumptions,omitempty"`
}

type Ingredient struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
	Optional bool    `json:"optional,omitempty"`
}

type Step struct {
	Title   string `json:"title"`
	Detail  string `json:"detail"`
	Minutes int    `json:"minutes"`
}

type Candidate struct {
	ID            string       `json:"id"`
	Title         string       `json:"title"`
	Summary       string       `json:"summary"`
	Servings      int          `json:"servings"`
	ActiveMinutes int          `json:"activeMinutes"`
	TotalMinutes  int          `json:"totalMinutes"`
	CostPence     int          `json:"costPence"`
	Ingredients   []Ingredient `json:"ingredients"`
	Equipment     []string     `json:"equipment"`
	Tags          []string     `json:"tags"`
	ReheatsWell   bool         `json:"reheatsWell"`
	Steps         []Step       `json:"steps"`
}

type ScoreBreakdown struct {
	Preference  float64            `json:"preference"`
	Fairness    float64            `json:"fairness"`
	ByDiner     map[string]float64 `json:"byDiner"`
	Time        float64            `json:"time"`
	Cost        float64            `json:"cost"`
	Leftovers   float64            `json:"leftovers"`
	Novelty     float64            `json:"novelty"`
	Ingredients float64            `json:"ingredients"`
	Total       float64            `json:"total"`
}

type Decision struct {
	ID         string         `json:"id"`
	Request    PlanRequest    `json:"request"`
	Candidate  Candidate      `json:"candidate"`
	Score      ScoreBreakdown `json:"score"`
	Violations []string       `json:"violations"`
	Status     string         `json:"status"`
	Correction string         `json:"correction,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
}

type Rating struct {
	DecisionID string   `json:"decisionId"`
	ProfileID  string   `json:"profileId"`
	Overall    int      `json:"overall"`
	Taste      int      `json:"taste"`
	Ease       int      `json:"ease"`
	Reheat     int      `json:"reheat"`
	Tags       []string `json:"tags"`
	Notes      string   `json:"notes"`
}

type PreferenceEvent struct {
	ID         string     `json:"id"`
	ProfileID  string     `json:"profileId"`
	Key        string     `json:"key"`
	Delta      float64    `json:"delta"`
	Reason     string     `json:"reason"`
	ReversalOf *string    `json:"reversalOf,omitempty"`
	RevertedAt *time.Time `json:"revertedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type DecisionEvent struct {
	ID         string         `json:"id"`
	DecisionID string         `json:"decisionId"`
	Type       string         `json:"type"`
	Actor      string         `json:"actor"`
	Detail     map[string]any `json:"detail"`
	CreatedAt  time.Time      `json:"createdAt"`
}

type ShoppingItem struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
	Checked  bool    `json:"checked"`
}

type WorkflowState struct {
	DecisionID      string    `json:"decisionId"`
	ShoppingChecked []string  `json:"shoppingChecked"`
	CookingStep     int       `json:"cookingStep"`
	UpdatedAt       time.Time `json:"updatedAt"`
}
