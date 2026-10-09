package app

import (
	"context"
	"errors"
	"fmt"
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
	history, err := s.Store.Decisions(ctx, 12)
	if err != nil {
		return nil, err
	}
	recent := make([]domain.Candidate, 0, len(history))
	for _, decision := range history {
		if decision.Status == "accepted" || decision.Status == "cooked" {
			recent = append(recent, decision.Candidate)
		}
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
	return s.Store.SetDecisionStatus(ctx, id, status, correction)
}

func (s Service) Rate(ctx context.Context, rating domain.Rating) ([]domain.PreferenceEvent, error) {
	if rating.Overall < 1 || rating.Overall > 5 || rating.Taste < 1 || rating.Taste > 5 ||
		rating.Ease < 1 || rating.Ease > 5 || rating.Reheat < 1 || rating.Reheat > 5 {
		return nil, errors.New("every rating dimension must be between 1 and 5")
	}
	decisions, err := s.Store.Decisions(ctx, 100)
	if err != nil {
		return nil, err
	}
	for _, decision := range decisions {
		if decision.ID == rating.DecisionID {
			return s.Store.SaveRating(ctx, rating, decision.Candidate)
		}
	}
	return nil, errors.New("decision not found")
}

func Shopping(candidate domain.Candidate) []domain.ShoppingItem {
	items := make([]domain.ShoppingItem, 0, len(candidate.Ingredients))
	for _, ingredient := range candidate.Ingredients {
		items = append(items, domain.ShoppingItem{Name: ingredient.Name, Quantity: ingredient.Quantity, Unit: ingredient.Unit})
	}
	return items
}
