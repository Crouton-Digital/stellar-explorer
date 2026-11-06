package testutil

import (
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func AquariusStrictReceiveSource(
	t *testing.T,
	hops int,
	refund bool,
) (xdr.TransactionEnvelope, xdr.TransactionResultPair, xdr.TransactionMeta) {
	t.Helper()
	if hops < 1 || hops > 3 {
		t.Fatal("strict-receive source helper supports one to three hops")
	}

	tokens := []string{SACContract(xdr.MustNewNativeAsset())}
	amounts := []uint64{20000000}
	for index := 0; index < hops; index++ {
		tokens = append(tokens, Contract(byte(10+index)))
		amounts = append(amounts, amounts[len(amounts)-1]/2)
	}

	maximum := amounts[0]
	if refund {
		maximum += 1000000
	}

	trader := Contract(2)
	envelope := InvocationEnvelope("swap_chained_strict_receive")
	call := envelope.V1.Tx.Operations[0].Body.InvokeHostFunctionOp.HostFunction.InvokeContract
	call.ContractAddress = ContractAddress(AquariusRouter)
	chain := []xdr.ScVal{}
	events := []xdr.ContractEvent{
		TokenEvent(tokens[0], "transfer", []string{trader, AquariusRouter},
			I128(0, maximum), String("native")),
	}
	for index := 0; index < hops; index++ {
		pool := Contract(byte(20 + index))
		step, emitted := aquariusSettlementHop(tokens[index], tokens[index+1], pool,
			trader, byte(index+1), amounts[index], amounts[index+1])
		chain = append(chain, step)
		events = append(events, emitted...)
	}

	output := amounts[len(amounts)-1]
	call.Args = []xdr.ScVal{
		AddressValue(trader),
		Vec(chain...),
		AddressValue(tokens[0]),
		U128(0, output),
		U128(0, maximum),
	}
	events = append(events, TokenEvent(tokens[len(tokens)-1], "transfer",
		[]string{AquariusRouter, trader}, I128(0, output)))
	if refund {
		events = append(events, TokenEvent(tokens[0], "transfer",
			[]string{AquariusRouter, trader}, I128(0, maximum-amounts[0]), String("native")))
	}

	returned := U128(0, amounts[0])
	meta := aquariusSettlementMeta(events, returned)
	result := Result(envelope, true)
	AquariusCommitment(t, &result, meta)

	return envelope, result, meta
}

func AquariusSplitSource(
	t *testing.T,
) (xdr.TransactionEnvelope, xdr.TransactionResultPair, xdr.TransactionMeta) {
	t.Helper()
	input, output, intermediate := SACContract(xdr.MustNewNativeAsset()), Contract(10), Contract(11)
	trader := Contract(2)
	envelope := InvocationEnvelope("swap_split")
	call := envelope.V1.Tx.Operations[0].Body.InvokeHostFunctionOp.HostFunction.InvokeContract
	call.ContractAddress = ContractAddress(AquariusRouter)
	events := []xdr.ContractEvent{
		TokenEvent(input, "transfer", []string{trader, AquariusRouter},
			I128(0, 100000000), String("native")),
	}
	first, emitted := aquariusSettlementHop(input, output, Contract(20), trader, 1,
		60000000, 30000000)
	events = append(events, emitted...)
	second, emitted := aquariusSettlementHop(input, intermediate, Contract(21), trader, 2,
		40000000, 20000000)
	events = append(events, emitted...)
	third, emitted := aquariusSettlementHop(intermediate, output, Contract(22), trader, 3,
		20000000, 10000000)
	events = append(events, emitted...)
	firstWeight, secondWeight := xdr.Uint32(6000), xdr.Uint32(4000)
	group := Vec(
		Vec(Vec(first), xdr.ScVal{Type: xdr.ScValTypeScvU32, U32: &firstWeight}),
		Vec(Vec(second, third), xdr.ScVal{Type: xdr.ScValTypeScvU32, U32: &secondWeight}),
	)
	call.Args = []xdr.ScVal{
		AddressValue(trader),
		Vec(group),
		AddressValue(input),
		AddressValue(output),
		U128(0, 100000000),
		U128(0, 39999999),
	}
	events = append(events, TokenEvent(output, "transfer", []string{AquariusRouter, trader},
		I128(0, 40000000)))
	meta := aquariusSettlementMeta(events, U128(0, 40000000))
	result := Result(envelope, true)
	AquariusCommitment(t, &result, meta)

	return envelope, result, meta
}

func aquariusSettlementHop(
	input, output, pool, trader string,
	index byte,
	inAmount, outAmount uint64,
) (xdr.ScVal, []xdr.ContractEvent) {
	tokens := AquariusPoolTokens(input, output)
	step := Vec(tokens, AquariusPoolIndex(index), AddressValue(output))
	events := []xdr.ContractEvent{}
	for _, movement := range []struct {
		asset  string
		from   string
		to     string
		amount uint64
	}{
		{
			asset:  input,
			from:   AquariusRouter,
			to:     pool,
			amount: inAmount,
		},
		{
			asset:  output,
			from:   pool,
			to:     AquariusRouter,
			amount: outAmount,
		},
	} {
		extra := []xdr.ScVal{}
		if movement.asset == SACContract(xdr.MustNewNativeAsset()) {
			extra = append(extra, String("native"))
		}

		events = append(events, TokenEvent(movement.asset, "transfer",
			[]string{movement.from, movement.to}, I128(0, movement.amount), extra...))
	}

	events = append(events,
		AquariusEvent(pool, []xdr.ScVal{
			Symbol("trade"),
			AddressValue(input),
			AddressValue(output),
			AddressValue(AquariusRouter),
		}, Vec(I128(0, inAmount), I128(0, outAmount), I128(0, 100))),
		AquariusEvent(AquariusRouter, []xdr.ScVal{
			Symbol("swap"),
			tokens,
			AddressValue(trader),
		}, Vec(AddressValue(pool), AddressValue(input), AddressValue(output),
			U128(0, inAmount), U128(0, outAmount))))

	return step, events
}

func aquariusSettlementMeta(events []xdr.ContractEvent, returned xdr.ScVal) xdr.TransactionMeta {
	return xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			Operations:  []xdr.OperationMetaV2{{Events: events}},
			SorobanMeta: &xdr.SorobanTransactionMetaV2{ReturnValue: &returned},
		},
	}
}

func AquariusStrictReceiveBundle(t *testing.T, sequence uint32) domain.Bundle {
	t.Helper()
	envelope, result, meta := AquariusStrictReceiveSource(t, 3, true)

	return aquariusSettlementBundle(t, sequence, envelope, result, meta)
}

func AquariusSplitBundle(t *testing.T, sequence uint32) domain.Bundle {
	t.Helper()
	envelope, result, meta := AquariusSplitSource(t)

	return aquariusSettlementBundle(t, sequence, envelope, result, meta)
}

func aquariusSettlementBundle(
	t *testing.T,
	sequence uint32,
	envelope xdr.TransactionEnvelope,
	result xdr.TransactionResultPair,
	meta xdr.TransactionMeta,
) domain.Bundle {
	t.Helper()
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
