package domain

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
)

type Planner struct{}

func (Planner) Validate(req PlanRequest, c Candidate, profiles []Profile, recent []Candidate) []string {
	var violations []string
	requiredServings := req.Servings + req.LunchPortions
	if requiredServings < 1 {
		requiredServings = max(1, len(req.Diners))
	}
	if c.Servings < requiredServings {
		violations = append(violations, fmt.Sprintf("needs %d portions, makes %d", requiredServings, c.Servings))
	}
	if req.MaxMinutes > 0 && c.TotalMinutes > req.MaxMinutes {
		violations = append(violations, fmt.Sprintf("takes %d minutes, limit is %d", c.TotalMinutes, req.MaxMinutes))
	}
	if req.MaxCostPence > 0 && c.CostPence > req.MaxCostPence {
		violations = append(violations, fmt.Sprintf("costs £%.2f, budget is £%.2f", float64(c.CostPence)/100, float64(req.MaxCostPence)/100))
	}
	if req.RequireReheating && !c.ReheatsWell {
		violations = append(violations, "leftovers do not reheat well")
	}
	for _, needed := range c.Equipment {
		if !containsFold(req.Equipment, needed) {
			violations = append(violations, "missing equipment: "+needed)
		}
	}
	avoids := append([]string{}, req.Avoid...)
	for _, p := range profiles {
		if slices.Contains(req.Diners, p.ID) {
			avoids = append(avoids, p.Dislikes...)
		}
	}
	for _, ingredient := range c.Ingredients {
		if containsFold(avoids, ingredient.Name) {
			violations = append(violations, "contains avoided ingredient: "+ingredient.Name)
		}
	}
	sort.Strings(violations)
	return slices.Compact(violations)
}

func (Planner) Score(req PlanRequest, c Candidate, profiles []Profile, recent []Candidate) ScoreBreakdown {
	score := ScoreBreakdown{
		Preference:  20,
		Time:        20,
		Cost:        15,
		Leftovers:   10,
		Novelty:     10,
		Ingredients: 10,
	}
	var preference, count float64
	for _, p := range profiles {
		if !slices.Contains(req.Diners, p.ID) {
			continue
		}
		for _, tag := range c.Tags {
			preference += p.Preferences[strings.ToLower(tag)]
			count++
		}
		for _, liked := range p.Likes {
			if candidateContains(c, liked) {
				preference += 1.5
				count++
			}
		}
	}
	if count > 0 {
		score.Preference = clamp(20+preference/count*8, 0, 40)
	}
	if req.MaxMinutes > 0 {
		score.Time = clamp(20*(1-float64(c.TotalMinutes)/float64(req.MaxMinutes)*0.35), 0, 20)
	}
	if req.MaxCostPence > 0 {
		score.Cost = clamp(15*(1-float64(c.CostPence)/float64(req.MaxCostPence)*0.35), 0, 15)
	}
	if req.LunchPortions > 0 && !c.ReheatsWell {
		score.Leftovers = 0
	}
	for _, item := range req.Available {
		if candidateContains(c, item) {
			score.Ingredients += 1.5
		}
	}
	score.Ingredients = clamp(score.Ingredients, 0, 15)
	for i, meal := range recent {
		if strings.EqualFold(meal.Title, c.Title) {
			score.Novelty -= math.Max(2, 8-float64(i))
		}
		for _, tag := range c.Tags {
			if containsFold(meal.Tags, tag) {
				score.Novelty -= math.Max(.25, 1.5-float64(i)*.15)
			}
		}
	}
	score.Novelty = clamp(score.Novelty, 0, 10)
	score.Total = round(score.Preference + score.Time + score.Cost + score.Leftovers + score.Novelty + score.Ingredients)
	return score
}

func (p Planner) Rank(req PlanRequest, candidates []Candidate, profiles []Profile, recent []Candidate) []Decision {
	out := make([]Decision, 0, len(candidates))
	for _, candidate := range candidates {
		violations := p.Validate(req, candidate, profiles, recent)
		out = append(out, Decision{
			Request: req, Candidate: candidate, Score: p.Score(req, candidate, profiles, recent),
			Violations: violations, Status: "proposed",
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i].Violations) != len(out[j].Violations) {
			return len(out[i].Violations) < len(out[j].Violations)
		}
		if out[i].Score.Total != out[j].Score.Total {
			return out[i].Score.Total > out[j].Score.Total
		}
		return out[i].Candidate.Title < out[j].Candidate.Title
	})
	return out
}

func candidateContains(c Candidate, value string) bool {
	if containsFold(c.Tags, value) || strings.Contains(strings.ToLower(c.Title), strings.ToLower(value)) {
		return true
	}
	for _, item := range c.Ingredients {
		if strings.Contains(strings.ToLower(item.Name), strings.ToLower(value)) {
			return true
		}
	}
	return false
}

func containsFold(items []string, value string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}

func clamp(v, low, high float64) float64 { return math.Min(high, math.Max(low, v)) }
func round(v float64) float64            { return math.Round(v*10) / 10 }
