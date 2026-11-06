package decoder_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestAquariusRealSourceFixturesAndDeterministicReplay(t *testing.T) {
	fixtures := testutil.AquariusFixtures(t)
	transactions := testutil.RealAquariusTransactions(t)
	for index, transaction := range transactions {
		t.Run(transaction.Hash, func(t *testing.T) {
			expected := fixtures[index].Expected
			action := transaction.Interpretation.Actions[0]
			swap := action.AquariusSwap
			if expected.Supported {
				if swap == nil || swap.Protocol != "aquarius" ||
					swap.Verification != "registered_router_events_and_transfers" ||
					swap.RouterWasmHash != nil || swap.Routing != expected.Routing ||
					swap.Mode != expected.Mode || swap.InputLimit != expected.InputLimit ||
					swap.OutputLimit != expected.OutputLimit ||
					swap.Trader != expected.Trader || swap.Input.Amount != expected.InputAmount ||
					swap.Output.Amount != expected.OutputAmount ||
					!slices.Equal(swap.Path, expected.TokenContracts) ||
					!slices.Equal(swap.TransferEventIndexes, expected.TransferEventIndexes) ||
					len(swap.Hops) != len(expected.PoolContracts) {
					t.Fatalf("real Aquarius exchange did not reconcile: %+v", swap)
				}

				for hopIndex, hop := range swap.Hops {
					if hop.PoolContract != expected.PoolContracts[hopIndex] ||
						hop.RouterEventIndex != expected.RouterEventIndexes[hopIndex] {
						t.Fatal("real route lost its committed pool/event evidence", hop)
					}
				}

				if expected.FundingEventIndex != nil {
					settlement := swap.Settlement
					if settlement == nil || settlement.FundedAmount != expected.FundedAmount ||
						settlement.RefundedAmount != expected.RefundedAmount ||
						settlement.FundingEventIndex != *expected.FundingEventIndex ||
						expected.PayoutEventIndex == nil ||
						settlement.PayoutEventIndex != *expected.PayoutEventIndex ||
						!reflect.DeepEqual(settlement.RefundEventIndex, expected.RefundEventIndex) {
						t.Fatal(
							"real funding, output payment or unused input refund was lost",
							settlement,
						)
					}
				} else if swap.Settlement != nil {
					t.Fatal("direct pool swap acquired fabricated router custody", swap.Settlement)
				}

				if len(swap.Routes) != len(expected.Routes) {
					t.Fatal("real split branches were omitted or invented", swap.Routes)
				}

				for routeIndex, route := range swap.Routes {
					reference := expected.Routes[routeIndex]
					if route.Index != reference.Index || route.WeightBps != reference.WeightBps ||
						route.Input.Amount != reference.InputAmount ||
						route.Output.Amount != reference.OutputAmount ||
						!slices.Equal(route.Path, reference.Path) ||
						!slices.Equal(route.HopIndexes, reference.HopIndexes) {
						t.Fatal("real split weights, amounts or branch provenance changed", route)
					}
				}

				if action.Kind != "swap" || action.From != swap.Trader ||
					action.To != swap.Trader || action.SourceAccount != transaction.SourceAccount {
					t.Fatal("actual trader and operation source were conflated", action)
				}
			} else if swap != nil || action.Kind != "contract_call" {
				t.Fatal("unsupported execution was interpreted", action)
			}

			if len(transaction.Interpretation.Actions) != 1+len(transaction.TokenTransfers) {
				t.Fatal("adapter hid committed movement evidence")
			}

			before, err := json.Marshal(transaction)
			if err != nil {
				t.Fatal(err)
			}

			if err = decoder.RebuildInterpretation(&transaction); err != nil {
				t.Fatal(err)
			}

			after, err := json.Marshal(transaction)
			if err != nil || string(before) != string(after) {
				t.Fatal("rebuilding real source changed the interpretation or retained data", err)
			}
		})
	}
}

func TestAquariusDirectAndChainedAcrossMetadataVersions(t *testing.T) {
	for _, chained := range []bool{false, true} {
		for _, version := range []int32{3, 4} {
			t.Run(fmt.Sprintf("chained_%t_metadata_%d", chained, version), func(t *testing.T) {
				envelope, result, meta := testutil.AquariusSource(t, chained)
				if version == 3 {
					meta = xdr.TransactionMeta{
						V: 3,
						V3: &xdr.TransactionMetaV3{
							Operations: []xdr.OperationMeta{{}},
							SorobanMeta: &xdr.SorobanTransactionMeta{
								Events:      meta.V4.Operations[0].Events,
								ReturnValue: *meta.V4.SorobanMeta.ReturnValue,
							},
						},
					}
				}

				transaction := decodeAquarius(t, envelope, result, meta)
				action := transaction.Interpretation.Actions[0]
				swap := action.AquariusSwap
				if swap == nil || swap.Mode != "exact_input" ||
					swap.Input.Amount != "20000000" || swap.InputLimit != "20000000" ||
					*swap.Input.Asset.Code != "XLM" || swap.Trader != testutil.Contract(2) ||
					action.From != swap.Trader || action.To != swap.Trader ||
					action.SourceAccount != testutil.Address(1) {
					t.Fatal("typed request or token owner was interpreted incorrectly", swap)
				}

				if chained {
					if swap.Routing != "chained" || swap.Output.Amount != "5000000" ||
						swap.OutputLimit != "4999999" || len(swap.Hops) != 2 ||
						len(swap.TransferEventIndexes) != 6 || len(swap.Path) != 3 ||
						swap.Hops[0].OutputEventIndex == swap.Hops[1].InputEventIndex ||
						swap.Output.Asset.Decimals != nil {
						t.Fatal(
							"router custody transfers or unknown token metadata were lost",
							swap,
						)
					}
				} else if swap.Routing != "direct" || swap.Output.Amount != "10000000" ||
					swap.OutputLimit != "9999999" || len(swap.Hops) != 1 ||
					len(swap.TransferEventIndexes) != 2 ||
					swap.Hops[0].PoolContract != testutil.Contract(20) {
					t.Fatal("direct pool route failed to reconcile", swap)
				}
			})
		}
	}
}

func TestAquariusRebuildUsesSourceAndPreservesDerivedPayload(t *testing.T) {
	transaction := testutil.AquariusChainedBundle(t, 10).Transactions[0]
	expected := transaction.Interpretation
	transaction.Calls = nil
	transaction.Events = nil
	transaction.TokenTransfers = nil
	transaction.Contracts = nil
	transaction.ReturnValue = "untrusted"
	transaction.Interpretation = domain.Interpretation{}
	if err := decoder.RebuildInterpretation(&transaction); err != nil ||
		!reflect.DeepEqual(expected, transaction.Interpretation) || transaction.Calls != nil ||
		transaction.Events != nil || transaction.TokenTransfers != nil || transaction.Contracts != nil ||
		transaction.ReturnValue != "untrusted" {
		t.Fatal("adapter trusted or changed derived payload instead of retained source", err)
	}

	transaction.Network = "mainnet"
	if err := decoder.RebuildInterpretation(&transaction); err == nil {
		t.Fatal("Testnet Aquarius deployment was interpreted on another network")
	}
}

func TestAquariusAccountTraderAndFeeBumpKeepDistinctProvenance(t *testing.T) {
	inner, innerResult, meta := testutil.AquariusSource(t, false)
	trader := testutil.Address(2)
	aquariusCall(&inner).Args[0] = testutil.AddressValue(trader)
	for index := range meta.V4.Operations[0].Events {
		for topicIndex, topic := range meta.V4.Operations[0].Events[index].Body.V0.Topics {
			if address, ok := topic.GetAddress(); ok {
				encoded, err := address.String()
				if err == nil && encoded == testutil.Contract(2) {
					meta.V4.Operations[0].Events[index].Body.V0.Topics[topicIndex] =
						testutil.AddressValue(trader)
				}
			}
		}
	}

	testutil.AquariusCommitment(t, &innerResult, meta)
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
	transaction := decodeAquarius(t, envelope, result, meta)
	action := transaction.Interpretation.Actions[0]
	if action.AquariusSwap == nil || action.From != trader || action.To != trader ||
		action.SourceAccount != testutil.Address(
			6,
		) || transaction.SourceAccount != testutil.Address(1) ||
		transaction.FeeAccount != testutil.Address(9) || transaction.Fees.Status != "complete" {
		t.Fatal("operation source, envelope source, fee payer and trader were conflated", action)
	}
}

func decodeAquarius(
	t *testing.T,
	envelope xdr.TransactionEnvelope,
	result xdr.TransactionResultPair,
	meta xdr.TransactionMeta,
) domain.Transaction {
	t.Helper()
	identity := testutil.Result(envelope, result.Result.Successful())
	result.TransactionHash = identity.TransactionHash
	transaction, err := decoder.Transaction(envelope, result, meta, testutil.Ledger(10), 1)
	if err != nil {
		t.Fatal(err)
	}

	return transaction
}

func aquariusCall(envelope *xdr.TransactionEnvelope) *xdr.InvokeContractArgs {
	return envelope.V1.Tx.Operations[0].Body.InvokeHostFunctionOp.HostFunction.InvokeContract
}
