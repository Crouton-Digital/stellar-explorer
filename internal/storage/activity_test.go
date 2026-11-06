package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/storage"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func activityBundle(t *testing.T, sequence uint32) domain.Bundle {
	t.Helper()
	ledger := testutil.Ledger(sequence)
	result := domain.Bundle{Ledger: ledger}
	for index, successful := range []bool{true, false, true} {
		envelope := testutil.PaymentEnvelope()
		envelope.V1.Tx.SeqNum = xdr.SequenceNumber(uint64(sequence)*10 + uint64(index))
		transaction, err := decoder.Transaction(
			envelope,
			testutil.Result(envelope, successful),
			xdr.TransactionMeta{V: 2, V2: &xdr.TransactionMetaV2{}},
			ledger,
			uint32(index+1),
		)
		if err != nil {
			t.Fatal(err)
		}

		result.Transactions = append(result.Transactions, transaction)
	}

	result.Ledger.TransactionCount = len(result.Transactions)

	return result
}

func TestActivityWindowIncludesZeroLedgersAndKeepsListSnapshot(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	for sequence := uint32(10); sequence <= 140; sequence++ {
		bundle := domain.Bundle{Ledger: testutil.Ledger(sequence)}
		if sequence == 10 || sequence == 140 {
			bundle = activityBundle(t, sequence)
		}

		if err := s.Save(ctx, bundle); err != nil {
			t.Fatal(err)
		}
	}

	activity, err := s.Activity(ctx, "transactions", nil, domain.Filters{})
	if err != nil {
		t.Fatal(err)
	}

	if activity.FirstLedger == nil || *activity.FirstLedger != 21 ||
		activity.LastLedger == nil || *activity.LastLedger != 140 ||
		activity.LedgerCount != 120 || activity.Total != 3 || len(activity.Buckets) != 12 ||
		activity.From == nil || !activity.From.Equal(testutil.Ledger(21).ClosedAt) ||
		activity.To == nil || !activity.To.Equal(testutil.Ledger(140).ClosedAt) {
		t.Fatalf("incorrect bounded coverage: %+v", activity)
	}

	for index, bucket := range activity.Buckets {
		expected := uint64(0)
		if index == 11 {
			expected = 3
		}

		if bucket.Count != expected || bucket.FirstLedger != uint32(21+index*10) ||
			bucket.LastLedger != uint32(30+index*10) {
			t.Fatalf("incorrect zero-filled bucket %d: %+v", index, bucket)
		}
	}

	if activity.Breakdown[0].Count != 2 || activity.Breakdown[1].Count != 1 {
		t.Fatalf("success and failure totals disagree: %+v", activity.Breakdown)
	}

	page, err := s.List(ctx, "transactions", "", 1, nil)
	if err != nil || page.NextCursor == nil {
		t.Fatal("missing transaction cursor", err)
	}

	cursor, err := storage.ParseCursor(*page.NextCursor, "transactions", "")
	if err != nil {
		t.Fatal(err)
	}

	if err = s.Save(ctx, activityBundle(t, 141)); err != nil {
		t.Fatal(err)
	}

	anchored, err := s.Activity(ctx, "transactions", cursor, domain.Filters{})
	if err != nil || anchored.Total != 3 || *anchored.LastLedger != 140 {
		t.Fatalf("activity leaked beyond the list snapshot: %+v %v", anchored, err)
	}

	latest, err := s.Activity(ctx, "transactions", nil, domain.Filters{})
	if err != nil || latest.Total != 6 || *latest.LastLedger != 141 {
		t.Fatalf("new window missed new committed rows: %+v %v", latest, err)
	}

	for _, mutate := range []func(*storage.Cursor){
		func(c *storage.Cursor) {
			c.Dataset = "another-dataset"
		},
		func(c *storage.Cursor) {
			c.Kind = "events"
		},
		func(c *storage.Cursor) {
			c.Scope = "another-scope"
		},
		func(c *storage.Cursor) {
			c.Snapshot = 142
		},
		func(c *storage.Cursor) {
			c.Filters = &domain.Filters{Status: "failed"}
		},
	} {
		changed := *cursor
		mutate(&changed)
		if _, err = s.Activity(ctx, "transactions", &changed, domain.Filters{}); !errors.Is(
			err,
			storage.ErrInvalidCursor,
		) {
			t.Fatal("activity accepted a cursor from another query", err)
		}
	}
}

func TestActivityUsesAllMatchingRowsAndExactListFilters(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	if err := s.Save(ctx, filterBundle(t)); err != nil {
		t.Fatal(err)
	}

	for _, scenario := range []struct {
		name    string
		kind    string
		filters domain.Filters
		count   uint64
	}{
		{"all_transactions", "transactions", domain.Filters{}, 3},
		{"failed", "transactions", domain.Filters{Status: "failed"}, 1},
		{"same_call", "transactions", domain.Filters{Contract: testutil.Contract(8), Function: "balance"}, 1},
		{"different_calls", "transactions", domain.Filters{Contract: testutil.Contract(9), Function: "balance"}, 0},
		{"recipient", "transactions", domain.Filters{Address: testutil.Address(4)}, 1},
		{"events", "events", domain.Filters{EventName: "transfer", EventType: "contract", EventScope: "operation"}, 3},
		{"other_events", "events", domain.Filters{EventType: "system"}, 0},
		{"movements", "transfers", domain.Filters{From: testutil.Address(3), To: testutil.Address(4)}, 3},
		{"opposite_direction", "transfers", domain.Filters{From: testutil.Address(4)}, 0},
		{"outside_range", "transactions", domain.Filters{MinLedger: 11}, 0},
		{"inclusive_range", "transactions", domain.Filters{MinLedger: 10, MaxLedger: 10}, 3},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			activity, err := s.Activity(ctx, scenario.kind, nil, scenario.filters)
			if err != nil || activity.Total != scenario.count {
				t.Fatalf("incorrect matching count: %+v %v", activity, err)
			}

			page, err := s.List(ctx, scenario.kind, "", 100, nil, scenario.filters)
			if err != nil || uint64(len(page.Items)) != activity.Total {
				t.Fatalf("activity and complete filtered list disagree: %+v %v", page, err)
			}

			var buckets, categories uint64
			for _, bucket := range activity.Buckets {
				buckets += bucket.Count
			}

			for _, category := range activity.Breakdown {
				categories += category.Count
			}

			if buckets != activity.Total || categories != activity.Total {
				t.Fatal("activity categories or buckets do not reconcile")
			}
		})
	}
}

func TestActivityEmptyDatasetHasNoFabricatedTimeOrRange(t *testing.T) {
	s := testutil.Database(t)
	for _, kind := range []string{"transactions", "events", "transfers"} {
		activity, err := s.Activity(context.Background(), kind, nil, domain.Filters{})
		if err != nil || activity.Total != 0 || activity.LedgerCount != 0 ||
			activity.FirstLedger != nil || activity.LastLedger != nil || activity.From != nil ||
			activity.To != nil || len(activity.Buckets) != 0 {
			t.Fatalf("empty dataset has misleading activity: %+v %v", activity, err)
		}
	}

	if _, err := s.Activity(context.Background(), "unknown", nil, domain.Filters{}); err == nil {
		t.Fatal("accepted unsupported aggregation")
	}
}
