package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/AGlElectronics/suppervisor/internal/domain"
)

func TestPostgresIntegrationLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.CheckSchema(ctx); err != nil {
		t.Fatal(err)
	}

	request := domain.ParseRequest("Dinner for Alan under 30 minutes")
	request.Diners = []string{"alan"}
	candidate := domain.Candidate{
		Title: "Integration beans", Summary: "A test dinner.", Servings: 2,
		ActiveMinutes: 10, TotalMinutes: 20, CostPence: 500, ReheatsWell: true,
		Ingredients: []domain.Ingredient{{Name: "beans", Quantity: 1, Unit: "tin"}},
		Equipment:   []string{"hob"}, Tags: []string{"beans"},
		Steps: []domain.Step{{Title: "Cook", Detail: "Cook the beans.", Minutes: 20}},
	}
	saved, err := db.SaveDecision(ctx, domain.Decision{
		Request: request, Candidate: candidate, Status: "proposed",
	})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := db.DecisionByID(ctx, saved.ID)
	if err != nil || decision.Candidate.Title != candidate.Title {
		t.Fatalf("decision round trip failed: %#v %v", decision, err)
	}
	if err := db.SetDecisionStatus(ctx, decision.ID, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	events, err := db.SaveRating(ctx, domain.Rating{
		DecisionID: decision.ID, ProfileID: "alan",
		Overall: 5, Taste: 5, Ease: 4, Reheat: 4, Tags: []string{"make again"},
	}, candidate)
	if err != nil || len(events) == 0 || events[0].RatingID == nil {
		t.Fatalf("rating learning was not persisted with provenance: %#v %v", events, err)
	}
	if err := db.RevertPreferenceEvent(ctx, events[0].ID); err != nil {
		t.Fatal(err)
	}

	updateID := time.Now().UnixNano()
	if claimed, err := db.ClaimTelegramUpdate(ctx, updateID); err != nil || !claimed {
		t.Fatalf("first Telegram claim failed: %v %v", claimed, err)
	}
	if err := db.FailTelegramUpdate(ctx, updateID, context.DeadlineExceeded); err != nil {
		t.Fatal(err)
	}
	if claimed, err := db.ClaimTelegramUpdate(ctx, updateID); err != nil || !claimed {
		t.Fatalf("failed Telegram update was not reclaimable: %v %v", claimed, err)
	}
	if err := db.CompleteTelegramUpdate(ctx, updateID); err != nil {
		t.Fatal(err)
	}
	if claimed, err := db.ClaimTelegramUpdate(ctx, updateID); err != nil || claimed {
		t.Fatalf("completed Telegram update was reclaimed: %v %v", claimed, err)
	}
}
