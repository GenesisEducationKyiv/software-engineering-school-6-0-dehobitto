package migrations

import (
	"embed"
	"testing"
)

//go:embed testdata/*.sql
var testMigrations embed.FS

func TestLoadOrdersVersionedSQLFiles(t *testing.T) {
	list, err := Load(testMigrations, "testdata")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("migrations = %d, want 2", len(list))
	}
	if list[0].Version != 1 || list[0].Name != "create_widgets" {
		t.Fatalf("first migration = %#v", list[0])
	}
	if list[1].Version != 2 || list[1].Name != "add_widget_name" {
		t.Fatalf("second migration = %#v", list[1])
	}
	if list[0].Checksum == "" {
		t.Fatal("checksum is empty")
	}
}
