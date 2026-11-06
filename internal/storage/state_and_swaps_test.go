package storage_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestStateAndSwapsMigrationRebuildsOnlyInterpretation(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	bundles := []domain.Bundle{
		testutil.StateBundle(t, 10, true),
		testutil.StateBundle(t, 11, false),
		testutil.SwapBundle(t, 12),
	}
	for _, bundle := range bundles {
		if err := s.Save(ctx, bundle); err != nil {
			t.Fatal(err)
		}
	}

	state, err := s.State(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var before string
	if err = s.Pool.QueryRow(ctx, `
		SELECT
		    md5(string_agg(envelope_xdr || result_xdr || meta_xdr ||
		        (payload - 'interpretation')::text, '' ORDER BY hash))
		FROM
		    transactions
	`).Scan(&before); err != nil {
		t.Fatal(err)
	}

	if _, err = s.Pool.Exec(ctx, `
		DELETE FROM
		    schema_migrations
		WHERE
		    version = 8;

		UPDATE transactions
		SET
		    interpretation_version = 1,
		    payload = jsonb_set(payload, '{interpretation}',
		        ((payload->'interpretation') - 'state_changes') || '{"version":1}'::jsonb)
	`); err != nil {
		t.Fatal(err)
	}

	for replay := 0; replay < 2; replay++ {
		if err = s.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}

	var after string
	if err = s.Pool.QueryRow(ctx, `
		SELECT
		    md5(string_agg(envelope_xdr || result_xdr || meta_xdr ||
		        (payload - 'interpretation')::text, '' ORDER BY hash))
		FROM
		    transactions
	`).Scan(&after); err != nil || after != before {
		t.Fatal("migration changed source or unrelated payload", err)
	}

	afterState, err := s.State(ctx)
	if err != nil || !reflect.DeepEqual(state, afterState) {
		t.Fatal("migration changed checkpoint, dataset or counters", err)
	}

	for _, bundle := range bundles {
		expected := bundle.Transactions[0]
		raw, err := s.Get(ctx, "transactions", expected.Hash)
		var transaction domain.Transaction
		if err != nil || json.Unmarshal(raw, &transaction) != nil ||
			!reflect.DeepEqual(transaction.Interpretation, expected.Interpretation) ||
			transaction.Interpretation.Version != decoder.InterpretationVersion {
			t.Fatal("migrated state and swaps differ from live decoding", err)
		}
	}
}

func TestStateAndSwapsMigrationIsAtomicOnCorruptMetadata(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	for sequence := uint32(10); sequence <= 11; sequence++ {
		if err := s.Save(ctx, testutil.StateBundle(t, sequence, true)); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := s.Pool.Exec(ctx, `
		DELETE FROM
		    schema_migrations
		WHERE
		    version = 8;

		UPDATE transactions
		SET
		    interpretation_version = 1;

		UPDATE transactions
		SET
		    payload = jsonb_set(payload, '{meta_xdr}', '"invalid"'::jsonb)
		WHERE
		    ledger_sequence = 11
	`); err != nil {
		t.Fatal(err)
	}

	if err := s.Migrate(ctx); err == nil {
		t.Fatal("corrupt metadata was accepted")
	}

	var marker bool
	var upgraded int
	if err := s.Pool.QueryRow(ctx, `
		SELECT
		    EXISTS (
		        SELECT
		            1
		        FROM
		            schema_migrations
		        WHERE
		            version = 8
		    ),
		    (
		        SELECT
		            count(*)
		        FROM
		            transactions
		        WHERE
		            interpretation_version != 1
		    )
	`).Scan(&marker, &upgraded); err != nil || marker || upgraded != 0 {
		t.Fatal("failed migration committed a marker or partial enrichment", err)
	}
}

func TestBinaryStateStringsPersistAndMigrateWithoutLoss(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	change := testutil.InstanceChange(testutil.Contract(9), 10,
		xdr.LedgerEntryChangeTypeLedgerEntryCreated, testutil.WasmExecutable(1))
	change.Created.Data.ContractData.Key = testutil.String("key\x00")
	change.Created.Data.ContractData.Val = testutil.String("a\x00\xff")
	bundle := testutil.ContractBundle(t, 10, change)
	if err := s.Save(ctx, bundle); err != nil {
		t.Fatal("state strings prevented atomic ledger ingestion", err)
	}

	if _, err := s.Pool.Exec(ctx, `
		DELETE FROM
		    schema_migrations
		WHERE
		    version = 8;

		UPDATE transactions
		SET
		    interpretation_version = 1,
		    payload = payload - 'interpretation'
	`); err != nil {
		t.Fatal(err)
	}

	if err := s.Migrate(ctx); err != nil {
		t.Fatal("binary source strings prevented interpretation upgrade", err)
	}

	raw, err := s.Get(ctx, "transactions", bundle.Transactions[0].Hash)
	var transaction domain.Transaction
	if err != nil || json.Unmarshal(raw, &transaction) != nil ||
		!reflect.DeepEqual(transaction.Interpretation, bundle.Transactions[0].Interpretation) ||
		transaction.MetaXDR != bundle.Transactions[0].MetaXDR {
		t.Fatal("binary strings or their XDR evidence changed during persistence", err)
	}
}
