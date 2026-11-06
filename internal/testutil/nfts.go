package testutil

import (
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func U32(value uint32) xdr.ScVal {
	number := xdr.Uint32(value)

	return xdr.ScVal{
		Type: xdr.ScValTypeScvU32,
		U32:  &number,
	}
}

func U64(value uint64) xdr.ScVal {
	number := xdr.Uint64(value)

	return xdr.ScVal{
		Type: xdr.ScValTypeScvU64,
		U64:  &number,
	}
}

func U256(highest, high, low, lowest uint64) xdr.ScVal {
	return xdr.ScVal{
		Type: xdr.ScValTypeScvU256,
		U256: &xdr.UInt256Parts{
			HiHi: xdr.Uint64(highest),
			HiLo: xdr.Uint64(high),
			LoHi: xdr.Uint64(low),
			LoLo: xdr.Uint64(lowest),
		},
	}
}

func NFTEvent(
	contract, kind string,
	addresses []string,
	identifier xdr.ScVal,
	mapped bool,
) xdr.ContractEvent {
	data := identifier
	if mapped {
		data = Map(xdr.ScMap{
			{
				Key: Symbol("token_id"),
				Val: identifier,
			},
		})
	}

	return TokenEvent(contract, kind, addresses, data)
}

func NFTSource(
	t *testing.T,
	events ...xdr.ContractEvent,
) (xdr.TransactionEnvelope, xdr.TransactionResultPair, xdr.TransactionMeta) {
	t.Helper()
	if len(events) == 0 {
		events = []xdr.ContractEvent{
			NFTEvent(Contract(30), "transfer", []string{Address(2), Address(3)}, U32(7), false),
		}
	}

	envelope := InvocationEnvelope("transfer")
	call := envelope.V1.Tx.Operations[0].Body.InvokeHostFunctionOp.HostFunction.InvokeContract
	call.ContractAddress = ContractAddress(Contract(30))
	call.Args = []xdr.ScVal{
		AddressValue(Address(2)),
		AddressValue(Address(3)),
		U32(7),
	}
	returned := xdr.ScVal{Type: xdr.ScValTypeScvVoid}
	meta := xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			Operations:  []xdr.OperationMetaV2{{Events: events}},
			SorobanMeta: &xdr.SorobanTransactionMetaV2{ReturnValue: &returned},
		},
	}
	result := Result(envelope, true)
	NFTCommitment(t, &result, meta)

	return envelope, result, meta
}

func NFTCommitment(
	t *testing.T,
	result *xdr.TransactionResultPair,
	meta xdr.TransactionMeta,
) {
	t.Helper()
	SetSoroswapCommitment(t, result, meta)
}

func NFTBundle(t *testing.T, sequence uint32, events ...xdr.ContractEvent) domain.Bundle {
	t.Helper()
	envelope, result, meta := NFTSource(t, events...)
	envelope.V1.Tx.SeqNum = xdr.SequenceNumber(sequence)
	identity := Result(envelope, true)
	result.TransactionHash = identity.TransactionHash
	ledger := Ledger(sequence)
	transaction, err := decoder.Transaction(envelope, result, meta, ledger, 1)
	if err != nil {
		t.Fatal(err)
	}

	ledger.TransactionCount = 1

	return domain.Bundle{
		Ledger:       ledger,
		Transactions: []domain.Transaction{transaction},
	}
}
