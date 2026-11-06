package storage_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/storage"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

func TestInterpretationMigrationPreservesSourceAndExistingDerivedData(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	for sequence := uint32(10); sequence <= 11; sequence++ {
		if err := s.Save(ctx, testutil.InterpretationBundle(t, sequence, sequence == 10)); err != nil {
			t.Fatal(err)
		}
	}

	beforeState, err := s.State(ctx)
	if err != nil {
		t.Fatal(err)
	}

	before := interpretationSnapshot(t, s)
	removeInterpretationMigration(t, s)
	for replay := 0; replay < 2; replay++ {
		if err = s.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}

	afterState, err := s.State(ctx)
	if err != nil || !reflect.DeepEqual(beforeState, afterState) ||
		before != interpretationSnapshot(t, s) {
		t.Fatal(
			"interpretation migration changed source, existing derived data or ingestion state",
			err,
		)
	}

	for _, successful := range []bool{true, false} {
		sequence := uint32(10)
		if !successful {
			sequence = 11
		}

		expected := testutil.InterpretationBundle(t, sequence, successful).Transactions[0]
		raw, err := s.Get(ctx, "transactions", expected.Hash)
		var actual domain.Transaction
		if err != nil || json.Unmarshal(raw, &actual) != nil ||
			!reflect.DeepEqual(expected.Interpretation, actual.Interpretation) {
			t.Fatal("migrated interpretation differs from live decoding", err)
		}
	}

	var stale int
	if err = s.Pool.QueryRow(ctx, `
		SELECT
		    count(*)
		FROM
		    transactions
		WHERE
		    interpretation_version != $1
	`, decoder.InterpretationVersion).Scan(&stale); err != nil || stale != 0 {
		t.Fatal("interpretation version not recorded", err)
	}
}

func TestInterpretationMigrationRollsBackCorruptSource(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	if err := s.Save(ctx, testutil.InterpretationBundle(t, 10, false)); err != nil {
		t.Fatal(err)
	}

	removeInterpretationMigration(t, s)
	if _, err := s.Pool.Exec(ctx, `
		UPDATE transactions
		SET
		    payload = jsonb_set(payload, '{result_xdr}', '"invalid"'::jsonb)
	`); err != nil {
		t.Fatal(err)
	}

	if err := s.Migrate(ctx); err == nil {
		t.Fatal("interpretation migration silently accepted corrupt result XDR")
	}

	var applied, columnExists bool
	if err := s.Pool.QueryRow(ctx, `
		SELECT
		    EXISTS (SELECT 1 FROM schema_migrations WHERE version = 7),
		    EXISTS (
		        SELECT 1 FROM information_schema.columns
		        WHERE table_schema = current_schema()
		          AND table_name = 'transactions'
		          AND column_name = 'interpretation_version'
		    )
	`).Scan(&applied, &columnExists); err != nil || applied || columnExists {
		t.Fatal("failed interpretation migration committed schema or marker", err)
	}
}

func removeInterpretationMigration(t *testing.T, s *storage.Store) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), `
		ALTER TABLE transactions DROP COLUMN interpretation_version;
		DELETE FROM schema_migrations WHERE version = 7;
		UPDATE transactions SET payload = payload - 'interpretation'
	`); err != nil {
		t.Fatal(err)
	}
}

func interpretationSnapshot(t *testing.T, s *storage.Store) string {
	t.Helper()
	var snapshot string
	if err := s.Pool.QueryRow(context.Background(), `
		SELECT
		    md5(string_agg(
		        t.envelope_xdr || t.result_xdr || t.meta_xdr ||
		        (t.payload - 'interpretation')::text || m.dataset_id,
		        '' ORDER BY t.hash
		    )) || (SELECT md5(string_agg(payload::text, '' ORDER BY transaction_hash, event_index))
		           FROM contract_events)
		FROM
		    transactions t
		    JOIN indexer_metadata m ON m.network = t.network
	`).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}

	return snapshot
}
