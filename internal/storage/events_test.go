package storage_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/storage"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

func TestTransactionEventsPersistenceReplayAndFilteredPagination(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	bundle := testutil.FeeBundle(t, 10, false)
	for replay := 0; replay < 2; replay++ {
		if err := s.Save(ctx, bundle); err != nil {
			t.Fatal(err)
		}
	}

	var fees, movements int
	if err := s.Pool.QueryRow(ctx, `
		SELECT
		    (SELECT count(*) FROM fee_events),
		    (SELECT count(*) FROM token_transfers)
	`).Scan(&fees, &movements); err != nil || fees != 2 || movements != 0 {
		t.Fatal("failed transaction fees duplicated or became movements", fees, movements, err)
	}

	filters := domain.Filters{EventScope: "transaction", EventName: "fee"}
	page, err := s.List(ctx, "events", "", 1, nil, filters)
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil {
		t.Fatal("missing transaction event page", err)
	}

	cursor, err := storage.ParseCursor(*page.NextCursor, "events", "", filters)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = storage.ParseCursor(*page.NextCursor, "events", "",
		domain.Filters{EventScope: "operation", EventName: "fee"}); err == nil {
		t.Fatal("transaction event cursor accepted for operation events")
	}

	next, err := s.List(ctx, "events", "", 1, cursor, filters)
	if err != nil || len(next.Items) != 1 || next.NextCursor != nil ||
		string(page.Items[0]) == string(next.Items[0]) {
		t.Fatal("fee event pagination duplicated or omitted a record", err)
	}

	page, err = s.List(ctx, "events", "", 20, nil, domain.Filters{EventScope: "operation"})
	if err != nil || len(page.Items) != 0 {
		t.Fatal("failed operation was exposed in activity", err)
	}
}

func TestFeeEventWriteFailureRollsBackWholeLedger(t *testing.T) {
	s := testutil.Database(t)
	bundle := testutil.FeeBundle(t, 10, true)
	bundle.Transactions[0].Fees.Events[1].EventIndex = 99
	if err := s.Save(context.Background(), bundle); err == nil {
		t.Fatal("fee without parent event accepted")
	}

	var ledgers, events, fees int
	if err := s.Pool.QueryRow(context.Background(), `
		SELECT
		    (SELECT count(*) FROM ledgers),
		    (SELECT count(*) FROM contract_events),
		    (SELECT count(*) FROM fee_events)
	`).Scan(&ledgers, &events, &fees); err != nil || ledgers != 0 || events != 0 || fees != 0 {
		t.Fatal("partial ledger committed after failed fee write", err)
	}

	if checkpoint, err := s.Checkpoint(context.Background()); err != nil || checkpoint != 0 {
		t.Fatal("checkpoint advanced after failed fee write", err)
	}
}

func TestEventMigrationPreservesSourcesStateAndMovementCursors(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	for sequence := uint32(10); sequence <= 11; sequence++ {
		if err := s.Save(ctx, testutil.FeeBundle(t, sequence, sequence == 10)); err != nil {
			t.Fatal(err)
		}
	}

	before := eventSourceSnapshot(t, s)
	state, err := s.State(ctx)
	if err != nil {
		t.Fatal(err)
	}

	page, err := s.List(ctx, "events", "", 1, nil, domain.Filters{EventScope: "operation"})
	if err != nil || len(page.Items) != 1 {
		t.Fatal(err)
	}

	removeEventMigration(t, s)
	for repetition := 0; repetition < 2; repetition++ {
		if err = s.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}

	afterState, err := s.State(ctx)
	if err != nil || !reflect.DeepEqual(state, afterState) || before != eventSourceSnapshot(t, s) {
		t.Fatal("event migration changed source data, movements, dataset, or ingestion state", err)
	}

	after, err := s.List(ctx, "events", "", 1, nil, domain.Filters{EventScope: "operation"})
	if err != nil || !reflect.DeepEqual(page, after) {
		t.Fatal("event migration changed operation event identities", err)
	}

	var fees int
	if err = s.Pool.QueryRow(ctx, "SELECT count(*) FROM fee_events").Scan(&fees); err != nil ||
		fees != 4 {
		t.Fatal("event migration failed to rebuild successful/failed fees", fees, err)
	}

	raw, err := s.Get(ctx, "transactions", testutil.FeeBundle(t, 11, false).Transactions[0].Hash)
	var transaction domain.Transaction
	if err != nil || json.Unmarshal(raw, &transaction) != nil ||
		transaction.Fees.Status != "complete" ||
		len(transaction.Events) != 2 ||
		len(transaction.TokenTransfers) != 0 {
		t.Fatal("migrated failed transaction has incorrect activity", err)
	}
}

func TestEventMigrationRollsBackUnreadableOrConflictingSource(t *testing.T) {
	for _, corrupt := range []string{
		`UPDATE transactions SET payload = jsonb_set(payload, '{meta_xdr}', '"invalid"'::jsonb)`,
		`UPDATE contract_events SET raw_xdr = 'different source'`,
	} {
		t.Run(corrupt, func(t *testing.T) {
			s := testutil.Database(t)
			ctx := context.Background()
			if err := s.Save(ctx, testutil.FeeBundle(t, 10, true)); err != nil {
				t.Fatal(err)
			}

			removeEventMigration(t, s)
			if _, err := s.Pool.Exec(ctx, corrupt); err != nil {
				t.Fatal(err)
			}

			if err := s.Migrate(ctx); err == nil {
				t.Fatal("event migration accepted invalid retained source")
			}

			var applied bool
			var table *string
			if err := s.Pool.QueryRow(ctx, `
				SELECT
				    EXISTS (SELECT 1 FROM schema_migrations WHERE version = 6),
				    to_regclass(current_schema() || '.fee_events')::text
			`).Scan(&applied, &table); err != nil || applied || table != nil {
				t.Fatal("failed event migration committed schema or completion marker", err)
			}
		})
	}
}

func removeEventMigration(t *testing.T, s *storage.Store) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), `
		DROP TABLE fee_events;
		DELETE FROM contract_events WHERE event_scope = 'transaction';
		ALTER TABLE contract_events DROP COLUMN event_scope;
		DELETE FROM schema_migrations WHERE version = 6;
		UPDATE transactions t
		SET
		    payload = (payload - 'fees') || jsonb_build_object(
		        'events', COALESCE((
		            SELECT jsonb_agg(e.payload - 'event_scope' - 'stage' ORDER BY e.event_index)
		            FROM contract_events e
		            WHERE e.network = t.network AND e.transaction_hash = t.hash
		        ), '[]'::jsonb)
		    );
		UPDATE contract_events SET payload = payload - 'event_scope' - 'stage'
	`); err != nil {
		t.Fatal(err)
	}
}

func eventSourceSnapshot(t *testing.T, s *storage.Store) string {
	t.Helper()
	var snapshot string
	if err := s.Pool.QueryRow(context.Background(), `
		SELECT
		    md5(string_agg(
		        t.envelope_xdr || t.result_xdr || t.meta_xdr ||
		        (t.payload - 'events' - 'fees')::text || l.raw_ref || m.dataset_id,
		        '' ORDER BY t.ledger_sequence, t.tx_index
		    )) || (SELECT md5(string_agg(payload::text, '' ORDER BY transaction_hash, event_index))
		           FROM token_transfers)
		FROM
		    transactions t
		    JOIN ledgers l ON l.network = t.network AND l.sequence = t.ledger_sequence
		    JOIN indexer_metadata m ON m.network = t.network
	`).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}

	return snapshot
}
