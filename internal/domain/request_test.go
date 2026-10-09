package domain

import "testing"

func TestParseRequest(t *testing.T) {
	got := ParseRequest("Just me, under £8, 25 minutes, 2 lunches, no oven and no mushrooms")
	if got.Servings != 1 || len(got.Diners) != 1 || got.MaxCostPence != 800 || got.MaxMinutes != 25 {
		t.Fatalf("unexpected structure: %#v", got)
	}
	if got.LunchPortions != 2 || !got.RequireReheating || len(got.Equipment) != 1 || len(got.Avoid) != 1 {
		t.Fatalf("failed to parse constraints: %#v", got)
	}
}
