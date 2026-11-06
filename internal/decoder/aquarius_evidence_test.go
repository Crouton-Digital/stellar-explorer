package decoder_test

import (
	"encoding/hex"
	"testing"

	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestAquariusExactU128AmountsAndObservedExecutable(t *testing.T) {
	envelope, result, meta := testutil.AquariusSource(t, false)
	unsigned := testutil.U128(9223372036854775807, 18446744073709551615)
	signed := testutil.I128(9223372036854775807, 18446744073709551615)
	aquariusCall(&envelope).Args[5] = unsigned
	aquariusCall(&envelope).Args[6] = unsigned
	events := meta.V4.Operations[0].Events
	events[0].Body.V0.Data = signed
	events[1].Body.V0.Data = signed
	trade := events[2].Body.V0.Data.MustVec()
	(*trade)[0], (*trade)[1] = signed, signed
	router := events[3].Body.V0.Data.MustVec()
	(*router)[3], (*router)[4] = unsigned, unsigned
	meta.V4.SorobanMeta.ReturnValue = valuePointer(unsigned)
	executable := testutil.WasmExecutable(7)
	meta.V4.TxChangesBefore = xdr.LedgerEntryChanges{
		testutil.InstanceChange(testutil.AquariusRouter, 10,
			xdr.LedgerEntryChangeTypeLedgerEntryState, executable),
	}
	testutil.AquariusCommitment(t, &result, meta)
	transaction := decodeAquarius(t, envelope, result, meta)
	swap := transaction.Interpretation.Actions[0].AquariusSwap
	if swap == nil || swap.Input.Amount != "170141183460469231731687303715884105727" ||
		swap.Output.Amount != swap.Input.Amount || swap.InputLimit != swap.Input.Amount ||
		swap.OutputLimit != swap.Output.Amount || swap.RouterWasmHash == nil ||
		*swap.RouterWasmHash != hex.EncodeToString(executable.WasmHash[:]) {
		t.Fatal("unsigned request/return precision or observed code evidence was lost", swap)
	}

	// An unsigned limit above signed token capacity must not wrap to a smaller minimum.
	aquariusCall(&envelope).Args[6] = testutil.U128(18446744073709551615, 18446744073709551615)
	transaction = decodeAquarius(t, envelope, result, meta)
	if transaction.Interpretation.Actions[0].AquariusSwap != nil {
		t.Fatal("u128 maximum output limit was truncated to fit signed token amounts")
	}
}

func TestAquariusCommitmentIncludesSystemEventsAndKeepsFeeProvenanceSeparate(t *testing.T) {
	envelope, result, meta := testutil.AquariusSource(t, false)
	events := meta.V4.Operations[0].Events
	system := xdr.ContractEvent{
		Type: xdr.ContractEventTypeSystem,
		Body: xdr.ContractEventBody{
			V: 0,
			V0: &xdr.ContractEventV0{
				Data: xdr.ScVal{Type: xdr.ScValTypeScvVoid},
			},
		},
	}
	meta.V4.Operations[0].Events = []xdr.ContractEvent{
		events[0],
		events[1],
		system,
		events[2],
		events[3],
	}
	meta.V4.Events = []xdr.TransactionEvent{
		testutil.FeeEvent(testutil.Address(1), 100,
			xdr.TransactionEventStageTransactionEventStageBeforeAllTxs),
	}
	meta.V4.DiagnosticEvents = []xdr.DiagnosticEvent{
		{InSuccessfulContractCall: true, Event: events[3]},
	}
	testutil.AquariusCommitment(t, &result, meta)
	transaction := decodeAquarius(t, envelope, result, meta)
	swap := transaction.Interpretation.Actions[0].AquariusSwap
	if swap == nil || swap.Hops[0].TradeEventIndex != 3 ||
		swap.Hops[0].RouterEventIndex != 4 || transaction.Fees.Status != "complete" ||
		len(transaction.TokenTransfers) != 2 || len(transaction.Events) != 6 {
		t.Fatal("system, diagnostic or fee events changed route provenance", swap)
	}

	// A hash over a selected subset does not authenticate the complete source metadata.
	withoutSystem := meta
	copyV4 := *meta.V4
	withoutSystem.V4 = &copyV4
	withoutSystem.V4.Operations = []xdr.OperationMetaV2{{Events: events}}
	testutil.AquariusCommitment(t, &result, withoutSystem)
	transaction = decodeAquarius(t, envelope, result, meta)
	if transaction.Interpretation.Actions[0].AquariusSwap != nil {
		t.Fatal("dropping an inconvenient system event validated a different result hash")
	}
}

func TestAquariusZeroOutputMinimumIsAnExactUnsignedLimit(t *testing.T) {
	envelope, result, meta := testutil.AquariusSource(t, false)
	aquariusCall(&envelope).Args[6] = testutil.U128(0, 0)
	transaction := decodeAquarius(t, envelope, result, meta)
	swap := transaction.Interpretation.Actions[0].AquariusSwap
	if swap == nil || swap.OutputLimit != "0" || swap.Output.Amount != "10000000" {
		t.Fatal("zero slippage minimum was confused with an absent or invalid limit", swap)
	}
}
