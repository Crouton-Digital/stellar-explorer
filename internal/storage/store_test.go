package storage_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/storage"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func bundle(t *testing.T, n uint32) domain.Bundle {
	t.Helper()
	b, e := decoder.Ledger(
		testutil.Remote(n, []xdr.TransactionEnvelope{testutil.PaymentEnvelope()}),
	)
	if e != nil {
		t.Fatal(e)
	}

	return b
}

func TestAtomicReplayAndRollback(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	b := bundle(t, 10)
	// An operation failure happens after ledger + transaction inserts and must roll back both.
	b.Transactions[0].Operations = append(
		b.Transactions[0].Operations,
		b.Transactions[0].Operations[0],
	)
	if e := s.Save(ctx, b); e == nil {
		t.Fatal("expected unique constraint violation")
	}

	if cp, e := s.Checkpoint(ctx); e != nil || cp != 0 {
		t.Fatalf("checkpoint advanced on failure: %d %v", cp, e)
	}

	if _, e := s.Get(ctx, "ledgers", "10"); !errors.Is(e, storage.ErrNotFound) {
		t.Fatalf("partial ledger visible: %v", e)
	}

	b = bundle(t, 10)
	if e := s.Save(ctx, b); e != nil {
		t.Fatal(e)
	}

	if e := s.Save(ctx, b); e != nil {
		t.Fatal(e)
	}

	var count int
	if e := s.Pool.QueryRow(ctx, `
		SELECT
		    count(*)
		FROM
		    transactions
	`).Scan(&count); e != nil || count != 1 {
		t.Fatalf("replay duplicated transactions: %d %v", count, e)
	}

	if cp, e := s.Checkpoint(ctx); e != nil || cp != 10 {
		t.Fatalf("incorrect checkpoint: %d %v", cp, e)
	}

	if state, e := s.State(ctx); e != nil || state.ProcessedTransactions != 1 {
		t.Fatalf("replay changed processed counter: %+v %v", state, e)
	}

	wrong := b
	wrong.Ledger.Hash = "other"
	if e := s.Save(ctx, wrong); !errors.Is(e, storage.ErrContinuity) {
		t.Fatalf("accepted reset ledger: %v", e)
	}

	if e := s.Save(ctx, domain.Bundle{
		Ledger: testutil.Ledger(12),
	}); !errors.Is(e, storage.ErrContinuity) {
		t.Fatalf("accepted ledger gap: %v", e)
	}

	if e := s.Save(ctx, domain.Bundle{
		Ledger: testutil.Ledger(11),
	}); e != nil {
		t.Fatal(e)
	}

	if cp, _ := s.Checkpoint(ctx); cp != 11 {
		t.Fatal("restart checkpoint not persisted")
	}
}

func TestPaginationSnapshotAndAccountDestination(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	if e := s.Save(ctx, bundle(t, 10)); e != nil {
		t.Fatal(e)
	}

	for n := uint32(11); n <= 12; n++ {
		if e := s.Save(ctx, domain.Bundle{
			Ledger: testutil.Ledger(n),
		}); e != nil {
			t.Fatal(e)
		}
	}

	p, e := s.List(ctx, "ledgers", "", 1, nil)
	if e != nil || len(p.Items) != 1 || p.NextCursor == nil {
		t.Fatalf("first page: %+v %v", p, e)
	}

	c, e := storage.ParseCursor(*p.NextCursor, "ledgers", "")
	if e != nil {
		t.Fatal(e)
	}

	foreign := *c
	foreign.Dataset = "another-database"
	if _, e = s.List(ctx, "ledgers", "", 1, &foreign); !errors.Is(e, storage.ErrInvalidCursor) {
		t.Fatalf("accepted cursor from another database: %v", e)
	}

	if e = s.Save(ctx, domain.Bundle{
		Ledger: testutil.Ledger(13),
	}); e != nil {
		t.Fatal(e)
	}

	p, e = s.List(ctx, "ledgers", "", 5, c)
	if e != nil || len(p.Items) != 2 {
		t.Fatalf("snapshot page: %+v %v", p, e)
	}

	var l domain.Ledger
	_ = json.Unmarshal(p.Items[0], &l)
	if l.Sequence != 11 {
		t.Fatalf("repeated or reordered ledger: %d", l.Sequence)
	}

	p, e = s.List(ctx, "account_transactions", testutil.Address(2), 20, nil)
	if e != nil || len(p.Items) != 1 {
		t.Fatalf("payment destination missing from account history: %+v %v", p, e)
	}

	for _, kind := range []string{
		"transactions",
		"ledger_transactions",
		"contract_transactions",
		"events",
	} {
		scope := ""
		if kind == "ledger_transactions" {
			scope = "10"
		}

		if kind == "contract_transactions" || kind == "events" {
			scope = testutil.Contract(1)
		}

		if _, e = s.List(ctx, kind, scope, 20, nil); e != nil {
			t.Errorf("%s: %v", kind, e)
		}
	}
}

func TestCursorRejectsCrossQueryReuse(t *testing.T) {
	c := storage.Cursor{
		Version:  1,
		Dataset:  "example-dataset",
		Kind:     "events",
		Scope:    "contract-a",
		Snapshot: 10,
		Ledger:   9,
		Index:    0,
		Hash:     "abc",
	}
	if _, e := storage.ParseCursor(c.Encode(), "events", "contract-a"); e != nil {
		t.Fatal(e)
	}

	for _, tc := range []struct {
		raw   string
		kind  string
		scope string
	}{
		{c.Encode(), "transactions", "contract-a"},
		{c.Encode(), "events", "contract-b"},
		{"garbage", "events", "contract-a"},
	} {
		if _, e := storage.ParseCursor(tc.raw, tc.kind, tc.scope); e == nil {
			t.Fatal("accepted invalid cursor")
		}
	}
}

func TestObservabilityMigrationPreservesData(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	if e := s.Save(ctx, bundle(t, 10)); e != nil {
		t.Fatal(e)
	}

	if _, e := s.Pool.Exec(ctx, `
		DELETE FROM schema_migrations
		WHERE
		    version = 2;
		
		ALTER TABLE indexer_status
		DROP COLUMN processed_transactions;
		
		ALTER TABLE indexer_status
		DROP COLUMN error_count
	`); e != nil {
		t.Fatal(e)
	}

	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}

	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}

	state, e := s.State(ctx)
	if e != nil || state.ProcessedTransactions != 1 || state.LastIndexedLedger == nil ||
		*state.LastIndexedLedger != 10 {
		t.Fatalf("migration lost state: %+v %v", state, e)
	}
}

func TestTransactionAndEventPaginationWithinLedger(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	b := testutil.RealBundle(t)
	if e := s.Save(ctx, b); e != nil {
		t.Fatal(e)
	}

	counts := map[string]int{}

	for _, tx := range b.Transactions {
		for _, ev := range tx.Events {
			counts[ev.ContractID]++
		}
	}

	for _, query := range []struct {
		kind     string
		scope    string
		expected int
	}{
		{"transactions", "", len(b.Transactions)},
		{"ledger_transactions", fmt.Sprint(b.Ledger.Sequence), len(b.Transactions)},
	} {
		assertPages(t, s, query.kind, query.scope, query.expected)
	}

	for contract, count := range counts {
		assertPages(t, s, "events", contract, count)
	}
}

func assertPages(t *testing.T, s *storage.Store, kind, scope string, expected int) {
	t.Helper()
	var cursor *storage.Cursor
	seen := map[string]bool{}

	for requests := 0; requests <= expected; requests++ {
		p, e := s.List(context.Background(), kind, scope, 2, cursor)
		if e != nil {
			t.Fatal(e)
		}

		for _, raw := range p.Items {
			var row struct {
				Hash            string `json:"hash"`
				TransactionHash string `json:"transaction_hash"`
				EventIndex      int    `json:"event_index"`
			}
			if e = json.Unmarshal(raw, &row); e != nil {
				t.Fatal(e)
			}

			key := row.Hash
			if kind == "events" {
				key = fmt.Sprintf("%s:%d", row.TransactionHash, row.EventIndex)
			}

			if seen[key] {
				t.Fatalf("%s pagination repeated %s", kind, key)
			}

			seen[key] = true
		}

		if p.NextCursor == nil {
			break
		}

		cursor, e = storage.ParseCursor(*p.NextCursor, kind, scope)
		if e != nil {
			t.Fatal(e)
		}
	}

	if len(seen) != expected {
		t.Fatalf("%s pagination lost rows: %d/%d", kind, len(seen), expected)
	}
}
