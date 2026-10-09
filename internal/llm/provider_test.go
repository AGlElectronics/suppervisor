package llm

import (
	"context"
	"testing"

	"github.com/AGlElectronics/suppervisor/internal/domain"
)

type sequenceProvider struct {
	responses []string
	calls     int
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
