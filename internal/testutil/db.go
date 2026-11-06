package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/crouton/stellar-explorer/internal/storage"
)

// Database gives each integration test an isolated schema, never truncating shared tables.
func Database(t *testing.T) *storage.Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("integration test: set TEST_DATABASE_URL to a disposable PostgreSQL database")
	}

	ctx := context.Background()
	admin, e := storage.Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}

	raw := make([]byte, 8)
	_, _ = rand.Read(raw)
	schema := "test_" + hex.EncodeToString(raw)
	if _, e = admin.Pool.Exec(ctx, `
		CREATE SCHEMA
	`+schema); e != nil {
		t.Fatal(e)
	}

	u, e := url.Parse(dsn)
	if e != nil {
		t.Fatal(e)
	}

	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, e := storage.Open(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}

	t.Cleanup(func() {
		s.Close()
		_, _ = admin.Pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}

	return s
}
