package storage_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/storage"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

func TestAquariusMigrationEnrichesOnlyInterpretationAndPreservesReplay(t *testing.T) {
	bundles := []domain.Bundle{
		testutil.AquariusBundle(t, 10),
		testutil.AquariusChainedBundle(t, 11),
		testutil.SoroswapBundle(t, 12),
		testutil.FeeBundle(t, 13, false),
		testutil.StateBundle(t, 14, true),
	}
	checkAquariusMigrationPreservesHistory(t, 10, 3, bundles)
}

func checkAquariusMigrationPreservesHistory(
	t *testing.T,
	migrationVersion, previousVersion int,
	bundles []domain.Bundle,
) {
	t.Helper()
	s := testutil.Database(t)
	ctx := context.Background()
	for _, bundle := range bundles {
		if err := s.Save(ctx, bundle); err != nil {
			t.Fatal(err)
		}
	}

	before := contractSwapStorageSnapshot(t, s, true)
	removeAquariusMigration(t, s, migrationVersion, previousVersion)
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(before, contractSwapStorageSnapshot(t, s, true)) {
		t.Fatal("Aquarius migration changed source data, existing derived data or ingestion state")
	}

	for _, bundle := range bundles {
		expected := bundle.Transactions[0]
		raw, err := s.Get(ctx, "transactions", expected.Hash)
		var actual domain.Transaction
		if err != nil || json.Unmarshal(raw, &actual) != nil ||
			!reflect.DeepEqual(actual.Interpretation, expected.Interpretation) {
			t.Fatal("migrated Aquarius interpretation differs from live decoding", err)
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
		t.Fatal("Aquarius migration left an old interpretation version", err)
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
			t.Fatal("Aquarius migration or ledger replay changed stored data")
		}
	}
}

func TestAquariusMigrationRollsBackSourceAndQueuedWriteFailures(t *testing.T) {
	checkAquariusMigrationRollsBack(t, 10, 3, testutil.AquariusBundle)
}

func checkAquariusMigrationRollsBack(
	t *testing.T,
	migrationVersion, previousVersion int,
	bundle func(*testing.T, uint32) domain.Bundle,
) {
	t.Helper()
	for _, test := range []struct {
		name         string
		transactions uint32
		prepareSQL   string
		errorText    string
	}{
		{
			name:         "invalid metadata after a complete batch",
			transactions: 101,
			prepareSQL: `
				UPDATE transactions
				SET
				    meta_xdr = 'invalid',
				    payload = jsonb_set(payload, '{meta_xdr}', '"invalid"'::jsonb)
				WHERE
				    ledger_sequence = 110
			`,
			errorText: "interpretation metadata",
		},
		{
			name:         "second queued update rejected",
			transactions: 2,
			prepareSQL: `
				CREATE FUNCTION reject_second_aquarius_upgrade()
				RETURNS trigger
				LANGUAGE plpgsql
				AS $$
				BEGIN
				    RAISE EXCEPTION 'blocked queued Aquarius upgrade';
				END;
				$$;

				CREATE TRIGGER reject_second_aquarius_upgrade
				BEFORE UPDATE OF interpretation_version ON transactions
				FOR EACH ROW
				WHEN (NEW.ledger_sequence = 11 AND NEW.interpretation_version > OLD.interpretation_version)
				EXECUTE FUNCTION reject_second_aquarius_upgrade()
			`,
			errorText: "blocked queued Aquarius upgrade",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := testutil.Database(t)
			ctx := context.Background()
			for offset := uint32(0); offset < test.transactions; offset++ {
				if err := s.Save(ctx, bundle(t, 10+offset)); err != nil {
					t.Fatal(err)
				}
			}

			removeAquariusMigration(t, s, migrationVersion, previousVersion)
			if _, err := s.Pool.Exec(ctx, test.prepareSQL); err != nil {
				t.Fatal(err)
			}

			before := contractSwapStorageSnapshot(t, s, false)
			if err := s.Migrate(ctx); err == nil || !strings.Contains(err.Error(), test.errorText) {
				t.Fatal("Aquarius migration did not report the expected failure", err)
			}

			if !reflect.DeepEqual(before, contractSwapStorageSnapshot(t, s, false)) {
				t.Fatal("failed Aquarius migration committed partial enrichment")
			}

			var applied bool
			var upgraded int
			if err := s.Pool.QueryRow(ctx, `
				SELECT
				    EXISTS (
				        SELECT
				            1
				        FROM
				            schema_migrations
				        WHERE
				            version = $1
				    ),
				    (
				        SELECT
				            count(*)
				        FROM
				            transactions
				        WHERE
				            interpretation_version != $2
				    )
			`, migrationVersion, previousVersion).Scan(&applied, &upgraded); err != nil ||
				applied || upgraded != 0 {
				t.Fatal("failed Aquarius migration committed its marker or a partial upgrade", err)
			}
		})
	}
}

func removeAquariusMigration(
	t *testing.T,
	s *storage.Store,
	migrationVersion, previousVersion int,
) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), `
		DELETE FROM
		    schema_migrations
		WHERE
		    version = $1
	`, migrationVersion); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Pool.Exec(context.Background(), `
		UPDATE transactions
		SET
		    interpretation_version = $1,
		    payload = jsonb_set(
		        jsonb_set(payload, '{interpretation,version}', to_jsonb($1::integer)),
		        '{interpretation,actions}',
		        (
		            SELECT
		                COALESCE(jsonb_agg(
		                    CASE
		                        WHEN $2 = 10
		                          OR action->'aquarius_swap'->>'mode' = 'exact_output'
		                          OR action->'aquarius_swap'->>'routing' = 'split'
		                        THEN action - 'aquarius_swap'
		                        ELSE action #- '{aquarius_swap,settlement}' #- '{aquarius_swap,routes}'
		                    END
		                    ORDER BY ordinal
		                ), '[]'::jsonb)
		            FROM
		                jsonb_array_elements(payload->'interpretation'->'actions')
		                    WITH ORDINALITY AS actions(action, ordinal)
		        )
		    )
	`, previousVersion, migrationVersion); err != nil {
		t.Fatal(err)
	}
}
