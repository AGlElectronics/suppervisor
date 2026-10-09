package domain

import (
	"regexp"
	"strconv"
	"strings"
)

var numberBefore = func(unit string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(\d+)\s*` + unit)
}

// ParseRequest converts common dinner shorthand into a conservative structured
// request. An LLM may enrich it later, but hard-rule inputs never depend on it.
func ParseRequest(text string) PlanRequest {
	req := PlanRequest{
		Text: text, Diners: []string{"alan", "partner"}, Servings: 2,
		MaxMinutes: 45, MaxCostPence: 1500, Equipment: []string{"hob", "oven"},
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "just me") || strings.Contains(lower, "only alan") {
		req.Diners, req.Servings = []string{"alan"}, 1
	}
	if m := numberBefore(`(?:people|diners|servings)`).FindStringSubmatch(lower); len(m) == 2 {
		req.Servings, _ = strconv.Atoi(m[1])
	}
	if m := numberBefore(`(?:min|mins|minutes)`).FindStringSubmatch(lower); len(m) == 2 {
		req.MaxMinutes, _ = strconv.Atoi(m[1])
	}
	money := regexp.MustCompile(`(?:£|under\s*£?)(\d+(?:\.\d{1,2})?)`).FindStringSubmatch(lower)
	if len(money) == 2 {
		if pounds, err := strconv.ParseFloat(money[1], 64); err == nil {
			req.MaxCostPence = int(pounds * 100)
		}
	}
	if m := numberBefore(`(?:lunch|lunches|lunch portions)`).FindStringSubmatch(lower); len(m) == 2 {
		req.LunchPortions, _ = strconv.Atoi(m[1])
		req.RequireReheating = true
	} else if strings.Contains(lower, "lunch") || strings.Contains(lower, "leftover") {
		req.LunchPortions, req.RequireReheating = 2, true
	}
	if strings.Contains(lower, "no oven") {
		req.Equipment = []string{"hob"}
	}
	for _, phrase := range []struct {
		marker string
		value  string
	}{{"no mushroom", "mushrooms"}, {"no fish", "fish"}, {"no aubergine", "aubergine"}, {"no coriander", "coriander"}} {
		if strings.Contains(lower, phrase.marker) {
			req.Avoid = append(req.Avoid, phrase.value)
		}
	}
	return req
}
