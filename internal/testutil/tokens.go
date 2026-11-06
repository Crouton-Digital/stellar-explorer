package testutil

import (
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func Symbol(value string) xdr.ScVal {
	symbol := xdr.ScSymbol(value)

	return xdr.ScVal{
		Type: xdr.ScValTypeScvSymbol,
		Sym:  &symbol,
	}
}

func String(value string) xdr.ScVal {
	text := xdr.ScString(value)

	return xdr.ScVal{
		Type: xdr.ScValTypeScvString,
		Str:  &text,
	}
}

func I128(hi int64, lo uint64) xdr.ScVal {
	return xdr.ScVal{
		Type: xdr.ScValTypeScvI128,
		I128: &xdr.Int128Parts{
			Hi: xdr.Int64(hi),
			Lo: xdr.Uint64(lo),
		},
	}
}

func Map(entries xdr.ScMap) xdr.ScVal {
	value := &entries

	return xdr.ScVal{
		Type: xdr.ScValTypeScvMap,
		Map:  &value,
	}
}

func SACContract(asset xdr.Asset) string {
	id, e := asset.ContractID(domain.Passphrase)
	if e != nil {
		panic(e)
	}

	return strkey.MustEncode(strkey.VersionByteContract, id[:])
}

func TokenEvent(
	contract, kind string,
	addresses []string,
	value xdr.ScVal,
	extra ...xdr.ScVal,
) xdr.ContractEvent {
	raw := strkey.MustDecode(strkey.VersionByteContract, contract)
	var id xdr.ContractId
	copy(id[:], raw)
	topics := []xdr.ScVal{Symbol(kind)}
	for _, address := range addresses {
		var value xdr.ScAddress
		if strkey.IsValidContractAddress(address) {
			raw := strkey.MustDecode(strkey.VersionByteContract, address)
			var contract xdr.ContractId
			copy(contract[:], raw)
			value.Type = xdr.ScAddressTypeScAddressTypeContract
			value.ContractId = &contract
		} else {
			account := xdr.MustAddress(address)
			value.Type = xdr.ScAddressTypeScAddressTypeAccount
			value.AccountId = &account
		}

		topics = append(topics, xdr.ScVal{
			Type:    xdr.ScValTypeScvAddress,
			Address: &value,
		})
	}

	topics = append(topics, extra...)

	return xdr.ContractEvent{
		ContractId: &id,
		Type:       xdr.ContractEventTypeContract,
		Body: xdr.ContractEventBody{
			V: 0,
			V0: &xdr.ContractEventV0{
				Topics: topics,
				Data:   value,
			},
		},
	}
}

func TokenBundle(t *testing.T, sequence uint32, events ...xdr.ContractEvent) domain.Bundle {
	t.Helper()
	env := PaymentEnvelope()
	env.V1.Tx.SeqNum = xdr.SequenceNumber(sequence)
	meta := xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			Operations: []xdr.OperationMetaV2{
				{Events: events},
			},
		},
	}
	ledger := Ledger(sequence)
	transaction, e := decoder.Transaction(env, Result(env, true), meta, ledger, 1)
	if e != nil {
		t.Fatal(e)
	}

	ledger.TransactionCount = 1

	return domain.Bundle{
		Ledger:       ledger,
		Transactions: []domain.Transaction{transaction},
	}
}
