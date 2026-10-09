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
		`{"candidates":[{"title":"Dinner","servings":2,"totalMinutes":20,"costPence":500,"ingredients":[{"name":"beans","quantity":1,"unit":"tin"}],"steps":[{"title":"Cook","detail":"Cook it.","minutes":20}]}]}`,
	}}
	items, err := (Generator{Provider: provider}).Candidates(context.Background(), domain.PlanRequest{Text: "quick dinner"})
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 2 || len(items) != 1 || items[0].Title != "Dinner" {
		t.Fatalf("retry did not return corrected candidate: calls=%d items=%#v", provider.calls, items)
	}
}
