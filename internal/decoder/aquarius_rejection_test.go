package decoder_test

import (
	"testing"

	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestAquariusRejectsUnsupportedAndUnreconciledDirectEvidence(t *testing.T) {
	tests := []struct {
		name  string
		alter func(*xdr.TransactionEnvelope, *xdr.TransactionMeta)
	}{
		{
			name: "unregistered_router",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				aquariusCall(envelope).ContractAddress = testutil.ContractAddress(
					testutil.Contract(9),
				)
			},
		},
		{
			name: "unsupported_split",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				aquariusCall(envelope).FunctionName = "swap_split"
			},
		},
		{
			name: "unsupported_strict_receive",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				aquariusCall(envelope).FunctionName = "swap_chained_strict_receive"
			},
		},
		{
			name: "missing_argument",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				aquariusCall(envelope).Args = aquariusCall(envelope).Args[:6]
			},
		},
		{
			name: "signed_amount_in_unsigned_request",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				aquariusCall(envelope).Args[5] = testutil.I128(0, 20000000)
			},
		},
		{
			name: "zero_input",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				aquariusCall(envelope).Args[5] = testutil.U128(0, 0)
			},
		},
		{
			name: "input_differs_from_execution",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				aquariusCall(envelope).Args[5] = testutil.U128(0, 19999999)
			},
		},
		{
			name: "below_minimum_output",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				aquariusCall(envelope).Args[6] = testutil.U128(0, 10000001)
			},
		},
		{
			name: "wrong_pool_index_type",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				aquariusCall(envelope).Args[4] = testutil.String("pool")
			},
		},
		{
			name: "wrong_pool_index_length",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				index := xdr.ScBytes{1}
				aquariusCall(envelope).Args[4] = xdr.ScVal{
					Type:  xdr.ScValTypeScvBytes,
					Bytes: &index,
				}
			},
		},
		{
			name: "unsorted_pool_tokens",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				tokens := aquariusCall(envelope).Args[1].MustVec()
				(*tokens)[0], (*tokens)[1] = (*tokens)[1], (*tokens)[0]
			},
		},
		{
			name: "duplicate_pool_tokens",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				tokens := aquariusCall(envelope).Args[1].MustVec()
				(*tokens)[1] = (*tokens)[0]
			},
		},
		{
			name: "account_as_pool_token",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				tokens := aquariusCall(envelope).Args[1].MustVec()
				(*tokens)[0] = testutil.AddressValue(testutil.Address(3))
			},
		},
		{
			name: "token_missing_from_pool",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				aquariusCall(envelope).Args[2] = testutil.AddressValue(testutil.Contract(99))
			},
		},
		{
			name: "identical_input_and_output",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				aquariusCall(envelope).Args[3] = aquariusCall(envelope).Args[2]
			},
		},
		{
			name: "wrong_router_topic_type",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events[3].Body.V0.Topics[0] = testutil.String("swap")
			},
		},
		{
			name: "wrong_router_trader",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events[3].Body.V0.Topics[2] =
					testutil.AddressValue(testutil.Address(8))
			},
		},
		{
			name: "account_as_pool_contract",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				data := meta.V4.Operations[0].Events[3].Body.V0.Data.MustVec()
				(*data)[0] = testutil.AddressValue(testutil.Address(20))
			},
		},
		{
			name: "duplicate_router_event",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events = append(meta.V4.Operations[0].Events,
					meta.V4.Operations[0].Events[3])
			},
		},
		{
			name: "wrong_router_amount_type",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				data := meta.V4.Operations[0].Events[3].Body.V0.Data.MustVec()
				(*data)[3] = testutil.I128(0, 20000000)
			},
		},
		{
			name: "router_amount_disagrees",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				data := meta.V4.Operations[0].Events[3].Body.V0.Data.MustVec()
				(*data)[4] = testutil.U128(0, 10000001)
			},
		},
		{
			name: "wrong_pool_trade_emitter",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events[2].ContractId =
					testutil.ContractAddress(testutil.Contract(21)).ContractId
			},
		},
		{
			name: "wrong_pool_trade_settlement_actor",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events[2].Body.V0.Topics[3] =
					testutil.AddressValue(testutil.AquariusRouter)
			},
		},
		{
			name: "pool_trade_amount_disagrees",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				data := meta.V4.Operations[0].Events[2].Body.V0.Data.MustVec()
				(*data)[0] = testutil.I128(0, 20000001)
			},
		},
		{
			name: "negative_raw_trade_fee",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				data := meta.V4.Operations[0].Events[2].Body.V0.Data.MustVec()
				(*data)[2] = testutil.I128(-1, 18446744073709551615)
			},
		},
		{
			name: "extra_pool_trade",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events = append(meta.V4.Operations[0].Events,
					meta.V4.Operations[0].Events[2])
			},
		},
		{
			name: "return_amount_disagrees",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.SorobanMeta.ReturnValue = valuePointer(testutil.U128(0, 10000001))
			},
		},
		{
			name: "signed_return_value",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.SorobanMeta.ReturnValue = valuePointer(testutil.I128(0, 10000000))
			},
		},
		{
			name: "transfer_amount_disagrees",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events[0].Body.V0.Data = testutil.I128(0, 20000001)
			},
		},
		{
			name: "extra_transfer",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events = append(meta.V4.Operations[0].Events,
					meta.V4.Operations[0].Events[0])
			},
		},
		{
			name: "unsupported_token_shaped_effect",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				extra := testutil.TokenEvent(testutil.Contract(8), "mint",
					[]string{testutil.Contract(2)}, testutil.I128(0, 1))
				meta.V4.Operations[0].Events = append(meta.V4.Operations[0].Events, extra)
			},
		},
		{
			name: "router_event_before_trade",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				events := meta.V4.Operations[0].Events
				events[2], events[3] = events[3], events[2]
			},
		},
		{
			name: "trade_before_output_transfer",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				events := meta.V4.Operations[0].Events
				events[1], events[2] = events[2], events[1]
			},
		},
		{
			name: "conflicting_observed_executables",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.TxChangesBefore = xdr.LedgerEntryChanges{
					testutil.InstanceChange(testutil.AquariusRouter, 10,
						xdr.LedgerEntryChangeTypeLedgerEntryState, testutil.WasmExecutable(1)),
				}
				meta.V4.TxChangesAfter = xdr.LedgerEntryChanges{
					testutil.InstanceChange(testutil.AquariusRouter, 10,
						xdr.LedgerEntryChangeTypeLedgerEntryState, testutil.WasmExecutable(2)),
				}
			},
		},
		{
			name: "multiple_operations",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				envelope.V1.Tx.Operations = append(envelope.V1.Tx.Operations,
					testutil.PaymentEnvelope().V1.Tx.Operations[0])
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			envelope, result, meta := testutil.AquariusSource(t, false)
			test.alter(&envelope, &meta)
			testutil.AquariusCommitment(t, &result, meta)
			transaction := decodeAquarius(t, envelope, result, meta)
			action := transaction.Interpretation.Actions[0]
			if action.AquariusSwap != nil || action.Kind != "contract_call" ||
				action.ContractID == "" || transaction.MetaXDR == "" {
				t.Fatal("unsupported swap replaced generic source evidence", action)
			}
		})
	}
}

func TestAquariusRejectsAmbiguousChainedCustodyAndPaths(t *testing.T) {
	tests := []struct {
		name  string
		alter func(*xdr.TransactionEnvelope, *xdr.TransactionMeta)
	}{
		{
			name: "wrong_funding_recipient",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events[0].Body.V0.Topics[2] =
					testutil.AddressValue(testutil.Contract(20))
			},
		},
		{
			name: "wrong_pool_settlement_actor",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events[3].Body.V0.Topics[3] =
					testutil.AddressValue(testutil.Contract(2))
			},
		},
		{
			name: "wrong_final_payout",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events[9].Body.V0.Data = testutil.I128(0, 4999999)
			},
		},
		{
			name: "funding_after_first_pool_input",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				events := meta.V4.Operations[0].Events
				events[0], events[1] = events[1], events[0]
			},
		},
		{
			name: "intermediate_amount_not_carried_forward",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				events := meta.V4.Operations[0].Events
				events[5].Body.V0.Data = testutil.I128(0, 9999999)
				trade := events[7].Body.V0.Data.MustVec()
				(*trade)[0] = testutil.I128(0, 9999999)
				router := events[8].Body.V0.Data.MustVec()
				(*router)[3] = testutil.U128(0, 9999999)
			},
		},
		{
			name: "empty_path",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				aquariusCall(envelope).Args[1] = testutil.Vec()
			},
		},
		{
			name: "too_many_hops",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				chain := aquariusCall(envelope).Args[1].MustVec()
				step := (*chain)[0]
				for len(*chain) <= 16 {
					*chain = append(*chain, step)
				}
			},
		},
		{
			name: "disconnected_hop",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				chain := aquariusCall(envelope).Args[1].MustVec()
				second := (*chain)[1].MustVec()
				(*second)[0] = testutil.AquariusPoolTokens(testutil.Contract(10),
					testutil.Contract(11))
			},
		},
		{
			name: "cyclic_path",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				call := aquariusCall(envelope)
				chain := call.Args[1].MustVec()
				first, second := (*chain)[0].MustVec(), (*chain)[1].MustVec()
				(*second)[0] = (*first)[0]
				(*second)[2] = call.Args[2]
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			envelope, result, meta := testutil.AquariusSource(t, true)
			test.alter(&envelope, &meta)
			testutil.AquariusCommitment(t, &result, meta)
			transaction := decodeAquarius(t, envelope, result, meta)
			if transaction.Interpretation.Actions[0].AquariusSwap != nil {
				t.Fatal("incomplete chained custody or ambiguous route became a swap")
			}
		})
	}
}

func TestAquariusFailedMissingOrUncommittedEvidenceFallsBack(t *testing.T) {
	for _, scenario := range []string{"failed", "bad_hash", "missing_return", "diagnostic_only"} {
		t.Run(scenario, func(t *testing.T) {
			envelope, result, meta := testutil.AquariusSource(t, false)
			switch scenario {
			case "failed":
				result.Result.Result.Code = xdr.TransactionResultCodeTxFailed
			case "bad_hash":
				hash := (*result.Result.Result.Results)[0].Tr.InvokeHostFunctionResult.Success
				hash[0] ^= 1
			case "missing_return":
				meta.V4.SorobanMeta.ReturnValue = nil
			case "diagnostic_only":
				for _, event := range meta.V4.Operations[0].Events {
					meta.V4.DiagnosticEvents = append(meta.V4.DiagnosticEvents,
						xdr.DiagnosticEvent{InSuccessfulContractCall: true, Event: event})
				}

				meta.V4.Operations[0].Events = nil
				testutil.AquariusCommitment(t, &result, meta)
			}

			transaction := decodeAquarius(t, envelope, result, meta)
			action := transaction.Interpretation.Actions[0]
			if action.AquariusSwap != nil || action.Kind != "contract_call" {
				t.Fatal("failed, diagnostic-only or noncommitted evidence became a swap", action)
			}
		})
	}
}
