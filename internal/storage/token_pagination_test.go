package storage_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/storage"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

func TestTokenPaginationAcrossTransactions(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	b := tokenBundle(t, 10)
	second := tokenBundle(t, 11).Transactions[0]
	second.LedgerSequence = b.Ledger.Sequence
	second.CreatedAt = b.Ledger.ClosedAt
	second.TxIndex = 2
	for index := range second.Contracts {
		contract := &second.Contracts[index]
		contract.FirstObservedLedger = b.Ledger.Sequence
		contract.LastObservedLedger = b.Ledger.Sequence
		if contract.ExecutableLedger != nil {
			contract.ExecutableLedger = &second.LedgerSequence
			contract.ExecutableTxIndex = &second.TxIndex
		}
	}

	for index := range second.Events {
		second.Events[index].LedgerSequence = b.Ledger.Sequence
		second.TokenTransfers[index].LedgerSequence = b.Ledger.Sequence
		second.TokenTransfers[index].TxIndex = second.TxIndex
		second.TokenTransfers[index].CreatedAt = second.CreatedAt
		second.TokenTransfers[index].Asset.FirstObservedLedger = b.Ledger.Sequence
	}

	b.Ledger.TransactionCount = 2
	b.Transactions = append(b.Transactions, second)
	expected := map[string]bool{}
	transactions := map[string]bool{}
	for _, transaction := range b.Transactions {
		for _, movement := range transaction.TokenTransfers {
			key := fmt.Sprintf("%s:%d", transaction.Hash, movement.EventIndex)
			expected[key] = true
			transactions[transaction.Hash] = true
		}
	}

	if len(transactions) < 2 {
		t.Fatal("fixture must contain movements in multiple transactions")
	}

	if e := s.Save(ctx, b); e != nil {
		t.Fatal(e)
	}

	var cursor *storage.Cursor
	var previous *domain.TokenTransfer
	seen := map[string]bool{}
	for requests := 0; requests <= len(expected); requests++ {
		page, e := s.List(ctx, "transfers", "", 1, cursor)
		if e != nil || len(page.Items) != 1 {
			t.Fatal("missing movement page", e)
		}

		var movement domain.TokenTransfer
		if e = json.Unmarshal(page.Items[0], &movement); e != nil {
			t.Fatal(e)
		}

		key := fmt.Sprintf("%s:%d", movement.TransactionHash, movement.EventIndex)
		if seen[key] || !expected[key] {
			t.Fatal("unexpected or repeated movement", key)
		}

		if previous != nil && (movement.TxIndex > previous.TxIndex ||
			(movement.TxIndex == previous.TxIndex && movement.EventIndex >= previous.EventIndex)) {
			t.Fatal("movements are not in reverse application order")
		}

		previous = &movement
		seen[key] = true
		if page.NextCursor == nil {
			break
		}

		cursor, e = storage.ParseCursor(*page.NextCursor, "transfers", "")
		if e != nil {
			t.Fatal(e)
		}
	}

	if len(seen) != len(expected) {
		t.Fatal("pagination lost movements")
	}
}
