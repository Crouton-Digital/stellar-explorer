package decoder_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestSoroswapRealSourceFixturesAndDeterministicReplay(t *testing.T) {
	fixtures := testutil.SoroswapFixtures(t)
	transactions := testutil.RealSoroswapTransactions(t)
	for index, transaction := range transactions {
		t.Run(transaction.Hash, func(t *testing.T) {
			swap := transaction.Interpretation.Actions[0].ContractSwap
			expected := fixtures[index].Expected
			if swap == nil || swap.Protocol != "soroswap" ||
				swap.Verification != "registered_router_and_transfers" || swap.RouterWasmHash != nil ||
				swap.Trader != expected.Trader || swap.Input.Amount != expected.InputAmount ||
				swap.Output.Amount != expected.OutputAmount ||
				swap.RouterEventIndex != expected.RouterEventIndex ||
				!slices.Equal(
					swap.Path,
					expected.TokenContracts,
				) || len(swap.Hops) != len(swap.Path)-1 {
				t.Fatalf("real Soroswap source did not reconcile: %+v", swap)
			}

			action := transaction.Interpretation.Actions[0]
			if action.From != swap.Trader || action.To != swap.Trader ||
				action.SourceAccount != transaction.SourceAccount || action.Kind != "swap" {
				t.Fatal("trader was confused with the operation source", action)
			}

			if len(transaction.Interpretation.Actions) != 1+len(transaction.TokenTransfers) {
				t.Fatal("committed transfer evidence was hidden")
			}

			before, _ := json.Marshal(transaction)
			if err := decoder.RebuildInterpretation(&transaction); err != nil {
				t.Fatal(err)
			}

			after, _ := json.Marshal(transaction)
			if string(before) != string(after) {
				t.Fatal("rebuilding a real Soroswap interpretation changed stored data")
			}
		})
	}
}

func TestSoroswapExactInputAndOutputAcrossMetadataVersions(t *testing.T) {
	for _, exactOutput := range []bool{false, true} {
		for _, version := range []int32{3, 4} {
			envelope, result, meta := testutil.SoroswapSource(t, exactOutput)
			if version == 3 {
				meta = xdr.TransactionMeta{
					V: 3,
					V3: &xdr.TransactionMetaV3{
						SorobanMeta: &xdr.SorobanTransactionMeta{
							ReturnValue: *meta.V4.SorobanMeta.ReturnValue,
							Events:      meta.V4.Operations[0].Events,
						},
					},
				}
			}

			transaction := decodeSoroswap(t, envelope, result, meta)
			action := transaction.Interpretation.Actions[0]
			swap := action.ContractSwap
			if swap == nil || swap.Input.Amount != "20000000" || swap.Output.Amount != "10000000" ||
				*swap.Input.Asset.Code != "XLM" || *swap.Output.Asset.Code != "USD" ||
				len(swap.Hops) != 1 || swap.Hops[0].PoolContract != testutil.SoroswapPool ||
				action.SourceAccount != testutil.Address(1) || swap.Trader != testutil.Address(2) {
				t.Fatal("Soroswap request or amounts were misinterpreted", swap)
			}

			if exactOutput && (swap.Mode != "exact_output" || swap.InputLimit != "30000000" ||
				swap.OutputLimit != "10000000") {
				t.Fatal("exact-output ABI order was reversed", swap)
			}
		}
	}
}

func TestSoroswapRebuildIgnoresDerivedMovementsCallsAndState(t *testing.T) {
	transaction := testutil.SoroswapBundle(t, 10).Transactions[0]
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
		t.Fatal("adapter trusted or changed derived payload", err)
	}
}

func decodeSoroswap(
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
