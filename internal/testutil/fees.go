package testutil

import (
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func FeeEvent(account string, amount int64, stage xdr.TransactionEventStage) xdr.TransactionEvent {
	hi := int64(0)
	if amount < 0 {
		hi = -1
	}

	return xdr.TransactionEvent{
		Stage: stage,
		Event: TokenEvent(
			SACContract(xdr.MustNewNativeAsset()),
			"fee",
			[]string{account},
			I128(hi, uint64(amount)),
		),
	}
}

func FeeBundle(t *testing.T, sequence uint32, successful bool) domain.Bundle {
	t.Helper()
	env := PaymentEnvelope()
	env.V1.Tx.SeqNum = xdr.SequenceNumber(sequence)
	movement := TokenEvent(
		SACContract(xdr.MustNewNativeAsset()),
		"transfer",
		[]string{Address(1), Address(2)},
		I128(0, 10000001),
		String("native"),
	)
	meta := xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			Operations: []xdr.OperationMetaV2{
				{Events: []xdr.ContractEvent{movement}},
			},
			Events: []xdr.TransactionEvent{
				FeeEvent(Address(1), 1000,
					xdr.TransactionEventStageTransactionEventStageBeforeAllTxs),
				FeeEvent(Address(1), -900,
					xdr.TransactionEventStageTransactionEventStageAfterAllTxs),
			},
		},
	}
	ledger := Ledger(sequence)
	transaction, err := decoder.Transaction(env, Result(env, successful), meta, ledger, 1)
	if err != nil {
		t.Fatal(err)
	}

	ledger.TransactionCount = 1

	return domain.Bundle{
		Ledger:       ledger,
		Transactions: []domain.Transaction{transaction},
	}
}
