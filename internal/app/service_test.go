package app

import (
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
