package decoder_test

import (
	"reflect"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestNFTRejectsAmbiguousAmountsUnsupportedMapsAndInvalidEventShapes(t *testing.T) {
	tests := []struct {
		name  string
		alter func(*xdr.ContractEvent)
	}{
		{
			name: "fungible_i128_amount",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Data = testutil.I128(0, 7)
			},
		},
		{
			name: "negative_i128",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Data = testutil.I128(-1, 18446744073709551615)
			},
		},
		{
			name: "signed_positive_i32",
			alter: func(event *xdr.ContractEvent) {
				value := xdr.Int32(7)
				event.Body.V0.Data = xdr.ScVal{Type: xdr.ScValTypeScvI32, I32: &value}
			},
		},
		{
			name: "numeric_string",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Data = testutil.String("7")
			},
		},
		{
			name: "numeric_symbol",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Data = testutil.Symbol("7")
			},
		},
		{
			name: "single_element_vector",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Data = testutil.Vec(testutil.U32(7))
			},
		},
		{
			name: "fungible_amount_map",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Data = testutil.Map(xdr.ScMap{
					{Key: testutil.Symbol("amount"), Val: testutil.I128(0, 7)},
				})
			},
		},
		{
			name: "token_id_map_with_signed_value",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Data = testutil.Map(xdr.ScMap{
					{Key: testutil.Symbol("token_id"), Val: testutil.I128(0, 7)},
				})
			},
		},
		{
			name: "token_id_map_with_extra_field",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Data = testutil.Map(xdr.ScMap{
					{Key: testutil.Symbol("amount"), Val: testutil.I128(0, 7)},
					{Key: testutil.Symbol("token_id"), Val: testutil.U32(7)},
				})
			},
		},
		{
			name: "duplicate_token_id_map_key",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Data = testutil.Map(xdr.ScMap{
					{Key: testutil.Symbol("token_id"), Val: testutil.U32(7)},
					{Key: testutil.Symbol("token_id"), Val: testutil.U32(8)},
				})
			},
		},
		{
			name: "string_token_id_map_key",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Data = testutil.Map(xdr.ScMap{
					{Key: testutil.String("token_id"), Val: testutil.U32(7)},
				})
			},
		},
		{
			name: "empty_map",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Data = testutil.Map(xdr.ScMap{})
			},
		},
		{
			name: "string_event_name",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Topics[0] = testutil.String("transfer")
			},
		},
		{
			name: "string_sender",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Topics[1] = testutil.String(testutil.Address(2))
			},
		},
		{
			name: "missing_recipient",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Topics = event.Body.V0.Topics[:2]
			},
		},
		{
			name: "extra_topic",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Topics = append(
					event.Body.V0.Topics,
					testutil.String("asset metadata"),
				)
			},
		},
		{
			name: "mint_with_legacy_sac_admin_topic",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Topics[0] = testutil.Symbol("mint")
			},
		},
		{
			name: "burn_with_extra_topic",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Topics[0] = testutil.Symbol("burn")
			},
		},
		{
			name: "approval",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Topics[0] = testutil.Symbol("approve")
			},
		},
		{
			name: "consecutive_mint",
			alter: func(event *xdr.ContractEvent) {
				event.Body.V0.Topics = []xdr.ScVal{
					testutil.Symbol("consecutive_mint"),
					testutil.AddressValue(testutil.Address(2)),
				}
				event.Body.V0.Data = testutil.Map(xdr.ScMap{
					{Key: testutil.Symbol("from_token_id"), Val: testutil.U32(0)},
					{Key: testutil.Symbol("to_token_id"), Val: testutil.U32(4294967295)},
				})
			},
		},
		{
			name: "system_event",
			alter: func(event *xdr.ContractEvent) {
				event.Type = xdr.ContractEventTypeSystem
			},
		},
		{
			name: "diagnostic_event",
			alter: func(event *xdr.ContractEvent) {
				event.Type = xdr.ContractEventTypeDiagnostic
			},
		},
		{
			name: "missing_emitter",
			alter: func(event *xdr.ContractEvent) {
				event.ContractId = nil
			},
		},
		{
			name: "reserved_native_sac_emitter",
			alter: func(event *xdr.ContractEvent) {
				contract := testutil.SACContract(xdr.MustNewNativeAsset())
				event.ContractId = testutil.ContractAddress(contract).ContractId
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := testutil.NFTEvent(testutil.Contract(30), "transfer",
				[]string{testutil.Address(2), testutil.Address(3)}, testutil.U32(7), false)
			test.alter(&event)
			transaction := testutil.NFTBundle(t, 10, event).Transactions[0]
			if len(transaction.NFTTransfers) != 0 || transaction.MetaXDR == "" ||
				len(transaction.Events) != 1 {
				t.Fatal("unsupported event shape became an NFT or lost its raw evidence")
			}
		})
	}
}

func TestNFTFeeBumpDoesNotInferTokenOwnerFromSourceAccounts(t *testing.T) {
	event := testutil.NFTEvent(
		testutil.Contract(30),
		"transfer",
		[]string{
			testutil.Address(2),
			testutil.Contract(3),
		},
		testutil.U64(18446744073709551615),
		true,
	)
	inner, innerResult, meta := testutil.NFTSource(t, event)
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
	transaction := decodeNFT(t, envelope, result, meta)
	if len(transaction.NFTTransfers) != 1 ||
		transaction.NFTTransfers[0].From != testutil.Address(2) ||
		transaction.NFTTransfers[0].To != testutil.Contract(3) ||
		transaction.SourceAccount != testutil.Address(
			1,
		) || transaction.FeeAccount != testutil.Address(9) {
		t.Fatal("NFT owner was inferred from the sponsor or operation source")
	}

	expected := transaction.NFTTransfers
	transaction.NFTTransfers = nil
	if err := decoder.RebuildNFTActivity(&transaction); err != nil ||
		!reflect.DeepEqual(expected, transaction.NFTTransfers) {
		t.Fatal("fee-bump NFT source failed deterministic reconstruction", err)
	}
}
