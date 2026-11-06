package testutil

import (
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func ContractAddress(address string) xdr.ScAddress {
	raw := strkey.MustDecode(strkey.VersionByteContract, address)
	var id xdr.ContractId
	copy(id[:], raw)

	return xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &id,
	}
}

func ContractEnvelope() xdr.TransactionEnvelope {
	env := PaymentEnvelope()
	env.V1.Tx.Operations[0].Body = xdr.OperationBody{
		Type: xdr.OperationTypeInvokeHostFunction,
		InvokeHostFunctionOp: &xdr.InvokeHostFunctionOp{
			HostFunction: xdr.HostFunction{
				Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
				InvokeContract: &xdr.InvokeContractArgs{
					ContractAddress: ContractAddress(Contract(9)),
					FunctionName:    "run",
				},
			},
		},
	}

	return env
}

func InstanceChange(
	address string,
	sequence uint32,
	kind xdr.LedgerEntryChangeType,
	executable xdr.ContractExecutable,
) xdr.LedgerEntryChange {
	entry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: xdr.Uint32(sequence),
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeContractData,
			ContractData: &xdr.ContractDataEntry{
				Contract:   ContractAddress(address),
				Durability: xdr.ContractDataDurabilityPersistent,
				Key: xdr.ScVal{
					Type: xdr.ScValTypeScvLedgerKeyContractInstance,
				},
				Val: xdr.ScVal{
					Type: xdr.ScValTypeScvContractInstance,
					Instance: &xdr.ScContractInstance{
						Executable: executable,
					},
				},
			},
		},
	}
	change := xdr.LedgerEntryChange{Type: kind}
	switch kind {
	case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
		change.Created = &entry
	case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
		change.Updated = &entry
	case xdr.LedgerEntryChangeTypeLedgerEntryState:
		change.State = &entry
	case xdr.LedgerEntryChangeTypeLedgerEntryRestored:
		change.Restored = &entry
	}

	return change
}

func WasmExecutable(value byte) xdr.ContractExecutable {
	hash := xdr.Hash{value}

	return xdr.ContractExecutable{
		Type:     xdr.ContractExecutableTypeContractExecutableWasm,
		WasmHash: &hash,
	}
}

func ContractBundle(t *testing.T, sequence uint32, changes ...xdr.LedgerEntryChange) domain.Bundle {
	t.Helper()
	env := ContractEnvelope()
	env.V1.Tx.SeqNum = xdr.SequenceNumber(sequence)
	meta := xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			Operations: []xdr.OperationMetaV2{
				{Changes: changes},
			},
		},
	}
	ledger := Ledger(sequence)
	transaction, err := decoder.Transaction(env, Result(env, true), meta, ledger, 1)
	if err != nil {
		t.Fatal(err)
	}

	ledger.TransactionCount = 1

	return domain.Bundle{
		Ledger:       ledger,
		Transactions: []domain.Transaction{transaction},
	}
}
