package storage_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

func TestSearchSuggestionsFindIndexedPrefixesAndExactIdentifiers(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	bundle := testutil.RealBundle(t)
	if err := s.Save(ctx, bundle); err != nil {
		t.Fatal(err)
	}

	transaction := bundle.Transactions[0]
	contract := ""
	for _, transaction := range bundle.Transactions {
		if len(transaction.Contracts) > 0 {
			contract = transaction.Contracts[0].ContractID
			break
		}
	}

	for _, scenario := range []struct {
		name  string
		query string
		kind  string
		key   string
		path  string
	}{
		{"hash_prefix", strings.ToUpper(transaction.Hash[:12]), "transaction", transaction.Hash, "/tx/"},
		{"exact_hash", strings.ToUpper(transaction.Hash), "transaction", transaction.Hash, "/tx/"},
		{"account_prefix", strings.ToLower(transaction.SourceAccount[:20]), "account", transaction.SourceAccount, "/accounts/"},
		{"exact_account", transaction.SourceAccount, "account", transaction.SourceAccount, "/accounts/"},
		{"contract_prefix", contract[:20], "contract", contract, "/contracts/"},
		{"exact_contract", contract, "contract", contract, "/contracts/"},
		{"ledger_prefix", "5569", "ledger", "5569284", "/ledgers/"},
		{"exact_ledger", "5569284", "ledger", "5569284", "/ledgers/"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			result, err := s.Suggest(ctx, scenario.query)
			if err != nil {
				t.Fatal(err)
			}

			found := false
			seen := map[string]bool{}
			for _, item := range result.Items {
				identity := item.Type + ":" + item.Key
				if seen[identity] {
					t.Fatal("repeated search result", identity)
				}

				seen[identity] = true
				if item.Type == scenario.kind && item.Key == scenario.key &&
					item.Path == scenario.path+scenario.key {
					found = true
				}
			}

			if !found || result.Network != domain.Network || len(result.Items) > result.Limit {
				t.Fatalf("missing or unbounded indexed suggestion: %+v", result)
			}
		})
	}

	result, err := s.Suggest(ctx, testutil.Address(99))
	if err != nil || len(result.Items) != 0 {
		t.Fatalf("unindexed account produced suggestions: %+v %v", result, err)
	}
}

func TestSearchLedgerRangesKeepExactMatchesAndLimitEveryType(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	for sequence := uint32(10); sequence <= 134; sequence++ {
		if err := s.Save(ctx, domain.Bundle{Ledger: testutil.Ledger(sequence)}); err != nil {
			t.Fatal(err)
		}
	}

	result, err := s.Suggest(ctx, "10")
	if err != nil || len(result.Items) != 5 || result.Items[0].Key != "10" {
		t.Fatalf("exact ledger was displaced by longer matches: %+v %v", result, err)
	}

	for _, item := range result.Items {
		if item.Type != "ledger" || !strings.HasPrefix(item.Key, "10") {
			t.Fatalf("ledger search escaped its numeric prefix: %+v", item)
		}
	}

	for _, prefix := range []string{"0", "abc", "GAB", "%", "G___", "1 OR 1=1", strings.Repeat("a", 65)} {
		if _, err = s.Suggest(ctx, prefix); err == nil {
			t.Fatal("accepted invalid prefix", prefix)
		}
	}
}

func TestSearchMigrationAddsIndexesWithoutChangingHistory(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	bundle := testutil.RealBundle(t)
	if err := s.Save(ctx, bundle); err != nil {
		t.Fatal(err)
	}

	before, err := s.Get(ctx, "transactions", bundle.Transactions[0].Hash)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = s.Pool.Exec(ctx, `
		DELETE FROM schema_migrations WHERE version = 13;
		DROP INDEX search_transaction_hash;
		DROP INDEX search_account_address;
		DROP INDEX search_contract_address;
	`); err != nil {
		t.Fatal(err)
	}

	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	after, err := s.Get(ctx, "transactions", bundle.Transactions[0].Hash)
	if err != nil || string(before) != string(after) || !json.Valid(after) {
		t.Fatal("search migration altered transaction evidence", err)
	}

	checkpoint, err := s.Checkpoint(ctx)
	if err != nil || checkpoint != bundle.Ledger.Sequence {
		t.Fatal("search migration changed the checkpoint", err)
	}

	if err = s.Migrate(ctx); err != nil {
		t.Fatal("migration is not restart-safe", err)
	}
}
