package decoder_test

import (
	"encoding/hex"
	"testing"

	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestSoroswapMultipleHopsShareIntermediateTransferEvidence(t *testing.T) {
	envelope, result, meta := testutil.SoroswapSource(t, false)
	assets, err := xdr.BuildAssets("EUR:" + testutil.Address(4))
	if err != nil {
		t.Fatal(err)
	}

	// Independent Python vector for USD:Address(3) <-> EUR:Address(4).
	secondPool := "CAHUAL4UZ7YIYDRYDS4PEA6IP6N24WCCLIH4PU2FQYWQ4EGUH3ROP6GM"
	firstPath := soroswapCall(&envelope).Args[2].MustVec()
	path := testutil.Vec((*firstPath)[0], (*firstPath)[1],
		testutil.AddressValue(testutil.SACContract(assets[0])))
	soroswapCall(&envelope).Args[2] = path
	soroswapCall(&envelope).Args[1] = testutil.I128(0, 4000000)
	meta.V4.Operations[0].Events[1].Body.V0.Topics[2] = testutil.AddressValue(secondPool)
	router := meta.V4.Operations[0].Events[2]
	router.Body.V0.Data = testutil.Map(xdr.ScMap{
		{Key: testutil.Symbol("amounts"), Val: testutil.Vec(testutil.I128(0, 20000000),
			testutil.I128(0, 10000000), testutil.I128(0, 5000000))},
		{Key: testutil.Symbol("path"), Val: path},
		{Key: testutil.Symbol("to"), Val: testutil.AddressValue(testutil.Address(2))},
	})
	meta.V4.Operations[0].Events = append(meta.V4.Operations[0].Events[:2],
		testutil.TokenEvent(testutil.SACContract(assets[0]), "transfer",
			[]string{secondPool, testutil.Address(2)}, testutil.I128(0, 5000000),
			testutil.String(assets[0].StringCanonical())), router)
	meta.V4.SorobanMeta.ReturnValue = valuePointer(testutil.Vec(testutil.I128(0, 20000000),
		testutil.I128(0, 10000000), testutil.I128(0, 5000000)))
	testutil.SetSoroswapCommitment(t, &result, meta)
	transaction := decodeSoroswap(t, envelope, result, meta)
	swap := transaction.Interpretation.Actions[0].ContractSwap
	if swap == nil || len(swap.Hops) != 2 || len(swap.TransferEventIndexes) != 3 ||
		swap.Hops[0].OutputEventIndex != swap.Hops[1].InputEventIndex ||
		swap.Output.Amount != "5000000" || swap.RouterEventIndex != 3 {
		t.Fatal("intermediate transfer was duplicated or route failed to reconcile", swap)
	}
}

func TestSoroswapExactI128AndObservedCodeEvidence(t *testing.T) {
	envelope, result, meta := testutil.SoroswapSource(t, false)
	quantity := testutil.I128(9223372036854775807, 18446744073709551615)
	soroswapCall(&envelope).Args[0] = quantity
	soroswapCall(&envelope).Args[1] = quantity
	for index := 0; index < 2; index++ {
		meta.V4.Operations[0].Events[index].Body.V0.Data = quantity
	}

	entries := meta.V4.Operations[0].Events[2].Body.V0.Data.MustMap()
	(*entries)[0].Val = testutil.Vec(quantity, quantity)
	meta.V4.SorobanMeta.ReturnValue = valuePointer(testutil.Vec(quantity, quantity))
	encodedHash := "4b95bbf9caec2c6e00c786f53c5f392c2fcdb8435ac0a862ab5e0645eb65824c"
	raw, err := hex.DecodeString(encodedHash)
	if err != nil {
		t.Fatal(err)
	}

	var hash xdr.Hash
	copy(hash[:], raw)
	meta.V4.TxChangesBefore = xdr.LedgerEntryChanges{
		testutil.InstanceChange(testutil.SoroswapRouter, 10,
			xdr.LedgerEntryChangeTypeLedgerEntryState, xdr.ContractExecutable{
				Type:     xdr.ContractExecutableTypeContractExecutableWasm,
				WasmHash: &hash,
			}),
	}
	testutil.SetSoroswapCommitment(t, &result, meta)
	transaction := decodeSoroswap(t, envelope, result, meta)
	swap := transaction.Interpretation.Actions[0].ContractSwap
	if swap == nil || swap.Input.Amount != "170141183460469231731687303715884105727" ||
		swap.Output.Amount != swap.Input.Amount || swap.RouterWasmHash == nil ||
		*swap.RouterWasmHash != encodedHash {
		t.Fatal("i128 precision or observed source code hash was lost", swap)
	}
}

func TestSoroswapCommitmentIncludesSystemEventsAndExcludesFeesAndDiagnostics(t *testing.T) {
	envelope, result, meta := testutil.SoroswapSource(t, false)
	router := meta.V4.Operations[0].Events[2]
	system := xdr.ContractEvent{
		Type: xdr.ContractEventTypeSystem,
		Body: xdr.ContractEventBody{
			V: 0,
			V0: &xdr.ContractEventV0{
				Data: xdr.ScVal{Type: xdr.ScValTypeScvVoid},
			},
		},
	}
	meta.V4.Operations[0].Events = append(meta.V4.Operations[0].Events[:2], system, router)
	meta.V4.Events = []xdr.TransactionEvent{
		testutil.FeeEvent(testutil.Address(1), 100,
			xdr.TransactionEventStageTransactionEventStageBeforeAllTxs),
	}
	meta.V4.DiagnosticEvents = []xdr.DiagnosticEvent{
		{InSuccessfulContractCall: true, Event: router},
	}
	testutil.SetSoroswapCommitment(t, &result, meta)
	transaction := decodeSoroswap(t, envelope, result, meta)
	swap := transaction.Interpretation.Actions[0].ContractSwap
	if swap == nil || swap.RouterEventIndex != 3 || transaction.Fees.Status != "complete" ||
		len(transaction.TokenTransfers) != 2 || len(transaction.Events) != 5 {
		t.Fatal("system/diagnostic/fee event provenance was mixed", swap)
	}

	// Hashing a convenient subset must not validate rewritten source metadata.
	reduced := meta
	reducedV4 := *meta.V4
	reduced.V4 = &reducedV4
	reduced.V4.Operations = []xdr.OperationMetaV2{
		{Events: []xdr.ContractEvent{meta.V4.Operations[0].Events[0],
			meta.V4.Operations[0].Events[1], router}},
	}
	testutil.SetSoroswapCommitment(t, &result, reduced)
	transaction = decodeSoroswap(t, envelope, result, meta)
	if transaction.Interpretation.Actions[0].ContractSwap != nil {
		t.Fatal("result hash was matched by dropping an inconvenient source event")
	}
}

func TestSoroswapFeeBumpSeparatesFeePayerOperationSourceAndTrader(t *testing.T) {
	inner, innerResult, meta := testutil.SoroswapSource(t, false)
	override := xdr.MustMuxedAddress(testutil.Address(6))
	inner.V1.Tx.Operations[0].SourceAccount = &override
	envelope := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTxFeeBump,
		FeeBump: &xdr.FeeBumpTransactionEnvelope{
			Tx: xdr.FeeBumpTransaction{
				FeeSource: xdr.MustMuxedAddress(testutil.Address(9)),
				Fee:       1000,
				InnerTx: xdr.FeeBumpTransactionInnerTx{
					Type: xdr.EnvelopeTypeEnvelopeTypeTx,
					V1:   inner.V1,
				},
			},
		},
	}
	result := testutil.Result(envelope, true)
	result.Result.Result = xdr.TransactionResultResult{
		Code: xdr.TransactionResultCodeTxFeeBumpInnerSuccess,
		InnerResultPair: &xdr.InnerTransactionResultPair{
			Result: xdr.InnerTransactionResult{
				Result: xdr.InnerTransactionResultResult{
					Code:    xdr.TransactionResultCodeTxSuccess,
					Results: innerResult.Result.Result.Results,
				},
			},
		},
	}
	meta.V4.Events = []xdr.TransactionEvent{
		testutil.FeeEvent(testutil.Address(9), 100,
			xdr.TransactionEventStageTransactionEventStageBeforeAllTxs),
	}
	transaction := decodeSoroswap(t, envelope, result, meta)
	action := transaction.Interpretation.Actions[0]
	if action.ContractSwap == nil || action.From != testutil.Address(2) ||
		action.SourceAccount != testutil.Address(
			6,
		) || transaction.FeeAccount != testutil.Address(9) ||
		transaction.SourceAccount != testutil.Address(1) || transaction.Fees.Status != "complete" {
		t.Fatal("fee payer, operation source and actual token owner were conflated", action)
	}
}
