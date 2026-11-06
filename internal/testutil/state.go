package testutil

import (
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func AccountEntry(address string, balance int64) xdr.LedgerEntry {
	return xdr.LedgerEntry{
		LastModifiedLedgerSeq: 10,
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{
				AccountId: xdr.MustAddress(address),
				Balance:   xdr.Int64(balance),
				SeqNum:    9223372036854775807,
			},
		},
	}
}

func EntryChange(
	t *testing.T,
	kind xdr.LedgerEntryChangeType,
	entry xdr.LedgerEntry,
) xdr.LedgerEntryChange {
	t.Helper()
	var value any = entry
	if kind == xdr.LedgerEntryChangeTypeLedgerEntryRemoved {
		key, err := entry.LedgerKey()
		if err != nil {
			t.Fatal(err)
		}

		value = key
	}

	change, err := xdr.NewLedgerEntryChange(kind, value)
	if err != nil {
		t.Fatal(err)
	}

	return change
}

func StateBundle(t *testing.T, sequence uint32, successful bool) domain.Bundle {
	t.Helper()
	envelope := InvocationEnvelope("update")
	envelope.V1.Tx.SeqNum = xdr.SequenceNumber(sequence)
	before := AccountEntry(Address(1), 9223372036854775807)
	after := AccountEntry(Address(1), 9223372036854775707)
	meta := xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			TxChangesBefore: xdr.LedgerEntryChanges{
				EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryState, before),
				EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryUpdated, after),
			},
			Operations: []xdr.OperationMetaV2{
				{
					Changes: xdr.LedgerEntryChanges{
						InstanceChange(
							Contract(9),
							sequence,
							xdr.LedgerEntryChangeTypeLedgerEntryCreated,
							WasmExecutable(1),
						),
					},
				},
			},
		},
	}
	ledger := Ledger(sequence)
	transaction, err := decoder.Transaction(envelope, Result(envelope, successful), meta, ledger, 1)
	if err != nil {
		t.Fatal(err)
	}

	ledger.TransactionCount = 1

	return domain.Bundle{
		Ledger:       ledger,
		Transactions: []domain.Transaction{transaction},
	}
}
