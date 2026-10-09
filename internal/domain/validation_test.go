package domain

import "testing"

func TestValidateRequestRejectsUnknownDinerAndInvalidBounds(t *testing.T) {
	profiles := []Profile{{ID: "alan"}, {ID: "partner"}}
	request := ParseRequest("Dinner")
	request.Diners = []string{"stranger"}
	if err := ValidateRequest(request, profiles); err == nil {
		t.Fatal("expected unknown diner rejection")
	}
	request.Diners = []string{"alan"}
	request.MaxMinutes = -1
	if err := ValidateRequest(request, profiles); err == nil {
		t.Fatal("expected invalid time rejection")
	}
}

func TestParseRequestHandlesPartnerEquipmentAvoidanceAndAvailableFood(t *testing.T) {
	request := ParseRequest("Partner only, use up chickpeas and spinach, no peanuts, only air fryer, under £9 and 25 minutes")
	if len(request.Diners) != 1 || request.Diners[0] != "partner" || request.Servings != 1 {
		t.Fatalf("partner-only request was not structured: %#v", request)
	}
	if len(request.Equipment) != 1 || request.Equipment[0] != "air fryer" {
		t.Fatalf("equipment was not structured: %#v", request.Equipment)
	}
	if !containsFold(request.Avoid, "peanuts") || !containsFold(request.Available, "chickpeas") || !containsFold(request.Available, "spinach") {
		t.Fatalf("ingredients were not structured: avoid=%#v available=%#v", request.Avoid, request.Available)
	}
	if request.MaxCostPence != 900 || request.MaxMinutes != 25 {
		t.Fatalf("limits were not structured: %#v", request)
	}
}
