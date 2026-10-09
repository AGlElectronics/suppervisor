package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

func ValidateRequest(req PlanRequest, profiles []Profile) error {
	if req.SchemaVersion != 1 {
		return fmt.Errorf("unsupported request schema version %d", req.SchemaVersion)
	}
	if strings.TrimSpace(req.Text) == "" {
		return errors.New("request text is required")
	}
	if len(req.Diners) == 0 {
		return errors.New("at least one diner is required")
	}
	known := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		known = append(known, profile.ID)
	}
	for _, diner := range req.Diners {
		if !slices.Contains(known, diner) {
			return fmt.Errorf("unknown diner %q", diner)
		}
	}
	if req.Servings < 1 || req.Servings > 20 {
		return errors.New("servings must be between 1 and 20")
	}
	if req.LunchPortions < 0 || req.LunchPortions > 20 {
		return errors.New("lunch portions must be between 0 and 20")
	}
	if req.MaxMinutes < 1 || req.MaxMinutes > 240 {
		return errors.New("maximum time must be between 1 and 240 minutes")
	}
	if req.MaxCostPence < 1 || req.MaxCostPence > 100_000 {
		return errors.New("maximum cost must be between £0.01 and £1000")
	}
	if len(req.Equipment) == 0 {
		return errors.New("available equipment is required")
	}
	return nil
}
