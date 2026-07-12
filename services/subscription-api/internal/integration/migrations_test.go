//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
)

func TestMigrations_CreateUniqueTokenIndex(t *testing.T) {
	var indexDef string
	err := sharedPool.QueryRow(context.Background(), `
SELECT indexdef
FROM pg_indexes
WHERE schemaname = 'public' AND indexname = 'idx_subscriptions_token'
`).Scan(&indexDef)
	if err != nil {
		t.Fatalf("query token index: %v", err)
	}
	if !strings.Contains(indexDef, "UNIQUE INDEX") || !strings.Contains(indexDef, "WHERE (token IS NOT NULL)") {
		t.Fatalf("token index definition = %q", indexDef)
	}
}
