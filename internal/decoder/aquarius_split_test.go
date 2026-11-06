package decoder_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestAquariusSplitBranchesKeepAggregateAmountsAndReplay(t *testing.T) {
	for _, version := range []int32{3, 4} {
		t.Run(fmt.Sprintf("metadata_%d", version), func(t *testing.T) {
			envelope, result, meta := testutil.AquariusSplitSource(t)
			if version == 3 {
				meta = aquariusMetadataV3(meta)
			}

			transaction := decodeAquarius(t, envelope, result, meta)
			action := transaction.Interpretation.Actions[0]
			swap := action.AquariusSwap
			if swap == nil || swap.Mode != "exact_input" || swap.Routing != "split" ||
				swap.AdapterVersion != 2 || len(swap.Path) != 0 || len(swap.Hops) != 3 ||
				len(swap.Routes) != 2 || swap.Input.Amount != "100000000" ||
				swap.Output.Amount != "40000000" || swap.InputLimit != "100000000" ||
				swap.OutputLimit != "39999999" || len(swap.TransferEventIndexes) != 8 {
				t.Fatal("split branch amounts replaced the aggregate exchange", swap)
			}

			first, second := swap.Routes[0], swap.Routes[1]
			if first.WeightBps != 6000 || second.WeightBps != 4000 ||
				first.Input.Amount != "60000000" || first.Output.Amount != "30000000" ||
				second.Input.Amount != "40000000" || second.Output.Amount != "10000000" ||
				!slices.Equal(first.HopIndexes, []int{0}) ||
				!slices.Equal(second.HopIndexes, []int{1, 2}) ||
				len(first.Path) != 2 || len(second.Path) != 3 {
				t.Fatal("split route weights, paths or pool references were lost", swap.Routes)
			}

			settlement := swap.Settlement
			if settlement == nil || settlement.FundedAmount != "100000000" ||
				settlement.RefundedAmount != "0" || settlement.RefundEventIndex != nil ||
				settlement.FundingEventIndex != 0 || settlement.PayoutEventIndex != 13 ||
				len(transaction.Interpretation.Actions) != 9 {
				t.Fatal(
					"split funding or payout was lost or counted as extra swap volume",
					settlement,
				)
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
				t.Fatal("split source replay changed retained data or its interpretation", err)
			}
		})
	}
}

func TestAquariusSplitRejectsUnsupportedAllocationAndSettlement(t *testing.T) {
	tests := []struct {
		name  string
		alter func(*xdr.InvokeContractArgs, *xdr.TransactionMeta)
	}{
		{
			name: "multiple_split_groups",
			alter: func(call *xdr.InvokeContractArgs, _ *xdr.TransactionMeta) {
				group := (*call.Args[1].MustVec())[0]
				call.Args[1] = testutil.Vec(group, group)
			},
		},
		{
			name: "only_one_branch",
			alter: func(call *xdr.InvokeContractArgs, _ *xdr.TransactionMeta) {
				group := aquariusSplitGroup(call)
				*group = (*group)[:1]
			},
		},
		{
			name: "more_than_eight_branches",
			alter: func(call *xdr.InvokeContractArgs, _ *xdr.TransactionMeta) {
				group := aquariusSplitGroup(call)
				first := (*group)[0]
				for len(*group) < 9 {
					*group = append(*group, first)
				}
			},
		},
		{
			name: "weights_do_not_sum_to_ten_thousand",
			alter: func(call *xdr.InvokeContractArgs, _ *xdr.TransactionMeta) {
				fields := (*aquariusSplitGroup(call))[0].MustVec()
				*(*fields)[1].U32 = 5999
			},
		},
		{
			name: "zero_weight",
			alter: func(call *xdr.InvokeContractArgs, _ *xdr.TransactionMeta) {
				fields := (*aquariusSplitGroup(call))[0].MustVec()
				*(*fields)[1].U32 = 0
			},
		},
		{
			name: "oversized_weight",
			alter: func(call *xdr.InvokeContractArgs, _ *xdr.TransactionMeta) {
				fields := (*aquariusSplitGroup(call))[0].MustVec()
				*(*fields)[1].U32 = 4294967295
			},
		},
		{
			name: "incorrect_weight_type",
			alter: func(call *xdr.InvokeContractArgs, _ *xdr.TransactionMeta) {
				fields := (*aquariusSplitGroup(call))[0].MustVec()
				(*fields)[1] = testutil.U128(0, 6000)
			},
		},
		{
			name: "branch_amounts_disagree_with_weights",
			alter: func(call *xdr.InvokeContractArgs, _ *xdr.TransactionMeta) {
				group := aquariusSplitGroup(call)
				first, second := (*group)[0].MustVec(), (*group)[1].MustVec()
				*(*first)[1].U32, *(*second)[1].U32 = 4000, 6000
			},
		},
		{
			name: "fractional_allocation_with_guessed_rounding",
			alter: func(call *xdr.InvokeContractArgs, meta *xdr.TransactionMeta) {
				call.Args[4] = testutil.U128(0, 100000001)
				events := meta.V4.Operations[0].Events
				events[0].Body.V0.Data = testutil.I128(0, 100000001)
				events[1].Body.V0.Data = testutil.I128(0, 60000001)
				trade := events[3].Body.V0.Data.MustVec()
				(*trade)[0] = testutil.I128(0, 60000001)
				router := events[4].Body.V0.Data.MustVec()
				(*router)[3] = testutil.U128(0, 60000001)
			},
		},
		{
			name: "branch_does_not_reach_common_output",
			alter: func(call *xdr.InvokeContractArgs, _ *xdr.TransactionMeta) {
				call.Args[3] = testutil.AddressValue(testutil.Contract(99))
			},
		},
		{
			name: "return_is_last_branch_instead_of_total",
			alter: func(_ *xdr.InvokeContractArgs, meta *xdr.TransactionMeta) {
				meta.V4.SorobanMeta.ReturnValue = valuePointer(testutil.U128(0, 10000000))
			},
		},
		{
			name: "payout_and_return_do_not_sum_to_branch_outputs",
			alter: func(_ *xdr.InvokeContractArgs, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events[13].Body.V0.Data = testutil.I128(0, 39999999)
				meta.V4.SorobanMeta.ReturnValue = valuePointer(testutil.U128(0, 39999999))
			},
		},
		{
			name: "minimum_output_is_not_met",
			alter: func(call *xdr.InvokeContractArgs, _ *xdr.TransactionMeta) {
				call.Args[5] = testutil.U128(0, 40000001)
			},
		},
		{
			name: "unexpected_refund",
			alter: func(_ *xdr.InvokeContractArgs, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events = append(meta.V4.Operations[0].Events,
					testutil.TokenEvent(
						testutil.SACContract(xdr.MustNewNativeAsset()),
						"transfer",
						[]string{
							testutil.AquariusRouter,
							testutil.Contract(2),
						},
						testutil.I128(0, 1),
						testutil.String("native"),
					))
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			envelope, result, meta := testutil.AquariusSplitSource(t)
			test.alter(aquariusCall(&envelope), &meta)
			testutil.AquariusCommitment(t, &result, meta)
			transaction := decodeAquarius(t, envelope, result, meta)
			action := transaction.Interpretation.Actions[0]
			if action.AquariusSwap != nil || action.Kind != "contract_call" {
				t.Fatal(
					"unsupported split allocation or incomplete settlement became a swap",
					action,
				)
			}
		})
	}
}

func aquariusSplitGroup(call *xdr.InvokeContractArgs) *xdr.ScVec {
	groups := call.Args[1].MustVec()
	return (*groups)[0].MustVec()
}
