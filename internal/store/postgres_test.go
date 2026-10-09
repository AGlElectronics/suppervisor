package store

import (
	"io/fs"
	"strings"
	"testing"
)

func TestEveryMigrationIsReversible(t *testing.T) {
	paths, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("expected committed migrations")
	}
	for _, path := range paths {
		raw, err := migrationFS.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		up, down, err := SplitMigration(string(raw))
		if err != nil || up == "" || down == "" {
			t.Fatalf("%s is not reversible: %v", path, err)
		}
	}
}

func TestInitialSchemaSupportsImmutableAuditEvents(t *testing.T) {
	raw, err := migrationFS.ReadFile("migrations/00001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(raw)
	for _, required := range []string{"CREATE TABLE decision_events", "reversal_of uuid UNIQUE", "CREATE TABLE telegram_updates", "CREATE TABLE workflow_states"} {
		if !strings.Contains(schema, required) {
			t.Fatalf("initial schema missing %q", required)
		}
	}
	if strings.Contains(schema, "UPDATE preference_events") {
		t.Fatal("preference history must be reversed with compensating rows, not mutation")
	}
}
