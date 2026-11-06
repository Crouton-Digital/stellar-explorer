package decoder_test

import (
	"testing"

	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestSoroswapRejectsUnsupportedAmbiguousAndUnreconciledEvidence(t *testing.T) {
	tests := []struct {
		name  string
		alter func(*xdr.TransactionEnvelope, *xdr.TransactionMeta)
	}{
		{
			name: "unregistered_router",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				soroswapCall(envelope).ContractAddress = testutil.ContractAddress(
					testutil.Contract(9),
				)
			},
		},
		{
			name: "function_name_alone",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				soroswapCall(envelope).FunctionName = "swap"
			},
		},
		{
			name: "missing_argument",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				soroswapCall(envelope).Args = soroswapCall(envelope).Args[:4]
			},
		},
		{
			name: "expired_at_exact_deadline",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				deadline := xdr.Uint64(1700000010)
				soroswapCall(envelope).Args[4] = xdr.ScVal{
					Type: xdr.ScValTypeScvU64,
					U64:  &deadline,
				}
			},
		},
		{
			name: "cyclic_path",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				path := soroswapCall(envelope).Args[2].MustVec()
				(*path)[1] = (*path)[0]
			},
		},
		{
			name: "account_as_token",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				path := soroswapCall(envelope).Args[2].MustVec()
				(*path)[0] = testutil.AddressValue(testutil.Address(3))
			},
		},
		{
			name: "negative_limit",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				soroswapCall(envelope).Args[0] = testutil.I128(-1, 18446744073709551615)
			},
		},
		{
			name: "exact_input_does_not_match",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				soroswapCall(envelope).Args[0] = testutil.I128(0, 19999999)
			},
		},
		{
			name: "below_minimum_output",
			alter: func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionMeta) {
				soroswapCall(envelope).Args[1] = testutil.I128(0, 10000001)
			},
		},
		{
			name: "wrong_router_topic_type",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events[2].Body.V0.Topics[0] = testutil.Symbol(
					"SoroswapRouter",
				)
			},
		},
		{
			name: "duplicate_router_swap",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events = append(meta.V4.Operations[0].Events,
					meta.V4.Operations[0].Events[2])
			},
		},
		{
			name: "duplicate_router_map_key",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				entries := meta.V4.Operations[0].Events[2].Body.V0.Data.MustMap()
				(*entries)[1].Key = testutil.Symbol("amounts")
			},
		},
		{
			name: "wrong_router_trader",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				entries := meta.V4.Operations[0].Events[2].Body.V0.Data.MustMap()
				(*entries)[2].Val = testutil.AddressValue(testutil.Address(5))
			},
		},
		{
			name: "return_vector_disagrees",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.SorobanMeta.ReturnValue = valuePointer(testutil.Vec(
					testutil.I128(0, 20000000), testutil.I128(0, 10000001)))
			},
		},
		{
			name: "nonpositive_return_amount",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.SorobanMeta.ReturnValue = valuePointer(testutil.Vec(
					testutil.I128(0, 20000000), testutil.I128(0, 0)))
			},
		},
		{
			name: "opposing_transfers_through_forged_pool",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Events[0].Body.V0.Topics[2] = testutil.AddressValue(
					testutil.Contract(7),
				)
				meta.V4.Operations[0].Events[1].Body.V0.Topics[1] = testutil.AddressValue(
					testutil.Contract(7),
				)
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
			name: "extra_unsupported_token_shaped_event",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				extra := testutil.TokenEvent(testutil.Contract(8), "mint",
					[]string{testutil.Address(2)}, testutil.I128(0, 1))
				meta.V4.Operations[0].Events = append(meta.V4.Operations[0].Events, extra)
			},
		},
		{
			name: "router_event_before_route_transfers",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				events := meta.V4.Operations[0].Events
				meta.V4.Operations[0].Events = []xdr.ContractEvent{events[2], events[0], events[1]}
			},
		},
		{
			name: "conflicting_observed_router_executable",
			alter: func(_ *xdr.TransactionEnvelope, meta *xdr.TransactionMeta) {
				meta.V4.Operations[0].Changes = xdr.LedgerEntryChanges{
					testutil.InstanceChange(testutil.SoroswapRouter, 10,
						xdr.LedgerEntryChangeTypeLedgerEntryState, testutil.WasmExecutable(7)),
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
			envelope, result, meta := testutil.SoroswapSource(t, false)
			test.alter(&envelope, &meta)
			testutil.SetSoroswapCommitment(t, &result, meta)
			transaction := decodeSoroswap(t, envelope, result, meta)
			if transaction.Interpretation.Actions[0].ContractSwap != nil ||
				transaction.Interpretation.Actions[0].Kind != "contract_call" ||
				transaction.Interpretation.Actions[0].ContractID == "" || transaction.MetaXDR == "" {
				t.Fatal(
					"unverified swap replaced generic source evidence",
					transaction.Interpretation,
				)
			}
		})
	}
}

func TestSoroswapFailureAndTamperedResultCannotCreateSwap(t *testing.T) {
	for _, failed := range []bool{false, true} {
		envelope, result, meta := testutil.SoroswapSource(t, false)
		if failed {
			result.Result.Result.Code = xdr.TransactionResultCodeTxFailed
		} else {
			hash := (*result.Result.Result.Results)[0].Tr.InvokeHostFunctionResult.Success
			hash[0] ^= 1
		}

		transaction := decodeSoroswap(t, envelope, result, meta)
		if transaction.Interpretation.Actions[0].ContractSwap != nil ||
			transaction.Interpretation.Actions[0].Kind == "swap" {
			t.Fatal("failed or noncommitted result created an interpreted swap")
		}
	}
}

func soroswapCall(envelope *xdr.TransactionEnvelope) *xdr.InvokeContractArgs {
	return envelope.V1.Tx.Operations[0].Body.InvokeHostFunctionOp.HostFunction.InvokeContract
}

func valuePointer(value xdr.ScVal) *xdr.ScVal {
	return &value
}
