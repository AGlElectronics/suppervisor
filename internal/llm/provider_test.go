package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AGlElectronics/suppervisor/internal/domain"
)

type unavailableProvider struct{}

func (unavailableProvider) Complete(context.Context, string, string) (string, error) {
	return "", errors.New("offline")
}

type sequenceProvider struct {
	responses []string
	calls     int
}

func TestOllamaReadyRequiresConfiguredModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"models":[{"name":"other:latest"}]}`))
	}))
	defer server.Close()
	ollama := Ollama{BaseURL: server.URL, Model: "qwen"}
	if err := ollama.Ready(context.Background()); err == nil {
		t.Fatal("expected readiness to reject a missing model")
	}
	ollama.Model = "other"
	if err := ollama.Ready(context.Background()); err != nil {
		t.Fatalf("expected available model to be ready: %v", err)
	}
}

func (s *sequenceProvider) Complete(_ context.Context, _, _ string) (string, error) {
	value := s.responses[s.calls]
	s.calls++
	return value, nil
}

func TestGeneratorRetriesInvalidCandidateOnce(t *testing.T) {
	provider := &sequenceProvider{responses: []string{
		`{"candidates":[{"title":""}]}`,
		`{"candidates":[
			{"title":"Dinner one","summary":"Fast beans.","servings":2,"activeMinutes":10,"totalMinutes":20,"costPence":500,"ingredients":[{"name":"beans","quantity":1,"unit":"tin"}],"equipment":["hob"],"tags":["quick"],"reheatsWell":true,"steps":[{"title":"Cook","detail":"Cook it.","minutes":20}]},
			{"title":"Dinner two","summary":"Fast lentils.","servings":2,"activeMinutes":10,"totalMinutes":20,"costPence":500,"ingredients":[{"name":"lentils","quantity":1,"unit":"tin"}],"equipment":["hob"],"tags":["quick"],"reheatsWell":true,"steps":[{"title":"Cook","detail":"Cook it.","minutes":20}]},
			{"title":"Dinner three","summary":"Fast chickpeas.","servings":2,"activeMinutes":10,"totalMinutes":20,"costPence":500,"ingredients":[{"name":"chickpeas","quantity":1,"unit":"tin"}],"equipment":["hob"],"tags":["quick"],"reheatsWell":true,"steps":[{"title":"Cook","detail":"Cook it.","minutes":20}]}
		]}`,
	}}
	items, err := (Generator{Provider: provider}).Candidates(context.Background(), domain.PlanRequest{Text: "quick dinner"})
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 2 || len(items) != 3 || items[0].Title != "Dinner one" {
		t.Fatalf("retry did not return corrected candidate: calls=%d items=%#v", provider.calls, items)
	}
}

func TestGeneratorFallsBackDeterministicallyWhenProviderIsUnavailable(t *testing.T) {
	items, err := (Generator{Provider: unavailableProvider{}}).Candidates(
		context.Background(),
		domain.ParseRequest("Dinner under 30 minutes"),
	)
	if err != nil || len(items) != 3 {
		t.Fatalf("expected three local fallback candidates: %#v %v", items, err)
	}
	for _, item := range items {
		if item.Source != "deterministic-fallback" {
			t.Fatalf("fallback source was not exposed: %#v", item)
		}
	}
}
