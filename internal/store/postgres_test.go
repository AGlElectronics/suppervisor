package store

import (
	"io/fs"
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
