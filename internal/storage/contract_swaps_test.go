package storage_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/storage"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/jackc/pgx/v5"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestContractSwapMigrationEnrichesHistoryWithoutChangingItsIdentity(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	bundles := []domain.Bundle{
		testutil.SoroswapBundle(t, 10),
		testutil.FeeBundle(t, 11, false),
		testutil.StateBundle(t, 12, true),
	}
	for _, bundle := range bundles {
		if err := s.Save(ctx, bundle); err != nil {
			t.Fatal(err)
		}
	}

	before := contractSwapStorageSnapshot(t, s, true)
	removeContractSwapMigration(t, s)
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(before, contractSwapStorageSnapshot(t, s, true)) {
		t.Fatal(
			"contract swap migration changed source data, existing derived data or ingestion state",
		)
	}

	for _, bundle := range bundles {
		expected := bundle.Transactions[0]
		raw, err := s.Get(ctx, "transactions", expected.Hash)
		var actual domain.Transaction
		if err != nil || json.Unmarshal(raw, &actual) != nil ||
			!reflect.DeepEqual(actual.Interpretation, expected.Interpretation) {
			t.Fatal("contract swap migration differs from live source decoding", err)
		}
	}

	var stale int
	if err := s.Pool.QueryRow(ctx, `
		SELECT
		    count(*)
		FROM
		    transactions
		WHERE
		    interpretation_version != $1
	`, decoder.InterpretationVersion).Scan(&stale); err != nil || stale != 0 {
		t.Fatal("contract swap migration left an old interpretation version", err)
	}

	after := contractSwapStorageSnapshot(t, s, false)
	for replay := 0; replay < 2; replay++ {
		if err := s.Migrate(ctx); err != nil {
			t.Fatal(err)
		}

		if err := s.Save(ctx, bundles[0]); err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(after, contractSwapStorageSnapshot(t, s, false)) {
			t.Fatal("migration or ledger replay changed interpretation or existing identities")
		}
	}
}

func TestContractSwapMigrationRollsBackAllEnrichmentOnCorruptSource(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	// The invalid row follows a full backfill batch, so prior writes must roll back too.
	for sequence := uint32(10); sequence <= 110; sequence++ {
		if err := s.Save(ctx, testutil.SoroswapBundle(t, sequence)); err != nil {
			t.Fatal(err)
		}
	}

	removeContractSwapMigration(t, s)
	if _, err := s.Pool.Exec(ctx, `
		UPDATE transactions
		SET
		    payload = jsonb_set(payload, '{meta_xdr}', '"invalid"'::jsonb)
		WHERE
		    ledger_sequence = 110
	`); err != nil {
		t.Fatal(err)
	}

	before := contractSwapStorageSnapshot(t, s, false)
	if err := s.Migrate(ctx); err == nil {
		t.Fatal("contract swap migration silently accepted corrupt source metadata")
	}

	if !reflect.DeepEqual(before, contractSwapStorageSnapshot(t, s, false)) {
		t.Fatal("failed contract swap migration committed partial enrichment")
	}

	var applied bool
	if err := s.Pool.QueryRow(ctx, `
		SELECT
		    EXISTS (
		        SELECT
		            1
		        FROM
		            schema_migrations
		        WHERE
		            version = 9
		    )
	`).Scan(&applied); err != nil || applied {
		t.Fatal("failed contract swap migration committed its completion marker", err)
	}
}

func TestContractSwapMigrationRollsBackAfterQueuedWriteFailure(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	for sequence := uint32(10); sequence <= 11; sequence++ {
		if err := s.Save(ctx, testutil.SoroswapBundle(t, sequence)); err != nil {
			t.Fatal(err)
		}
	}

	removeContractSwapMigration(t, s)
	if _, err := s.Pool.Exec(ctx, `
		CREATE FUNCTION reject_second_contract_swap_upgrade()
		RETURNS trigger
		LANGUAGE plpgsql
		AS $$
		BEGIN
		    RAISE EXCEPTION 'blocked queued interpretation upgrade';
		END;
		$$;

		CREATE TRIGGER reject_second_contract_swap_upgrade
		BEFORE UPDATE OF interpretation_version ON transactions
		FOR EACH ROW
		WHEN (NEW.ledger_sequence = 11 AND NEW.interpretation_version > 2)
		EXECUTE FUNCTION reject_second_contract_swap_upgrade()
	`); err != nil {
		t.Fatal(err)
	}

	before := contractSwapStorageSnapshot(t, s, false)
	err := s.Migrate(ctx)
	if err == nil || !strings.Contains(err.Error(), "blocked queued interpretation upgrade") {
		t.Fatal("contract swap migration did not report the queued write failure", err)
	}

	if !reflect.DeepEqual(before, contractSwapStorageSnapshot(t, s, false)) {
		t.Fatal("queued write failure committed a prior update or changed existing data")
	}

	var applied bool
	var upgraded int
	if err = s.Pool.QueryRow(ctx, `
		SELECT
		    EXISTS (
		        SELECT
		            1
		        FROM
		            schema_migrations
		        WHERE
		            version = 9
		    ),
		    (
		        SELECT
		            count(*)
		        FROM
		            transactions
		        WHERE
		            interpretation_version != 2
		    )
	`).Scan(&applied, &upgraded); err != nil || applied || upgraded != 0 {
		t.Fatal("queued write failure committed a migration marker or partial upgrade", err)
	}
}

func TestContractSwapMigrationRejectsPayloadSourceMismatch(t *testing.T) {
	bundles := []domain.Bundle{
		testutil.SoroswapBundle(t, 10),
		testutil.SoroswapBundle(t, 11),
	}
	operations := []xdr.OperationMeta{}
	emptyMetadata, err := xdr.MarshalBase64(xdr.TransactionMeta{
		V:          0,
		Operations: &operations,
	})
	if err != nil {
		t.Fatal(err)
	}

	var changedResult xdr.TransactionResult
	if err = xdr.SafeUnmarshalBase64(bundles[0].Transactions[0].ResultXDR, &changedResult); err != nil {
		t.Fatal(err)
	}

	changedResult.FeeCharged++
	changedResultXDR, err := xdr.MarshalBase64(changedResult)
	if err != nil {
		t.Fatal(err)
	}

	if changedResultXDR == bundles[0].Transactions[0].ResultXDR {
		t.Fatal("result mismatch fixture must differ from the retained source")
	}

	for _, test := range []struct {
		name  string
		field string
		value any
	}{
		{
			name:  "network",
			field: "network",
			value: "mainnet",
		},
		{
			name:  "ledger sequence",
			field: "ledger_sequence",
			value: 999,
		},
		{
			name:  "transaction index",
			field: "tx_index",
			value: 99,
		},
		{
			name:  "another transaction hash",
			field: "hash",
			value: bundles[1].Transactions[0].Hash,
		},
		{
			name:  "another valid envelope XDR",
			field: "envelope_xdr",
			value: bundles[1].Transactions[0].EnvelopeXDR,
		},
		{
			name:  "another valid result XDR",
			field: "result_xdr",
			value: changedResultXDR,
		},
		{
			name:  "valid empty metadata XDR",
			field: "meta_xdr",
			value: emptyMetadata,
		},
		{
			name:  "source account",
			field: "source_account",
			value: testutil.Address(3),
		},
		{
			name:  "successful status",
			field: "successful",
			value: false,
		},
		{
			name:  "creation timestamp",
			field: "created_at",
			value: bundles[0].Transactions[0].CreatedAt.Add(time.Second),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := testutil.Database(t)
			ctx := context.Background()
			for _, bundle := range bundles {
				if err := s.Save(ctx, bundle); err != nil {
					t.Fatal(err)
				}
			}

			removeContractSwapMigration(t, s)
			value, err := json.Marshal(test.value)
			if err != nil {
				t.Fatal(err)
			}

			if _, err = s.Pool.Exec(ctx, `
				UPDATE transactions
				SET
				    payload = jsonb_set(payload, $1::text[], $2::jsonb)
				WHERE
				    ledger_sequence = 10
			`, []string{test.field}, value); err != nil {
				t.Fatal(err)
			}

			before := contractSwapStorageSnapshot(t, s, false)
			if err = s.Migrate(ctx); err == nil {
				t.Fatal("contract swap migration accepted inconsistent payload source")
			}

			if !reflect.DeepEqual(before, contractSwapStorageSnapshot(t, s, false)) {
				t.Fatal("rejected payload source changed a transaction or existing data")
			}

			var applied bool
			if err = s.Pool.QueryRow(ctx, `
				SELECT
				    EXISTS (
				        SELECT
				            1
				        FROM
				            schema_migrations
				        WHERE
				            version = 9
				    )
			`).Scan(&applied); err != nil || applied {
				t.Fatal("rejected payload source committed its migration marker", err)
			}
		})
	}
}

func removeContractSwapMigration(t *testing.T, s *storage.Store) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), `
		DELETE FROM
		    schema_migrations
		WHERE
		    version = 9;

		UPDATE transactions
		SET
		    interpretation_version = 2,
		    payload = jsonb_set(payload, '{interpretation}', '{
		        "version": 2,
		        "summary": "Legacy interpretation",
		        "actions": [],
		        "failure": null,
		        "diagnostic_errors": [],
		        "diagnostics_truncated": false,
		        "state_changes": []
		    }'::jsonb)
	`); err != nil {
		t.Fatal(err)
	}
}

func contractSwapStorageSnapshot(
	t *testing.T,
	s *storage.Store,
	omitInterpretation bool,
) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	for _, table := range []string{
		"ledgers",
		"transactions",
		"operations",
		"contract_calls",
		"contract_events",
		"fee_events",
		"assets",
		"token_transfers",
		"contracts",
		"transaction_contracts",
		"transaction_accounts",
		"indexer_metadata",
		"indexer_checkpoints",
		"indexer_status",
	} {
		projection := "to_jsonb(t)"
		if table == "transactions" && omitInterpretation {
			projection = `(to_jsonb(t) - 'payload' - 'interpretation_version') ||
			    jsonb_build_object('payload', t.payload - 'interpretation')`
		}

		query := fmt.Sprintf(`
			SELECT
			    COALESCE(jsonb_agg(row_data ORDER BY row_data::text), '[]'::jsonb)::text
			FROM
			    (
			        SELECT
			            %s AS row_data
			        FROM
			            %s t
			    ) source_rows
		`, projection, pgx.Identifier{table}.Sanitize())
		var rows string
		if err := s.Pool.QueryRow(context.Background(), query).Scan(&rows); err != nil {
			t.Fatal("snapshot "+table, err)
		}

		snapshot[table] = rows
	}

	return snapshot
}
