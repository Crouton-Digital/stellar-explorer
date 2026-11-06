package storage_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/storage"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func tokenBundle(t *testing.T, sequence uint32) domain.Bundle {
	t.Helper()
	asset := xdr.MustNewNativeAsset()
	contract := testutil.SACContract(asset)
	event := testutil.TokenEvent(
		contract,
		"transfer",
		[]string{testutil.Address(3), testutil.Address(4)},
		testutil.I128(9223372036854775807, 18446744073709551615),
		testutil.String("native"),
	)
	b := testutil.TokenBundle(t, sequence, event, event, event)

	return b
}

func TestTokenPersistenceReplayRollbackAndAccountHistory(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	b := tokenBundle(t, 10)
	broken := tokenBundle(t, 10)
	broken.Transactions[0].TokenTransfers[2].Amount = "-1"
	if e := s.Save(ctx, broken); e == nil {
		t.Fatal("negative movement accepted")
	}

	var assets, transfers int
	if e := s.Pool.QueryRow(ctx, `
		SELECT
		    (SELECT count(*) FROM assets),
		    (SELECT count(*) FROM token_transfers)
	`).Scan(&assets, &transfers); e != nil || assets != 0 || transfers != 0 {
		t.Fatalf("partial token data committed: %d %d %v", assets, transfers, e)
	}

	if checkpoint, _ := s.Checkpoint(ctx); checkpoint != 0 {
		t.Fatal("checkpoint advanced after token write failure")
	}

	for replay := 0; replay < 2; replay++ {
		if e := s.Save(ctx, b); e != nil {
			t.Fatal(e)
		}
	}

	if e := s.Pool.QueryRow(ctx, `
		SELECT
		    count(*)
		FROM
		    token_transfers
	`).Scan(&transfers); e != nil || transfers != 3 {
		t.Fatalf("token replay changed movement count: %d %v", transfers, e)
	}

	page, e := s.List(ctx, "account_transactions", testutil.Address(4), 20, nil)
	if e != nil || len(page.Items) != 1 {
		t.Fatal("token recipient absent from transaction history", e)
	}

	page, e = s.List(ctx, "transfers", "", 20, nil)
	if e != nil || len(page.Items) != 3 {
		t.Fatal("global movement list missing", e)
	}

	var movement domain.TokenTransfer
	if e = json.Unmarshal(page.Items[0], &movement); e != nil ||
		movement.Amount != "170141183460469231731687303715884105727" {
		t.Fatal("stored i128 lost precision", e)
	}
}

func TestTokenPaginationSnapshotAndScope(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	b := tokenBundle(t, 10)
	if e := s.Save(ctx, b); e != nil {
		t.Fatal(e)
	}

	contract := b.Transactions[0].TokenTransfers[0].Asset.ContractID
	for _, query := range []struct {
		kind  string
		scope string
	}{
		{"transfers", ""},
		{"asset_transfers", contract},
		{"account_transfers", testutil.Address(4)},
		{"contract_transfers", contract},
	} {
		var cursor *storage.Cursor
		seen := map[int]bool{}
		for requests := 0; requests < 4; requests++ {
			page, e := s.List(ctx, query.kind, query.scope, 1, cursor)
			if e != nil || len(page.Items) != 1 {
				t.Fatalf("%s page missing: %v", query.kind, e)
			}

			var movement domain.TokenTransfer
			if e = json.Unmarshal(page.Items[0], &movement); e != nil || seen[movement.EventIndex] {
				t.Fatal("duplicate movement page", e)
			}

			seen[movement.EventIndex] = true
			if page.NextCursor == nil {
				break
			}

			cursor, e = storage.ParseCursor(*page.NextCursor, query.kind, query.scope)
			if e != nil {
				t.Fatal(e)
			}

			if _, e = storage.ParseCursor(*page.NextCursor, query.kind, "another-scope"); e == nil {
				t.Fatal("cursor accepted for another address")
			}
		}

		if len(seen) != 3 {
			t.Fatal("movement pagination lost rows")
		}
	}

	page, e := s.List(ctx, "transfers", "", 1, nil)
	if e != nil || page.NextCursor == nil {
		t.Fatal(e)
	}

	cursor, _ := storage.ParseCursor(*page.NextCursor, "transfers", "")
	b = tokenBundle(t, 11)
	b.Transactions[0].Hash = fmt.Sprintf("%064x", 999)
	for index := range b.Transactions[0].Events {
		b.Transactions[0].Events[index].TransactionHash = b.Transactions[0].Hash
		b.Transactions[0].TokenTransfers[index].TransactionHash = b.Transactions[0].Hash
	}

	if e = s.Save(ctx, b); e != nil {
		t.Fatal(e)
	}

	page, e = s.List(ctx, "transfers", "", 20, cursor)
	if e != nil || len(page.Items) != 2 {
		t.Fatal("movement snapshot changed after ingestion", e)
	}
}

func TestTokenMigrationBackfillsWithoutChangingCheckpointOrRawData(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	b := tokenBundle(t, 10)
	if e := s.Save(ctx, b); e != nil {
		t.Fatal(e)
	}

	// Recreate a version-2 database with source events but no derived token data.
	if _, e := s.Pool.Exec(ctx, `
		DROP TABLE token_transfers;
		DROP TABLE assets;
		DELETE FROM schema_migrations WHERE version = 3;
		UPDATE transactions
		SET payload = payload - 'token_transfers'
	`); e != nil {
		t.Fatal(e)
	}

	if _, e := s.Pool.Exec(ctx, `
		DELETE FROM transaction_accounts
		WHERE
		    address = $1
	`, testutil.Address(4)); e != nil {
		t.Fatal(e)
	}

	before, e := s.State(ctx)
	if e != nil {
		t.Fatal(e)
	}

	for repetition := 0; repetition < 2; repetition++ {
		if e = s.Migrate(ctx); e != nil {
			t.Fatal(e)
		}
	}

	after, e := s.State(ctx)
	if e != nil || *after.LastIndexedLedger != *before.LastIndexedLedger ||
		after.ProcessedTransactions != before.ProcessedTransactions {
		t.Fatal("token migration changed ingestion checkpoint or counters", e)
	}

	raw, e := s.Get(ctx, "transactions", b.Transactions[0].Hash)
	if e != nil {
		t.Fatal(e)
	}

	var transaction domain.Transaction
	if e = json.Unmarshal(raw, &transaction); e != nil || len(transaction.TokenTransfers) != 3 ||
		transaction.MetaXDR != b.Transactions[0].MetaXDR || transaction.EnvelopeXDR != b.Transactions[0].EnvelopeXDR {
		t.Fatal("migration lost source data or failed to enrich history", e)
	}

	page, e := s.List(ctx, "account_transfers", testutil.Address(4), 20, nil)
	if e != nil || len(page.Items) != 3 {
		t.Fatal("migration lost token recipient history", e)
	}
}

func TestTokenMigrationRollsBackOnUnreadableSource(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	if e := s.Save(ctx, tokenBundle(t, 10)); e != nil {
		t.Fatal(e)
	}

	if _, e := s.Pool.Exec(ctx, `
		DROP TABLE token_transfers;
		DROP TABLE assets;
		DELETE FROM schema_migrations WHERE version = 3;
		UPDATE transactions
		SET payload = jsonb_set(payload, '{meta_xdr}', '"invalid"'::jsonb)
	`); e != nil {
		t.Fatal(e)
	}

	if e := s.Migrate(ctx); e == nil {
		t.Fatal("migration silently accepted unreadable metadata")
	}

	var applied bool
	if e := s.Pool.QueryRow(ctx, `
		SELECT
		    EXISTS (SELECT 1 FROM schema_migrations WHERE version = 3)
	`).Scan(&applied); e != nil || applied {
		t.Fatal("failed token migration was marked complete", e)
	}

	if checkpoint, e := s.Checkpoint(ctx); e != nil || checkpoint != 10 {
		t.Fatal("failed migration changed checkpoint", e)
	}
}

func TestAssetMetadataPromotionAndSnapshotPagination(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	asset := xdr.MustNewCreditAsset("USD", testutil.Address(9))
	contract := testutil.SACContract(asset)
	// Without the classic asset topic, a non-native SAC cannot yet be identified.
	unknown := testutil.TokenEvent(
		contract,
		"transfer",
		[]string{testutil.Address(3), testutil.Address(4)},
		testutil.I128(0, 1),
	)
	custom := testutil.TokenEvent(
		testutil.Contract(99),
		"burn",
		[]string{testutil.Address(3)},
		testutil.I128(0, 2),
	)
	b := testutil.TokenBundle(t, 10, unknown, custom)
	if e := s.Save(ctx, b); e != nil {
		t.Fatal(e)
	}

	first, e := s.List(ctx, "assets", "", 1, nil)
	if e != nil || first.NextCursor == nil {
		t.Fatal("asset pagination missing", e)
	}

	cursor, e := storage.ParseCursor(*first.NextCursor, "assets", "")
	if e != nil {
		t.Fatal(e)
	}

	verified := testutil.TokenEvent(
		contract,
		"transfer",
		[]string{testutil.Address(3), testutil.Address(4)},
		testutil.I128(0, 3),
		testutil.String(asset.StringCanonical()),
	)
	other := testutil.TokenEvent(
		testutil.Contract(98),
		"burn",
		[]string{testutil.Address(3)},
		testutil.I128(0, 4),
	)
	if e = s.Save(ctx, testutil.TokenBundle(t, 11, verified, unknown, other)); e != nil {
		t.Fatal(e)
	}

	page, e := s.List(ctx, "assets", "", 10, cursor)
	if e != nil || len(page.Items) != 1 {
		t.Fatal("new asset leaked into existing snapshot", e)
	}

	raw, e := s.Get(ctx, "assets", contract)
	if e != nil {
		t.Fatal(e)
	}

	var decoded domain.Asset
	if e = json.Unmarshal(raw, &decoded); e != nil || decoded.Kind != "sac" ||
		decoded.FirstObservedLedger != 10 || decoded.Decimals == nil {
		t.Fatal("verified metadata downgraded or first-observed position changed", e)
	}
}
