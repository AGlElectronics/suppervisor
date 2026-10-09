package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AGlElectronics/suppervisor/internal/domain"
)

type Provider interface {
	Complete(ctx context.Context, system, prompt string) (string, error)
}

type Ollama struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

func (o Ollama) Complete(ctx context.Context, system, prompt string) (string, error) {
	client := o.Client
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	payload, _ := json.Marshal(map[string]any{
		"model": o.Model, "stream": false, "format": "json",
		"messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": prompt}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(o.BaseURL, "/")+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode/100 != 2 {
		return "", fmt.Errorf("ollama returned %s: %s", res.Status, body)
	}
	var response struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	return response.Message.Content, nil
}

type Generator struct{ Provider Provider }

func (g Generator) Candidates(ctx context.Context, req domain.PlanRequest) ([]domain.Candidate, error) {
	if g.Provider == nil {
		return demoCandidates(req), nil
	}
	request, _ := json.Marshal(req)
	system := `You are Suppervisor's recipe generator. Return JSON only: {"candidates":[...]}. Each candidate must contain title, summary, servings, activeMinutes, totalMinutes, costPence, ingredients, equipment, tags, reheatsWell and ordered steps. Cost is a realistic whole number of UK pence. Do not decide whether constraints pass: generate concrete recipes for deterministic validation.`
	prompt := "Create exactly three varied dinner candidates for this structured request:\n" + string(request)
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt == 1 {
			prompt += "\nYour previous output failed validation: " + lastErr.Error() + ". Correct every missing or invalid field."
		}
		raw, err := g.Provider.Complete(ctx, system, prompt)
		if err != nil {
			return nil, err
		}
		var envelope struct {
			Candidates []domain.Candidate `json:"candidates"`
		}
		if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
			lastErr = fmt.Errorf("invalid JSON: %w", err)
			continue
		}
		var shape struct {
			Candidates []map[string]json.RawMessage `json:"candidates"`
		}
		if err := json.Unmarshal([]byte(raw), &shape); err != nil {
			lastErr = fmt.Errorf("invalid candidate shape: %w", err)
			continue
		}
		if err := validateCandidateShape(shape.Candidates); err != nil {
			lastErr = err
			continue
		}
		if err := validateCandidates(envelope.Candidates); err != nil {
			lastErr = err
			continue
		}
		return envelope.Candidates, nil
	}
	return nil, fmt.Errorf("candidate generation failed after one retry: %w", lastErr)
}

func validateCandidates(items []domain.Candidate) error {
	if len(items) != 3 {
		return fmt.Errorf("expected exactly three candidates, got %d", len(items))
	}
	for i, candidate := range items {
		if strings.TrimSpace(candidate.Title) == "" || strings.TrimSpace(candidate.Summary) == "" ||
			candidate.Servings < 1 || candidate.ActiveMinutes < 1 || candidate.TotalMinutes < candidate.ActiveMinutes ||
			candidate.CostPence < 1 || len(candidate.Ingredients) == 0 || len(candidate.Equipment) == 0 ||
			len(candidate.Tags) == 0 || len(candidate.Steps) == 0 {
			return fmt.Errorf("candidate %d lacks required recipe fields", i+1)
		}
		for _, ingredient := range candidate.Ingredients {
			if strings.TrimSpace(ingredient.Name) == "" || ingredient.Quantity <= 0 {
				return fmt.Errorf("candidate %d contains an invalid ingredient", i+1)
			}
		}
		for _, step := range candidate.Steps {
			if strings.TrimSpace(step.Title) == "" || strings.TrimSpace(step.Detail) == "" || step.Minutes < 1 {
				return fmt.Errorf("candidate %d contains an invalid cooking step", i+1)
			}
		}
	}
	return nil
}

func validateCandidateShape(items []map[string]json.RawMessage) error {
	if len(items) != 3 {
		return fmt.Errorf("expected exactly three candidates, got %d", len(items))
	}
	required := []string{"title", "summary", "servings", "activeMinutes", "totalMinutes", "costPence", "ingredients", "equipment", "tags", "reheatsWell", "steps"}
	for index, candidate := range items {
		for _, key := range required {
			if _, ok := candidate[key]; !ok {
				return fmt.Errorf("candidate %d is missing %s", index+1, key)
			}
		}
	}
	return nil
}

func demoCandidates(req domain.PlanRequest) []domain.Candidate {
	servings := max(req.Servings+req.LunchPortions, 2)
	return []domain.Candidate{
		{
			ID: "smoky-chickpea-traybake", Title: "Smoky chickpea & pepper traybake",
			Summary: "Jammy peppers, crisp chickpeas and lemony herb yoghurt.", Servings: servings,
			ActiveMinutes: 12, TotalMinutes: 32, CostPence: 780, ReheatsWell: true,
			Equipment: []string{"oven"}, Tags: []string{"mediterranean", "vegetarian", "traybake"},
			Ingredients: []domain.Ingredient{{Name: "chickpeas", Quantity: 2, Unit: "tins"}, {Name: "red peppers", Quantity: 3, Unit: ""}, {Name: "Greek yoghurt", Quantity: 150, Unit: "g"}, {Name: "smoked paprika", Quantity: 2, Unit: "tsp"}, {Name: "lemon", Quantity: 1, Unit: ""}},
			Steps:       []domain.Step{{Title: "Heat & chop", Detail: "Heat oven to 220°C. Slice peppers and drain chickpeas.", Minutes: 7}, {Title: "Roast", Detail: "Toss with paprika and oil; roast until crisp at the edges.", Minutes: 20}, {Title: "Finish", Detail: "Mix yoghurt with lemon and spoon over the tray.", Minutes: 5}},
		},
		{
			ID: "tomato-lentil-orzo", Title: "One-pan tomato lentil orzo",
			Summary: "Creamy without the fuss, bright with basil and lemon.", Servings: servings,
			ActiveMinutes: 10, TotalMinutes: 28, CostPence: 690, ReheatsWell: true,
			Equipment: []string{"hob"}, Tags: []string{"italian", "vegetarian", "one-pan"},
			Ingredients: []domain.Ingredient{{Name: "orzo", Quantity: 300, Unit: "g"}, {Name: "green lentils", Quantity: 1, Unit: "tin"}, {Name: "chopped tomatoes", Quantity: 1, Unit: "tin"}, {Name: "spinach", Quantity: 150, Unit: "g"}, {Name: "basil", Quantity: 1, Unit: "handful"}},
			Steps:       []domain.Step{{Title: "Build the base", Detail: "Soften garlic, then add tomatoes, lentils and orzo.", Minutes: 8}, {Title: "Simmer", Detail: "Add stock and simmer, stirring, until silky.", Minutes: 15}, {Title: "Green finish", Detail: "Fold through spinach and basil; season.", Minutes: 5}},
		},
		{
			ID: "ginger-chicken-rice", Title: "Sticky ginger chicken rice bowls",
			Summary: "Fast savoury-sweet chicken, crunchy cucumber and sesame.", Servings: servings,
			ActiveMinutes: 18, TotalMinutes: 35, CostPence: 1150, ReheatsWell: true,
			Equipment: []string{"hob"}, Tags: []string{"asian", "chicken", "rice"},
			Ingredients: []domain.Ingredient{{Name: "chicken thighs", Quantity: 600, Unit: "g"}, {Name: "rice", Quantity: 300, Unit: "g"}, {Name: "cucumber", Quantity: 1, Unit: ""}, {Name: "fresh ginger", Quantity: 25, Unit: "g"}, {Name: "soy sauce", Quantity: 3, Unit: "tbsp"}},
			Steps:       []domain.Step{{Title: "Cook rice", Detail: "Rinse and cook rice until fluffy.", Minutes: 18}, {Title: "Glaze chicken", Detail: "Brown chicken, then bubble with ginger and soy.", Minutes: 12}, {Title: "Build bowls", Detail: "Slice cucumber and assemble. Pack lunches separately.", Minutes: 5}},
		},
	}
}
