package decoder_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestAquariusStrictReceiveForwardPathsRefundsAndMetadataVersions(t *testing.T) {
	for _, hops := range []int{1, 3} {
		for _, refund := range []bool{false, true} {
			for _, version := range []int32{3, 4} {
				name := fmt.Sprintf("hops_%d_refund_%t_metadata_%d", hops, refund, version)
				t.Run(name, func(t *testing.T) {
					envelope, result, meta := testutil.AquariusStrictReceiveSource(t, hops, refund)
					if version == 3 {
						meta = aquariusMetadataV3(meta)
					}

					transaction := decodeAquarius(t, envelope, result, meta)
					action := transaction.Interpretation.Actions[0]
					swap := action.AquariusSwap
					output := strconv.FormatUint(uint64(20000000)>>hops, 10)
					if swap == nil || swap.Mode != "exact_output" || swap.Routing != "chained" ||
						swap.Input.Amount != "20000000" || swap.Output.Amount != output ||
						swap.OutputLimit != output || len(swap.Path) != hops+1 ||
						len(swap.Hops) != hops || swap.Path[0] !=
						testutil.SACContract(xdr.MustNewNativeAsset()) ||
						swap.Path[hops] != testutil.Contract(byte(9+hops)) ||
						action.From != testutil.Contract(2) || action.To != testutil.Contract(2) ||
						action.SourceAccount != testutil.Address(1) {
						t.Fatal(
							"strict receive reversed the path or confused input/output amounts",
							swap,
						)
					}

					settlement := swap.Settlement
					if settlement == nil || settlement.FundingEventIndex != 0 ||
						settlement.PayoutEventIndex != 4*hops+1 {
						t.Fatal(
							"strict-receive custody lost its source event positions",
							settlement,
						)
					}

					count := 2*hops + 2
					if refund {
						count++
						if swap.InputLimit != "21000000" || settlement.FundedAmount != "21000000" ||
							settlement.RefundedAmount != "1000000" ||
							settlement.RefundEventIndex == nil ||
							*settlement.RefundEventIndex != 4*hops+2 {
							t.Fatal(
								"unused input was not refunded after the final output",
								settlement,
							)
						}
					} else if swap.InputLimit != "20000000" ||
						settlement.FundedAmount != "20000000" ||
						settlement.RefundedAmount != "0" || settlement.RefundEventIndex != nil {
						t.Fatal("zero refund acquired an invented payment or event", settlement)
					}

					if len(swap.TransferEventIndexes) != count ||
						len(transaction.Interpretation.Actions) != 1+count {
						t.Fatal("strict receive dropped committed transfer evidence", swap)
					}
				})
			}
		}
	}
}

func TestAquariusStrictReceiveExactLargeAmounts(t *testing.T) {
	envelope, result, meta := testutil.AquariusStrictReceiveSource(t, 1, true)
	maximum := testutil.U128(9223372036854775807, 18446744073709551615)
	input := testutil.U128(9223372036854775807, 18446744073709551614)
	output := testutil.U128(9223372036854775807, 18446744073709551613)
	maximumSigned := testutil.I128(9223372036854775807, 18446744073709551615)
	inputSigned := testutil.I128(9223372036854775807, 18446744073709551614)
	outputSigned := testutil.I128(9223372036854775807, 18446744073709551613)
	call := aquariusCall(&envelope)
	call.Args[3], call.Args[4] = output, maximum
	events := meta.V4.Operations[0].Events
	events[0].Body.V0.Data = maximumSigned
	events[1].Body.V0.Data = inputSigned
	events[2].Body.V0.Data = outputSigned
	trade := events[3].Body.V0.Data.MustVec()
	(*trade)[0], (*trade)[1] = inputSigned, outputSigned
	router := events[4].Body.V0.Data.MustVec()
	(*router)[3], (*router)[4] = input, output
	events[5].Body.V0.Data = outputSigned
	events[6].Body.V0.Data = testutil.I128(0, 1)
	meta.V4.SorobanMeta.ReturnValue = valuePointer(input)
	testutil.AquariusCommitment(t, &result, meta)
	transaction := decodeAquarius(t, envelope, result, meta)
	swap := transaction.Interpretation.Actions[0].AquariusSwap
	if swap == nil || swap.InputLimit != "170141183460469231731687303715884105727" ||
		swap.Input.Amount != "170141183460469231731687303715884105726" ||
		swap.Output.Amount != "170141183460469231731687303715884105725" ||
		swap.Settlement == nil || swap.Settlement.RefundedAmount != "1" {
		t.Fatal("exact-output settlement rounded, wrapped or reversed a u128 amount", swap)
	}
}

func TestAquariusStrictReceiveRejectsUnreconciledSettlement(t *testing.T) {
	tests := []struct {
		name  string
		alter func(*xdr.TransactionEnvelope, *xdr.TransactionMeta)
	}{
		{
			name: "returned_output_instead_of_actual_input",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.SorobanMeta.ReturnValue = valuePointer(testutil.U128(0, 10000000))
			},
		},
		{
			name: "missing_refund",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events = meta.V4.Operations[0].Events[:6]
			},
		},
		{
			name: "incorrect_refund_subtraction",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events[6].Body.V0.Data = testutil.I128(0, 999999)
			},
		},
		{
			name: "refund_in_wrong_asset",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events[6].ContractId =
					testutil.ContractAddress(testutil.Contract(99)).ContractId
			},
		},
		{
			name: "refund_to_another_account",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events[6].Body.V0.Topics[2] =
					testutil.AddressValue(testutil.Address(9))
			},
		},
		{
			name: "refund_before_output_payment",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				events := meta.V4.Operations[0].Events
				events[5], events[6] = events[6], events[5]
			},
		},
		{
			name: "refund_before_pool_execution",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				events := meta.V4.Operations[0].Events
				meta.V4.Operations[0].Events = append([]xdr.ContractEvent{events[0], events[6]},
					events[1:6]...)
			},
		},
		{
			name: "duplicate_refund",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events = append(meta.V4.Operations[0].Events,
					meta.V4.Operations[0].Events[6])
			},
		},
		{
			name: "additional_asset_refund",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				extra := testutil.TokenEvent(testutil.Contract(11), "transfer",
					[]string{testutil.AquariusRouter, testutil.Contract(2)}, testutil.I128(0, 1))
				meta.V4.Operations[0].Events = append(meta.V4.Operations[0].Events, extra)
			},
		},
		{
			name: "requested_output_differs",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				aquariusCall(envelope).Args[3] = testutil.U128(0, 9999999)
			},
		},
		{
			name: "actual_input_exceeds_maximum",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				aquariusCall(envelope).Args[4] = testutil.U128(0, 19999999)
			},
		},
		{
			name: "unsigned_input_limit_exceeds_signed_token_capacity",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				aquariusCall(envelope).Args[4] = testutil.U128(18446744073709551615,
					18446744073709551615)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			envelope, result, meta := testutil.AquariusStrictReceiveSource(t, 1, true)
			test.alter(&envelope, &meta)
			testutil.AquariusCommitment(t, &result, meta)
			transaction := decodeAquarius(t, envelope, result, meta)
			action := transaction.Interpretation.Actions[0]
			if action.AquariusSwap != nil || action.Kind != "contract_call" {
				t.Fatal(
					"unreconciled maximum, refund or exact output was presented as a swap",
					action,
				)
			}
		})
	}
}

func TestAquariusStrictReceiveCannotHideIntermediateLeftovers(t *testing.T) {
	envelope, result, meta := testutil.AquariusStrictReceiveSource(t, 3, true)
	events := meta.V4.Operations[0].Events
	events[5].Body.V0.Data = testutil.I128(0, 9999999)
	trade := events[7].Body.V0.Data.MustVec()
	(*trade)[0] = testutil.I128(0, 9999999)
	router := events[8].Body.V0.Data.MustVec()
	(*router)[3] = testutil.U128(0, 9999999)
	events = append(events, testutil.TokenEvent(testutil.Contract(10), "transfer",
		[]string{testutil.AquariusRouter, testutil.Contract(2)}, testutil.I128(0, 1)))
	meta.V4.Operations[0].Events = events
	testutil.AquariusCommitment(t, &result, meta)
	transaction := decodeAquarius(t, envelope, result, meta)
	if transaction.Interpretation.Actions[0].AquariusSwap != nil ||
		len(transaction.TokenTransfers) != 10 {
		t.Fatal("intermediate leftovers were concealed by a simplified strict-receive route")
	}
}

func aquariusMetadataV3(meta xdr.TransactionMeta) xdr.TransactionMeta {
	return xdr.TransactionMeta{
		V: 3,
		V3: &xdr.TransactionMetaV3{
			Operations: []xdr.OperationMeta{{Changes: meta.V4.Operations[0].Changes}},
			SorobanMeta: &xdr.SorobanTransactionMeta{
				Events:      meta.V4.Operations[0].Events,
				ReturnValue: *meta.V4.SorobanMeta.ReturnValue,
			},
		},
	}
}
