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
