package decoder_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestCAP67TransactionEventsSurviveFailureWithoutCreatingMovements(t *testing.T) {
	for _, successful := range []bool{true, false} {
		t.Run(map[bool]string{true: "successful", false: "failed"}[successful], func(t *testing.T) {
			bundle := testutil.FeeBundle(t, 10, successful)
			transaction := bundle.Transactions[0]
			if transaction.Fees.Status != "complete" || *transaction.Fees.Charged != "1000" ||
				*transaction.Fees.Refunded != "900" || *transaction.Fees.Net != "100" {
				t.Fatalf("incorrect fee summary: %+v", transaction.Fees)
			}

			operations := 0
			if successful {
				operations = 1
			}

			if len(transaction.Events) != operations+2 ||
				len(transaction.TokenTransfers) != operations {
				t.Fatal("fees were omitted or counted as token movements")
			}

			for _, event := range transaction.Events[operations:] {
				if event.Scope != "transaction" || event.OpIndex != nil || event.Stage == nil {
					t.Fatalf("incorrect transaction event provenance: %+v", event)
				}
			}

			source, _ := json.Marshal(transaction)
			if err := decoder.RebuildTransactionEvents(&transaction, bundle.Ledger.ProtocolVersion); err != nil {
				t.Fatal(err)
			}

			rebuilt, _ := json.Marshal(transaction)
			if string(source) != string(rebuilt) {
				t.Fatal("rebuilding events changed source fields or event identities")
			}
		})
	}
}

func TestCAP67FeeValidationAndMissingData(t *testing.T) {
	before := xdr.TransactionEventStageTransactionEventStageBeforeAllTxs
	after := xdr.TransactionEventStageTransactionEventStageAfterAllTxs
	charge := testutil.FeeEvent(testutil.Address(1), 100, before)
	cases := []struct {
		name   string
		events []xdr.TransactionEvent
		status string
		reason string
	}{
		{
			name:   "no events remain unknown",
			status: "unavailable",
			reason: "fee_events_not_provided",
		},
		{
			name:   "charge without refund",
			events: []xdr.TransactionEvent{charge},
			status: "complete",
		},
		{
			name: "refund without charge",
			events: []xdr.TransactionEvent{
				testutil.FeeEvent(testutil.Address(1), -100, after),
			},
			status: "invalid",
			reason: "incomplete_or_duplicate_fee_events",
		},
		{
			name:   "duplicate charge",
			events: []xdr.TransactionEvent{charge, charge},
			status: "invalid",
			reason: "incomplete_or_duplicate_fee_events",
		},
		{
			name: "wrong payer",
			events: []xdr.TransactionEvent{
				testutil.FeeEvent(testutil.Address(2), 100, before),
			},
			status: "invalid",
			reason: "invalid_fee_account_or_amount",
		},
		{
			name: "incorrect charge stage",
			events: []xdr.TransactionEvent{
				testutil.FeeEvent(testutil.Address(1), 100, after),
			},
			status: "invalid",
			reason: "invalid_fee_stage",
		},
		{
			name: "incorrect refund stage",
			events: []xdr.TransactionEvent{
				charge,
				testutil.FeeEvent(testutil.Address(1), -1, before),
			},
			status: "invalid",
			reason: "invalid_refund_stage",
		},
		{
			name: "refund exceeds charge",
			events: []xdr.TransactionEvent{
				charge,
				testutil.FeeEvent(testutil.Address(1), -101, after),
			},
			status: "invalid",
			reason: "incomplete_or_duplicate_fee_events",
		},
		{
			name: "result does not reconcile",
			events: []xdr.TransactionEvent{
				testutil.FeeEvent(testutil.Address(1), 101, before),
			},
			status: "invalid",
			reason: "fee_events_do_not_match_result",
		},
	}

	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) {
			meta := xdr.TransactionMeta{
				V: 4,
				V4: &xdr.TransactionMetaV4{
					Events: scenario.events,
				},
			}
			env := testutil.PaymentEnvelope()
			transaction, err := decoder.Transaction(env, testutil.Result(env, true),
				meta, testutil.Ledger(10), 1)
			if err != nil || transaction.Fees.Status != scenario.status {
				t.Fatalf("unexpected fee result: %+v, %v", transaction.Fees, err)
			}

			if scenario.reason != "" && (transaction.Fees.Reason == nil ||
				*transaction.Fees.Reason != scenario.reason) {
				t.Fatalf("missing fee reason: %+v", transaction.Fees)
			}

			if scenario.status != "complete" && (transaction.Fees.Charged != nil ||
				transaction.Fees.Refunded != nil || transaction.Fees.Net != nil) {
				t.Fatal("unverified fees were presented as known totals")
			}

			if len(transaction.Events) != len(scenario.events) {
				t.Fatal("unrecognized fee data lost its raw event")
			}
		})
	}
}

func TestCAP67RealFixtureFeeReconciliation(t *testing.T) {
	bundle := testutil.RealBundle(t)
	for _, transaction := range bundle.Transactions {
		if transaction.Fees.Status != "complete" || transaction.Fees.Net == nil ||
			*transaction.Fees.Net != transaction.FeeCharged {
			t.Fatalf("real transaction %s failed reconciliation: %+v",
				transaction.Hash, transaction.Fees)
		}

		for _, event := range transaction.Events {
			var raw xdr.ContractEvent
			if err := xdr.SafeUnmarshalBase64(event.RawXDR, &raw); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestLegacyMetadataDoesNotInventFeeBreakdown(t *testing.T) {
	for _, meta := range []xdr.TransactionMeta{
		{V: 2, V2: &xdr.TransactionMetaV2{}},
		{V: 3, V3: &xdr.TransactionMetaV3{}},
	} {
		env := testutil.PaymentEnvelope()
		transaction, err := decoder.Transaction(env, testutil.Result(env, true),
			meta, testutil.Ledger(10), 1)
		if err != nil || transaction.Fees.Status != "unavailable" ||
			transaction.Fees.Net != nil || transaction.Fees.Refunded != nil {
			t.Fatal("legacy metadata invented fee/refund totals", err)
		}
	}
}

func TestCAP67FeeBumpUsesOuterPayer(t *testing.T) {
	inner := testutil.PaymentEnvelope()
	env := xdr.TransactionEnvelope{
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
	result := testutil.Result(env, true)
	results := []xdr.OperationResult{}
	result.Result.Result = xdr.TransactionResultResult{
		Code: xdr.TransactionResultCodeTxFeeBumpInnerSuccess,
		InnerResultPair: &xdr.InnerTransactionResultPair{
			Result: xdr.InnerTransactionResult{
				Result: xdr.InnerTransactionResultResult{
					Code:    xdr.TransactionResultCodeTxSuccess,
					Results: &results,
				},
			},
		},
	}
	meta := xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			Events: []xdr.TransactionEvent{
				testutil.FeeEvent(testutil.Address(9), 100,
					xdr.TransactionEventStageTransactionEventStageBeforeAllTxs),
			},
		},
	}
	transaction, err := decoder.Transaction(env, result, meta, testutil.Ledger(10), 1)
	if err != nil || transaction.Fees.Status != "complete" ||
		transaction.FeeAccount != testutil.Address(9) ||
		transaction.Fees.Events[0].Account != testutil.Address(9) {
		t.Fatal("fee-bump fee attributed to inner source", err)
	}
}

func TestTransactionEventRebuildPreservesOperationAndMovementIDs(t *testing.T) {
	transaction := testutil.FeeBundle(t, 10, true).Transactions[0]
	operation := transaction.Events[0]
	movements := append([]domain.TokenTransfer{}, transaction.TokenTransfers...)
	if err := decoder.RebuildTransactionEvents(&transaction, 23); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(operation, transaction.Events[0]) ||
		!reflect.DeepEqual(movements, transaction.TokenTransfers) {
		t.Fatal("operation event or movement identity changed")
	}
}

func TestCAP67Signed128FeesAndLegacyRefundStage(t *testing.T) {
	before := xdr.TransactionEventStageTransactionEventStageBeforeAllTxs
	after := xdr.TransactionEventStageTransactionEventStageAfterTx
	charge := testutil.FeeEvent(testutil.Address(1), 0, before)
	charge.Event.Body.V0.Data = testutil.I128(9223372036854775807, 18446744073709551615)
	refund := testutil.FeeEvent(testutil.Address(1), 0, after)
	refund.Event.Body.V0.Data = testutil.I128(-9223372036854775808, 101)
	meta := xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			Events: []xdr.TransactionEvent{charge, refund},
		},
	}
	ledger := testutil.Ledger(10)
	ledger.ProtocolVersion = 22
	env := testutil.PaymentEnvelope()
	transaction, err := decoder.Transaction(env, testutil.Result(env, true), meta, ledger, 1)
	if err != nil || transaction.Fees.Status != "complete" || *transaction.Fees.Net != "100" ||
		*transaction.Fees.Charged != "170141183460469231731687303715884105727" ||
		*transaction.Fees.Refunded != "170141183460469231731687303715884105627" {
		t.Fatal("signed i128 fee arithmetic lost precision", transaction.Fees, err)
	}
}

func TestCAP67InvalidShapesAndFutureEventsRemainInspectable(t *testing.T) {
	before := xdr.TransactionEventStageTransactionEventStageBeforeAllTxs
	cases := []struct {
		name   string
		mutate func(*xdr.TransactionEvent)
	}{
		{
			name: "wrong asset",
			mutate: func(event *xdr.TransactionEvent) {
				event.Event.ContractId = &xdr.ContractId{9}
			},
		},
		{
			name: "extra topic",
			mutate: func(event *xdr.TransactionEvent) {
				event.Event.Body.V0.Topics = append(
					event.Event.Body.V0.Topics,
					testutil.Symbol("extra"),
				)
			},
		},
		{
			name: "system event",
			mutate: func(event *xdr.TransactionEvent) {
				event.Event.Type = xdr.ContractEventTypeSystem
			},
		},
		{
			name: "unsigned amount",
			mutate: func(event *xdr.TransactionEvent) {
				event.Event.Body.V0.Data = xdr.ScVal{
					Type: xdr.ScValTypeScvU32,
					U32:  new(xdr.Uint32),
				}
			},
		},
	}

	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) {
			event := testutil.FeeEvent(testutil.Address(1), 100, before)
			scenario.mutate(&event)
			meta := xdr.TransactionMeta{
				V: 4,
				V4: &xdr.TransactionMetaV4{
					Events: []xdr.TransactionEvent{event},
				},
			}
			env := testutil.PaymentEnvelope()
			transaction, err := decoder.Transaction(env, testutil.Result(env, true),
				meta, testutil.Ledger(10), 1)
			if err != nil || transaction.Fees.Status != "invalid" ||
				len(transaction.Events) != 1 || transaction.Events[0].RawXDR == "" {
				t.Fatal("invalid fee shape was accepted or discarded", err)
			}
		})
	}

	unknown := testutil.FeeEvent(testutil.Address(1), 100, before)
	unknown.Event.Body.V0.Topics[0] = testutil.Symbol("future_event")
	meta := xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			Events: []xdr.TransactionEvent{unknown},
		},
	}
	env := testutil.PaymentEnvelope()
	transaction, err := decoder.Transaction(
		env,
		testutil.Result(env, true),
		meta,
		testutil.Ledger(10),
		1,
	)
	if err != nil || len(transaction.Events) != 1 || transaction.Fees.Status != "unavailable" ||
		len(transaction.TokenTransfers) != 0 {
		t.Fatal("future transaction event was discarded or interpreted as a fee/movement", err)
	}
}
