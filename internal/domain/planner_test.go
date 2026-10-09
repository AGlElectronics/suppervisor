package domain

import (
	"reflect"
	"testing"
)

func TestRankHardConstraintsBeforeScore(t *testing.T) {
	req := PlanRequest{
		Diners: []string{"alan", "partner"}, Servings: 2, LunchPortions: 2,
		MaxMinutes: 35, MaxCostPence: 1200, Equipment: []string{"hob", "oven"}, RequireReheating: true,
	}
	profiles := []Profile{{ID: "partner", Dislikes: []string{"mushrooms"}}}
	valid := Candidate{Title: "Smoky bean traybake", Servings: 4, TotalMinutes: 30, CostPence: 850, ReheatsWell: true, Equipment: []string{"oven"}}
	invalid := Candidate{Title: "Mushroom risotto", Servings: 2, TotalMinutes: 50, CostPence: 1500, ReheatsWell: false, Equipment: []string{"hob"}, Ingredients: []Ingredient{{Name: "mushrooms"}}}

	ranked := (Planner{}).Rank(req, []Candidate{invalid, valid}, profiles, nil)
	if ranked[0].Candidate.Title != valid.Title {
		t.Fatalf("expected valid candidate first, got %q", ranked[0].Candidate.Title)
	}
	if got := len(ranked[1].Violations); got != 5 {
		t.Fatalf("expected five hard violations, got %d: %v", got, ranked[1].Violations)
	}
}

func TestScoreIsDeterministicAndPenalizesRecentMeal(t *testing.T) {
	req := PlanRequest{Diners: []string{"alan"}, MaxMinutes: 45, MaxCostPence: 1500}
	profiles := []Profile{{ID: "alan", Likes: []string{"beans"}, Preferences: map[string]float64{"mexican": 1}}}
	candidate := Candidate{Title: "Black bean tacos", TotalMinutes: 25, CostPence: 900, Tags: []string{"mexican"}, Ingredients: []Ingredient{{Name: "black beans"}}}
	planner := Planner{}

	first := planner.Score(req, candidate, profiles, nil)
	second := planner.Score(req, candidate, profiles, []Candidate{candidate})
	if first.Total <= second.Total {
		t.Fatalf("expected recent meal penalty: first %.1f, second %.1f", first.Total, second.Total)
	}
	if again := planner.Score(req, candidate, profiles, nil); !reflect.DeepEqual(again, first) {
		t.Fatalf("score changed between identical runs: %#v != %#v", again, first)
	}
}

func TestScoreExposesPerDinerFitAndPenalizesImbalance(t *testing.T) {
	req := ParseRequest("Dinner for both")
	profiles := []Profile{
		{ID: "alan", Preferences: map[string]float64{"spicy": 2}},
		{ID: "partner", Preferences: map[string]float64{"spicy": -1}},
	}
	candidate := Candidate{Title: "Chilli", Tags: []string{"spicy"}, Servings: 4, TotalMinutes: 20, CostPence: 500, ReheatsWell: true}
	score := (Planner{}).Score(req, candidate, profiles, nil)
	if len(score.ByDiner) != 2 || score.ByDiner["alan"] <= score.ByDiner["partner"] {
		t.Fatalf("expected transparent opposing diner scores, got %#v", score.ByDiner)
	}
	average := (score.ByDiner["alan"] + score.ByDiner["partner"]) / 2
	if score.Preference >= average || score.Fairness != score.ByDiner["partner"] {
		t.Fatalf("expected fairness penalty and minimum diner score, got %#v", score)
	}
}
