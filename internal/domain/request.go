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
		SchemaVersion: 1, Text: text, Diners: []string{"alan", "partner"}, Servings: 2,
		MaxMinutes: 45, MaxCostPence: 1500, Equipment: []string{"hob", "oven"},
		Assumptions: []string{"both diners", "standard hob and oven available", "£15 maximum", "45 minute maximum"},
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "just me") || strings.Contains(lower, "only alan") {
		req.Diners, req.Servings = []string{"alan"}, 1
		req.Assumptions = removeAssumption(req.Assumptions, "both diners")
	}
	if strings.Contains(lower, "just partner") || strings.Contains(lower, "partner only") || strings.Contains(lower, "only partner") {
		req.Diners, req.Servings = []string{"partner"}, 1
		req.Assumptions = removeAssumption(req.Assumptions, "both diners")
	}
	if m := numberBefore(`(?:people|diners|servings)`).FindStringSubmatch(lower); len(m) == 2 {
		req.Servings, _ = strconv.Atoi(m[1])
	}
	if m := numberBefore(`(?:min|mins|minutes)`).FindStringSubmatch(lower); len(m) == 2 {
		req.MaxMinutes, _ = strconv.Atoi(m[1])
		req.Assumptions = removeAssumption(req.Assumptions, "45 minute maximum")
	}
	money := regexp.MustCompile(`(?:£|under\s*£?)(\d+(?:\.\d{1,2})?)`).FindStringSubmatch(lower)
	if len(money) == 2 {
		if pounds, err := strconv.ParseFloat(money[1], 64); err == nil {
			req.MaxCostPence = int(pounds * 100)
			req.Assumptions = removeAssumption(req.Assumptions, "£15 maximum")
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
		req.Assumptions = removeAssumption(req.Assumptions, "standard hob and oven available")
	}
	if strings.Contains(lower, "no hob") {
		req.Equipment = []string{"oven"}
		req.Assumptions = removeAssumption(req.Assumptions, "standard hob and oven available")
	}
	for _, equipment := range []string{"microwave", "air fryer", "hob", "oven"} {
		if strings.Contains(lower, "only "+equipment) || strings.Contains(lower, "just "+equipment) {
			req.Equipment = []string{equipment}
			req.Assumptions = removeAssumption(req.Assumptions, "standard hob and oven available")
		}
	}
	avoidPattern := regexp.MustCompile(`(?:no|without|avoid)\s+([a-z][a-z-]*)`)
	for _, match := range avoidPattern.FindAllStringSubmatch(lower, -1) {
		value := strings.TrimSpace(match[1])
		if value != "" && value != "oven" && value != "hob" && !containsFold(req.Avoid, value) {
			req.Avoid = append(req.Avoid, value)
		}
	}
	availablePattern := regexp.MustCompile(`(?:have|using|use up)\s+([a-z][a-z ,&-]{1,80})`)
	if match := availablePattern.FindStringSubmatch(lower); len(match) == 2 {
		value := match[1]
		for _, stop := range []string{" tonight", " under", " for ", " but ", " with no", ", no ", ", only ", "."} {
			value = strings.SplitN(value, stop, 2)[0]
		}
		for _, item := range regexp.MustCompile(`\s*(?:,|&|\band\b)\s*`).Split(value, -1) {
			if item = strings.TrimSpace(item); item != "" && len(item) <= 40 {
				req.Available = append(req.Available, item)
			}
		}
	}
	return req
}

func removeAssumption(items []string, value string) []string {
	out := items[:0]
	for _, item := range items {
		if item != value {
			out = append(out, item)
		}
	}
	return out
}
