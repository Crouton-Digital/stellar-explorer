package testutil

import (
	"bytes"
	"sort"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const AquariusRouter = "CBCFTQSPDBAIZ6R6PJQKSQWKNKWH2QIV3I4J72SHWBIK3ADRRAM5A6GD"

func U128(hi, lo uint64) xdr.ScVal {
	return xdr.ScVal{
		Type: xdr.ScValTypeScvU128,
		U128: &xdr.UInt128Parts{
			Hi: xdr.Uint64(hi),
			Lo: xdr.Uint64(lo),
		},
	}
}

func AquariusPoolIndex(seed byte) xdr.ScVal {
	index := xdr.ScBytes(make([]byte, 32))
	index[0] = seed

	return xdr.ScVal{
		Type:  xdr.ScValTypeScvBytes,
		Bytes: &index,
	}
}

func AquariusPoolTokens(tokens ...string) xdr.ScVal {
	ordered := append([]string{}, tokens...)
	sort.Slice(ordered, func(first, second int) bool {
		left := strkey.MustDecode(strkey.VersionByteContract, ordered[first])
		right := strkey.MustDecode(strkey.VersionByteContract, ordered[second])

		return bytes.Compare(left, right) < 0
	})

	values := []xdr.ScVal{}
	for _, address := range ordered {
		values = append(values, AddressValue(address))
	}

	return Vec(values...)
}

func AquariusEvent(
	contract string,
	topics []xdr.ScVal,
	data xdr.ScVal,
) xdr.ContractEvent {
	id := *ContractAddress(contract).ContractId

	return xdr.ContractEvent{
		ContractId: &id,
		Type:       xdr.ContractEventTypeContract,
		Body: xdr.ContractEventBody{
			V: 0,
			V0: &xdr.ContractEventV0{
				Topics: topics,
				Data:   data,
			},
		},
	}
}

func AquariusSource(
	t *testing.T,
	chained bool,
) (xdr.TransactionEnvelope, xdr.TransactionResultPair, xdr.TransactionMeta) {
	t.Helper()
	assets, err := xdr.BuildAssets("USD:" + Address(3))
	if err != nil {
		t.Fatal(err)
	}

	native, usd := xdr.MustNewNativeAsset(), assets[0]
	tokens := []string{SACContract(native), SACContract(usd)}
	amounts := []uint64{20000000, 10000000}
	trader := Contract(2)
	settlement := trader
	if chained {
		tokens = append(tokens, Contract(10))
		amounts = append(amounts, 5000000)
		settlement = AquariusRouter
	}

	envelope := InvocationEnvelope("swap")
	call := envelope.V1.Tx.Operations[0].Body.InvokeHostFunctionOp.HostFunction.InvokeContract
	call.ContractAddress = ContractAddress(AquariusRouter)
	call.Args = []xdr.ScVal{
		AddressValue(trader),
		AquariusPoolTokens(tokens[0], tokens[1]),
		AddressValue(tokens[0]),
		AddressValue(tokens[1]),
		AquariusPoolIndex(1),
		U128(0, amounts[0]),
		U128(0, amounts[1]-1),
	}
	chain := []xdr.ScVal{}
	events := []xdr.ContractEvent{}
	if chained {
		call.FunctionName = "swap_chained"
		events = append(events, TokenEvent(tokens[0], "transfer",
			[]string{trader, AquariusRouter}, I128(0, amounts[0]),
			String(native.StringCanonical())))
	}

	for index := 0; index < len(tokens)-1; index++ {
		// These synthetic addresses are event evidence, not deterministic pool pins.
		pool := Contract(byte(20 + index))
		poolTokens := AquariusPoolTokens(tokens[index], tokens[index+1])
		chain = append(chain, Vec(poolTokens, AquariusPoolIndex(byte(index+1)),
			AddressValue(tokens[index+1])))
		for _, movement := range []struct {
			asset  string
			from   string
			to     string
			amount uint64
		}{
			{
				asset:  tokens[index],
				from:   settlement,
				to:     pool,
				amount: amounts[index],
			},
			{
				asset:  tokens[index+1],
				from:   pool,
				to:     settlement,
				amount: amounts[index+1],
			},
		} {
			extra := []xdr.ScVal{}
			if movement.asset == SACContract(native) {
				extra = append(extra, String(native.StringCanonical()))
			}

			if movement.asset == SACContract(usd) {
				extra = append(extra, String(usd.StringCanonical()))
			}

			events = append(events, TokenEvent(movement.asset, "transfer",
				[]string{movement.from, movement.to}, I128(0, movement.amount), extra...))
		}

		events = append(events,
			AquariusEvent(pool, []xdr.ScVal{
				Symbol("trade"),
				AddressValue(tokens[index]),
				AddressValue(tokens[index+1]),
				AddressValue(settlement),
			}, Vec(I128(0, amounts[index]), I128(0, amounts[index+1]), I128(0, 100))),
			AquariusEvent(AquariusRouter, []xdr.ScVal{
				Symbol("swap"),
				poolTokens,
				AddressValue(trader),
			}, Vec(AddressValue(pool), AddressValue(tokens[index]),
				AddressValue(tokens[index+1]), U128(0, amounts[index]),
				U128(0, amounts[index+1]))))
	}

	if chained {
		call.Args = []xdr.ScVal{
			AddressValue(trader),
			Vec(chain...),
			AddressValue(tokens[0]),
			U128(0, amounts[0]),
			U128(0, amounts[len(amounts)-1]-1),
		}
		events = append(events, TokenEvent(tokens[len(tokens)-1], "transfer",
			[]string{AquariusRouter, trader}, I128(0, amounts[len(amounts)-1])))
	}

	returned := U128(0, amounts[len(amounts)-1])
	meta := xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			Operations:  []xdr.OperationMetaV2{{Events: events}},
			SorobanMeta: &xdr.SorobanTransactionMetaV2{ReturnValue: &returned},
		},
	}
	result := Result(envelope, true)
	AquariusCommitment(t, &result, meta)

	return envelope, result, meta
}

func AquariusCommitment(
	t *testing.T,
	result *xdr.TransactionResultPair,
	meta xdr.TransactionMeta,
) {
	t.Helper()
	SetSoroswapCommitment(t, result, meta)
}

func AquariusBundle(t *testing.T, sequence uint32) domain.Bundle {
	t.Helper()

	return aquariusBundle(t, sequence, false)
}

func AquariusChainedBundle(t *testing.T, sequence uint32) domain.Bundle {
	t.Helper()

	return aquariusBundle(t, sequence, true)
}

func aquariusBundle(t *testing.T, sequence uint32, chained bool) domain.Bundle {
	t.Helper()
	envelope, result, meta := AquariusSource(t, chained)
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
