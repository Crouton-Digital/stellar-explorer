package storage_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/storage"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func filterBundle(t *testing.T) domain.Bundle {
	t.Helper()
	bundle := tokenBundle(t, 10)
	first := &bundle.Transactions[0]
	// A synthetic normalized multi-call row verifies that contract and function match one call.
	first.Calls = []domain.Call{
		{
			Index:        0,
			ContractID:   testutil.Contract(9),
			FunctionName: "transfer",
			Arguments:    []any{},
			DecodeStatus: "decoded",
		},
		{
			Index:        1,
			ContractID:   testutil.Contract(8),
			FunctionName: "balance",
			Arguments:    []any{},
			DecodeStatus: "decoded",
		},
	}
	for _, address := range []string{testutil.Contract(8), testutil.Contract(9)} {
		first.Contracts = append(first.Contracts, domain.Contract{
			Network:             domain.Network,
			ContractID:          address,
			Kind:                "unknown",
			FirstObservedLedger: 10,
			LastObservedLedger:  10,
		})
	}

	second := testutil.ContractBundle(t, 10).Transactions[0]
	second.TxIndex = 2
	env := testutil.ContractEnvelope()
	env.V1.Tx.SeqNum = 99
	env.V1.Tx.Operations[0].Body.InvokeHostFunctionOp.HostFunction.InvokeContract.ContractAddress = testutil.ContractAddress(
		testutil.Contract(8),
	)
	meta := xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{}}
	failed, err := decoder.Transaction(env, testutil.Result(env, false), meta, bundle.Ledger, 3)
	if err != nil {
		t.Fatal(err)
	}

	bundle.Transactions = append(bundle.Transactions, second, failed)
	bundle.Ledger.TransactionCount = 3

	return bundle
}

func TestTransactionFiltersMatchTogetherWithoutDuplicates(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	bundle := filterBundle(t)
	if err := s.Save(ctx, bundle); err != nil {
		t.Fatal(err)
	}

	for _, scenario := range []struct {
		name    string
		filters domain.Filters
		indexes []int
	}{
		{"all", domain.Filters{}, []int{2, 1, 0}},
		{"recipient", domain.Filters{Address: testutil.Address(4)}, []int{0}},
		{"contract_participant", domain.Filters{Address: testutil.Contract(8)}, []int{2, 0}},
		{"contract", domain.Filters{Contract: testutil.Contract(9)}, []int{1, 0}},
		{"function", domain.Filters{Function: "run"}, []int{2, 1}},
		{"matching_call", domain.Filters{Contract: testutil.Contract(8), Function: "balance"}, []int{0}},
		{"different_calls", domain.Filters{Contract: testutil.Contract(9), Function: "balance"}, nil},
		{"failed", domain.Filters{Status: "failed"}, []int{2}},
		{"success", domain.Filters{Status: "success"}, []int{1, 0}},
		{"inclusive", domain.Filters{MinLedger: 10, MaxLedger: 10}, []int{2, 1, 0}},
		{"below", domain.Filters{MaxLedger: 9}, nil},
		{"above", domain.Filters{MinLedger: 11}, nil},
		{"combined", domain.Filters{Contract: testutil.Contract(9), Function: "run", Status: "success", MinLedger: 10}, []int{1}},
		{"case_sensitive", domain.Filters{Function: "RUN"}, nil},
		{"unknown_address", domain.Filters{Address: testutil.Address(99)}, nil},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			page, err := s.List(ctx, "transactions", "", 20, nil, scenario.filters)
			if err != nil || len(page.Items) != len(scenario.indexes) {
				t.Fatalf(
					"unexpected filtered rows: %d, want %d: %v",
					len(page.Items),
					len(scenario.indexes),
					err,
				)
			}

			for index, expected := range scenario.indexes {
				var transaction domain.Transaction
				if err = json.Unmarshal(page.Items[index], &transaction); err != nil ||
					transaction.Hash != bundle.Transactions[expected].Hash {
					t.Fatal("incorrect match or order", err)
				}
			}
		})
	}
}

func TestEventAndMovementFiltersPreserveEntityScope(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	bundle := tokenBundle(t, 10)
	if err := s.Save(ctx, bundle); err != nil {
		t.Fatal(err)
	}

	asset := bundle.Transactions[0].TokenTransfers[0].Asset.ContractID
	for _, scenario := range []struct {
		name    string
		kind    string
		scope   string
		filters domain.Filters
		count   int
	}{
		{"event_contract", "events", "", domain.Filters{Contract: asset, EventType: "contract", EventName: "transfer"}, 3},
		{"event_system", "events", "", domain.Filters{EventType: "system"}, 0},
		{"event_name", "events", "", domain.Filters{EventName: "burn"}, 0},
		{"event_range", "events", "", domain.Filters{MinLedger: 11}, 0},
		{"event_scope", "events", asset, domain.Filters{Contract: testutil.Contract(99)}, 0},
		{"movement_asset", "transfers", "", domain.Filters{Asset: asset}, 3},
		{"movement_parties", "transfers", "", domain.Filters{From: testutil.Address(3), To: testutil.Address(4), MinLedger: 10, MaxLedger: 10}, 3},
		{"movement_direction", "transfers", "", domain.Filters{From: testutil.Address(4)}, 0},
		{"movement_recipient", "transfers", "", domain.Filters{To: testutil.Address(3)}, 0},
		{"movement_range", "transfers", "", domain.Filters{MaxLedger: 9}, 0},
		{"account_scope", "account_transfers", testutil.Address(4), domain.Filters{Asset: asset}, 3},
		{"asset_scope", "asset_transfers", asset, domain.Filters{Asset: testutil.Contract(99)}, 0},
		{"contract_scope", "contract_transfers", asset, domain.Filters{From: testutil.Address(3)}, 3},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			page, err := s.List(ctx, scenario.kind, scenario.scope, 20, nil, scenario.filters)
			if err != nil || len(page.Items) != scenario.count {
				t.Fatalf("unexpected result: %d, want %d: %v", len(page.Items), scenario.count, err)
			}
		})
	}
}

func TestFilteredCursorSnapshotAndQueryBinding(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	if err := s.Save(ctx, filterBundle(t)); err != nil {
		t.Fatal(err)
	}

	filters := domain.Filters{Contract: testutil.Contract(9), Status: "success"}
	first, err := s.List(ctx, "transactions", "", 1, nil, filters)
	if err != nil || first.NextCursor == nil {
		t.Fatal("filtered cursor missing", err)
	}

	cursor, err := storage.ParseCursor(*first.NextCursor, "transactions", "", filters)
	if err != nil {
		t.Fatal(err)
	}

	if err = s.Save(ctx, testutil.ContractBundle(t, 11)); err != nil {
		t.Fatal(err)
	}

	next, err := s.List(ctx, "transactions", "", 20, cursor, filters)
	if err != nil || len(next.Items) != 1 || next.NextCursor != nil ||
		string(next.Items[0]) == string(first.Items[0]) {
		t.Fatal("filtered pagination repeated or leaked new transactions", err)
	}

	for _, changed := range []domain.Filters{
		{},
		{Contract: testutil.Contract(8), Status: "success"},
		{Contract: testutil.Contract(9), Status: "failed"},
		{Contract: testutil.Contract(9), Status: "success", MinLedger: 10},
	} {
		if _, err = storage.ParseCursor(*first.NextCursor, "transactions", "", changed); err == nil {
			t.Fatal("cursor reused with different filters")
		}

		if _, err = s.List(ctx, "transactions", "", 20, cursor, changed); !errors.Is(
			err,
			storage.ErrInvalidCursor,
		) {
			t.Fatal("storage accepted mismatched cursor filters", err)
		}
	}

	for _, kind := range []string{"events", "transfers"} {
		filters := domain.Filters{MinLedger: 10, MaxLedger: 10}
		page, err := s.List(ctx, kind, "", 1, nil, filters)
		if err != nil || page.NextCursor == nil {
			t.Fatal("missing event/movement cursor", kind, err)
		}

		cursor, err := storage.ParseCursor(*page.NextCursor, kind, "", filters)
		if err != nil {
			t.Fatal(err)
		}

		page, err = s.List(ctx, kind, "", 20, cursor, filters)
		if err != nil || len(page.Items) != 2 {
			t.Fatal("filtered event/movement pagination lost rows", kind, err)
		}
	}
}
