package app

import (
	"context"
	"testing"

	"github.com/AGlElectronics/suppervisor/internal/domain"
)

func TestShoppingExcludesPantryAndAggregatesUnits(t *testing.T) {
	candidate := domain.Candidate{Ingredients: []domain.Ingredient{
		{Name: "Orzo", Quantity: 200, Unit: "g"},
		{Name: "orzo", Quantity: 100, Unit: "g"},
		{Name: "Tomatoes", Quantity: 2, Unit: "tin"},
	}}
	items := Shopping(candidate, []string{"tomatoes"})
	if len(items) != 1 || items[0].Name != "Orzo" || items[0].Quantity != 300 {
		t.Fatalf("unexpected shopping list: %#v", items)
	}
}

func TestRatingRequiresEveryStructuredDimension(t *testing.T) {
	_, err := (Service{}).Rate(context.Background(), domain.Rating{
		Overall: 5, Taste: 5, Ease: 5, Reheat: 0,
	})
	if err == nil {
		t.Fatal("expected missing reheat score to be rejected")
	}
}
