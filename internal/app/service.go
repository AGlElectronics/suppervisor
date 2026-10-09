package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/AGlElectronics/suppervisor/internal/domain"
	"github.com/AGlElectronics/suppervisor/internal/llm"
	"github.com/AGlElectronics/suppervisor/internal/store"
)

type Service struct {
	Store     *store.Postgres
	Generator llm.Generator
	Planner   domain.Planner
}

type PlanInput struct {
	Text       string              `json:"text"`
	Structured *domain.PlanRequest `json:"structured,omitempty"`
}

func (s Service) Plan(ctx context.Context, input PlanInput) ([]domain.Decision, error) {
	var req domain.PlanRequest
	if input.Structured != nil {
		req = *input.Structured
		if req.SchemaVersion == 0 {
			req.SchemaVersion = 1
		}
		if req.Text == "" {
			req.Text = input.Text
		}
	} else {
		req = domain.ParseRequest(input.Text)
	}
	if strings.TrimSpace(req.Text) == "" {
		return nil, errors.New("tell Suppervisor what dinner needs to do")
	}
	profiles, err := s.Store.Profiles(ctx)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateRequest(req, profiles); err != nil {
		return nil, err
	}
	recent, err := s.Store.RecentMeals(ctx, 20)
	if err != nil {
		return nil, err
	}
	candidates, err := s.Generator.Candidates(ctx, req)
	if err != nil {
		return nil, err
	}
	ranked := s.Planner.Rank(req, candidates, profiles, recent)
	for i := range ranked {
		saved, err := s.Store.SaveDecision(ctx, ranked[i])
		if err != nil {
			return nil, err
		}
		ranked[i] = saved
	}
	return ranked, nil
}

func (s Service) SetStatus(ctx context.Context, id, status, correction string) error {
	switch status {
	case "accepted", "rejected", "adjusted", "cooked":
	default:
		return fmt.Errorf("unsupported decision status %q", status)
	}
	decision, err := s.Store.DecisionByID(ctx, id)
	if err != nil {
		return err
	}
	if status == "accepted" && len(decision.Violations) > 0 {
		return fmt.Errorf("candidate is ineligible: %s", strings.Join(decision.Violations, "; "))
	}
	if status == "cooked" && decision.Status != "accepted" {
		return errors.New("only an accepted dinner can be marked cooked")
	}
	return s.Store.SetDecisionStatus(ctx, id, status, correction)
}

func (s Service) Rate(ctx context.Context, rating domain.Rating) ([]domain.PreferenceEvent, error) {
	if rating.Overall < 1 || rating.Overall > 5 || rating.Taste < 1 || rating.Taste > 5 ||
		rating.Ease < 1 || rating.Ease > 5 || rating.Reheat < 1 || rating.Reheat > 5 {
		return nil, errors.New("every rating dimension must be between 1 and 5")
	}
	if len(rating.Tags) > 8 {
		return nil, errors.New("a rating may include at most eight tags")
	}
	for _, tag := range rating.Tags {
		if len(strings.TrimSpace(tag)) == 0 || len(tag) > 40 {
			return nil, errors.New("rating tags must contain 1 to 40 characters")
		}
	}
	decision, err := s.Store.DecisionByID(ctx, rating.DecisionID)
	if err != nil {
		return nil, err
	}
	if decision.Status != "accepted" && decision.Status != "cooked" {
		return nil, errors.New("only accepted or cooked dinners can be rated")
	}
	if !slices.Contains(decision.Request.Diners, rating.ProfileID) {
		return nil, errors.New("rating profile was not a diner for this decision")
	}
	return s.Store.SaveRating(ctx, rating, decision.Candidate)
}

func Shopping(candidate domain.Candidate, available []string) []domain.ShoppingItem {
	have := map[string]bool{}
	for _, item := range available {
		have[strings.ToLower(strings.TrimSpace(item))] = true
	}
	items := make([]domain.ShoppingItem, 0, len(candidate.Ingredients))
	index := map[string]int{}
	for _, ingredient := range candidate.Ingredients {
		if have[strings.ToLower(strings.TrimSpace(ingredient.Name))] {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(ingredient.Name)) + "|" + strings.ToLower(strings.TrimSpace(ingredient.Unit))
		if position, ok := index[key]; ok {
			items[position].Quantity += ingredient.Quantity
			continue
		}
		index[key] = len(items)
		items = append(items, domain.ShoppingItem{Name: ingredient.Name, Quantity: ingredient.Quantity, Unit: ingredient.Unit})
	}
	return items
}
