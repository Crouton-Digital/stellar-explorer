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

func TestNFTMigrationPreservesExistingDataAndAddsOnlySourceDerivedActivity(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	bundle := testutil.NFTBundle(t, 10)
	for _, source := range []domain.Bundle{bundle, testutil.SoroswapBundle(t, 11)} {
		if err := s.Save(ctx, source); err != nil {
			t.Fatal(err)
		}
	}

	removeNFTMigration(t, s)
	if _, err := s.Pool.Exec(ctx, `
		DELETE FROM
		    transaction_accounts
		WHERE
		    transaction_hash = $1
		    AND address = ANY($2::text[])
	`, bundle.Transactions[0].Hash, []string{testutil.Address(2), testutil.Address(3)}); err != nil {
		t.Fatal(err)
	}

	before := nftExistingDataSnapshot(t, s)
	for replay := 0; replay < 2; replay++ {
		if err := s.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}

	after := nftExistingDataSnapshot(t, s)
	accounts := before["transaction_accounts"]
	delete(before, "transaction_accounts")
	delete(after, "transaction_accounts")
	if !reflect.DeepEqual(before, after) {
		t.Fatal(
			"NFT migration changed raw data, interpretations, previous derived data or ingestion state",
		)
	}

	var relationsValid bool
	if err := s.Pool.QueryRow(ctx, `
		WITH previous AS (
		    SELECT
		        network, address, transaction_hash
		    FROM
		        jsonb_to_recordset($1::jsonb)
		            AS rows(network text, address text, transaction_hash text)
		), permitted AS (
		    SELECT
		        network, address, transaction_hash
		    FROM
		        previous
		    UNION
		    SELECT
		        network, from_address, transaction_hash
		    FROM
		        nft_transfers
		    WHERE
		        left(from_address, 1) = 'G'
		    UNION
		    SELECT
		        network, to_address, transaction_hash
		    FROM
		        nft_transfers
		    WHERE
		        left(to_address, 1) = 'G'
		)
		SELECT
		    NOT EXISTS (
		        SELECT * FROM transaction_accounts
		        EXCEPT
		        SELECT * FROM permitted
		    )
		    AND NOT EXISTS (
		        SELECT * FROM permitted
		        EXCEPT
		        SELECT * FROM transaction_accounts
		    )
	`, accounts).Scan(&relationsValid); err != nil || !relationsValid {
		t.Fatal("NFT participant enrichment removed old links or added unrelated addresses", err)
	}

	raw, err := s.Get(ctx, "transactions", bundle.Transactions[0].Hash)
	var actual domain.Transaction
	if err != nil || json.Unmarshal(raw, &actual) != nil ||
		!reflect.DeepEqual(actual.NFTTransfers, bundle.Transactions[0].NFTTransfers) ||
		!reflect.DeepEqual(actual.Participants, bundle.Transactions[0].Participants) {
		t.Fatal("NFT backfill differs from live decoding or rewrote participant payload", err)
	}

	if err := s.Save(ctx, bundle); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := s.Pool.QueryRow(ctx, "SELECT count(*) FROM nft_transfers").Scan(&count); err != nil ||
		count != 1 {
		t.Fatal("NFT ledger replay duplicated normalized activity", err)
	}
}

func TestNFTMigrationRollsBackSourcesParentConflictsAndQueuedWrites(t *testing.T) {
	for _, test := range []struct {
		name    string
		count   uint32
		prepare string
	}{
		{
			name:  "source decoding after one completed batch",
			count: 101,
			prepare: `
				UPDATE transactions
				SET
				    meta_xdr = 'invalid',
				    payload = jsonb_set(payload, '{meta_xdr}', '"invalid"'::jsonb)
				WHERE
				    ledger_sequence = 110
			`,
		},
		{
			name:  "payload identity conflicts",
			count: 2,
			prepare: `
				UPDATE transactions
				SET
				    payload = jsonb_set(payload, '{network}', '"mainnet"'::jsonb)
				WHERE
				    ledger_sequence = 11
			`,
		},
		{
			name:  "retained parent event conflicts",
			count: 2,
			prepare: `
				UPDATE contract_events
				SET
				    raw_xdr = 'different source'
				WHERE
				    ledger_sequence = 11
			`,
		},
		{
			name:  "second queued payload update fails",
			count: 2,
			prepare: `
				CREATE FUNCTION reject_nft_payload_update()
				RETURNS trigger
				LANGUAGE plpgsql
				AS $$
				BEGIN
				    RAISE EXCEPTION 'blocked NFT update';
				END;
				$$;

				CREATE TRIGGER reject_nft_payload_update
				BEFORE UPDATE OF payload ON transactions
				FOR EACH ROW
				WHEN (NEW.ledger_sequence = 11)
				EXECUTE FUNCTION reject_nft_payload_update()
			`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := testutil.Database(t)
			ctx := context.Background()
			for offset := uint32(0); offset < test.count; offset++ {
				if err := s.Save(ctx, testutil.NFTBundle(t, 10+offset)); err != nil {
					t.Fatal(err)
				}
			}

			removeNFTMigration(t, s)
			if _, err := s.Pool.Exec(ctx, test.prepare); err != nil {
				t.Fatal(err)
			}

			before := contractSwapStorageSnapshot(t, s, false)
			if err := s.Migrate(ctx); err == nil {
				t.Fatal("NFT migration accepted corrupt source or a failed write")
			}

			if !reflect.DeepEqual(before, contractSwapStorageSnapshot(t, s, false)) {
				t.Fatal("failed NFT migration changed prior rows, source data or ingestion state")
			}

			var applied bool
			var table *string
			if err := s.Pool.QueryRow(ctx, `
				SELECT
				    EXISTS (SELECT 1 FROM schema_migrations WHERE version = 12),
				    to_regclass(current_schema() || '.nft_transfers')::text
			`).Scan(&applied, &table); err != nil || applied || table != nil {
				t.Fatal("failed NFT migration committed its schema or completion marker", err)
			}
		})
	}
}

func removeNFTMigration(t *testing.T, s *storage.Store) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), `
		DROP TABLE nft_transfers;
		DROP TABLE nft_collections;
		DELETE FROM schema_migrations WHERE version = 12;
		UPDATE transactions SET payload = payload - 'nft_transfers'
	`); err != nil {
		t.Fatal(err)
	}
}

func nftExistingDataSnapshot(t *testing.T, s *storage.Store) map[string]string {
	t.Helper()
	snapshot := contractSwapStorageSnapshot(t, s, false)
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(snapshot["transactions"]), &rows); err != nil {
		t.Fatal(err)
	}

	for _, row := range rows {
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(row["payload"], &payload); err != nil {
			t.Fatal(err)
		}

		delete(payload, "nft_transfers")
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}

		row["payload"] = encoded
	}

	encoded, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}

	snapshot["transactions"] = string(encoded)

	return snapshot
}
