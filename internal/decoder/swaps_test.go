package decoder_test

import (
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestPathPaymentSwapUsesExecutedAmounts(t *testing.T) {
	for _, strictSend := range []bool{false, true} {
		envelope, result := testutil.PathPaymentSource(t, strictSend)
		transaction, err := decoder.Transaction(envelope, result,
			xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{}}, testutil.Ledger(10), 1)
		if err != nil {
			t.Fatal(err)
		}

		action := transaction.Interpretation.Actions[0]
		exchange := action.PathPayment.Execution
		if action.Kind != "swap" || action.Status != "applied" ||
			action.From != testutil.Address(1) || action.To != testutil.Address(2) ||
			exchange == nil || exchange.Input.Amount != "20000000" ||
			exchange.Output.Amount != "10000000" || *exchange.Input.Asset.Code != "XLM" ||
			*exchange.Output.Asset.Code != "USD" || len(exchange.Trades) != 1 ||
			exchange.Trades[0].Venue != "order_book" || exchange.Trades[0].ClaimIndex != 0 {
			t.Fatal("swap invented amounts from limits or lost result evidence", action)
		}
	}
}

func TestSwapReconcilesMultipleHopsAndPoolClaims(t *testing.T) {
	envelope, result := testutil.PathPaymentSource(t, false)
	executed := result.Result.Result.Results
	success := (*executed)[0].Tr.PathPaymentStrictReceiveResult.Success
	first := *success.Offers[0].OrderBook
	assets, err := xdr.BuildAssets("EUR:" + testutil.Address(5))
	if err != nil {
		t.Fatal(err)
	}

	middle := assets[0]
	success.Offers = []xdr.ClaimAtom{
		{
			Type: xdr.ClaimAtomTypeClaimAtomTypeV0,
			V0: &xdr.ClaimOfferAtomV0{
				SellerEd25519: xdr.Uint256{4},
				AssetBought:   first.AssetBought,
				AmountBought:  20000000,
				AssetSold:     middle,
				AmountSold:    40000000,
			},
		},
		{
			Type: xdr.ClaimAtomTypeClaimAtomTypeLiquidityPool,
			LiquidityPool: &xdr.ClaimLiquidityAtom{
				LiquidityPoolId: xdr.PoolId{6},
				AssetBought:     middle,
				AmountBought:    40000000,
				AssetSold:       first.AssetSold,
				AmountSold:      10000000,
			},
		},
	}
	transaction, err := decoder.Transaction(envelope, result,
		xdr.TransactionMeta{V: 2, V2: &xdr.TransactionMetaV2{}}, testutil.Ledger(10), 1)
	if err != nil {
		t.Fatal(err)
	}

	exchange := transaction.Interpretation.Actions[0].PathPayment.Execution
	if exchange == nil || exchange.Input.Amount != "20000000" ||
		exchange.Output.Amount != "10000000" || len(exchange.Trades) != 2 ||
		exchange.Trades[1].Venue != "liquidity_pool" {
		t.Fatal("intermediate amounts were counted as source input", exchange)
	}

	success.Offers[1].LiquidityPool.AmountBought--
	transaction, err = decoder.Transaction(envelope, result,
		xdr.TransactionMeta{V: 2, V2: &xdr.TransactionMetaV2{}}, testutil.Ledger(10), 1)
	if err != nil || transaction.Interpretation.Actions[0].Kind == "swap" ||
		transaction.Interpretation.Actions[0].PathPayment.Execution != nil {
		t.Fatal("unbalanced intermediate asset was described as a verified swap", err)
	}
}

func TestSwapNeverClaimsRolledBackOrUnreconciledExecution(t *testing.T) {
	cases := []struct {
		name  string
		alter func(*xdr.TransactionEnvelope, *xdr.TransactionResultPair)
	}{
		{"rolled_back", func(_ *xdr.TransactionEnvelope, result *xdr.TransactionResultPair) {
			result.Result.Result.Code = xdr.TransactionResultCodeTxFailed
		}},
		{"missing_claims", func(_ *xdr.TransactionEnvelope, result *xdr.TransactionResultPair) {
			(*result.Result.Result.Results)[0].Tr.PathPaymentStrictReceiveResult.Success.Offers = nil
		}},
		{"wrong_destination", func(_ *xdr.TransactionEnvelope, result *xdr.TransactionResultPair) {
			(*result.Result.Result.Results)[0].Tr.PathPaymentStrictReceiveResult.Success.Last.Destination = xdr.MustAddress(
				testutil.Address(7),
			)
		}},
		{"above_send_max", func(envelope *xdr.TransactionEnvelope, _ *xdr.TransactionResultPair) {
			envelope.V1.Tx.Operations[0].Body.PathPaymentStrictReceiveOp.SendMax = 10000000
		}},
		{"wrong_output", func(_ *xdr.TransactionEnvelope, result *xdr.TransactionResultPair) {
			(*result.Result.Result.Results)[0].Tr.PathPaymentStrictReceiveResult.Success.Last.Amount--
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			envelope, result := testutil.PathPaymentSource(t, false)
			test.alter(&envelope, &result)
			updated := testutil.Result(envelope, result.Result.Successful())
			result.TransactionHash = updated.TransactionHash
			transaction, err := decoder.Transaction(envelope, result,
				xdr.TransactionMeta{V: 2, V2: &xdr.TransactionMetaV2{}}, testutil.Ledger(10), 1)
			if err != nil || transaction.Interpretation.Actions[0].Kind == "swap" ||
				transaction.Interpretation.Actions[0].PathPayment.Execution != nil {
				t.Fatal("uncommitted or unverifiable swap was shown as executed", err)
			}
		})
	}
}

func TestSameAssetPathPaymentDoesNotInventConversion(t *testing.T) {
	envelope, result := testutil.PathPaymentSource(t, false)
	payment := envelope.V1.Tx.Operations[0].Body.PathPaymentStrictReceiveOp
	payment.DestAsset = payment.SendAsset
	success := (*result.Result.Result.Results)[0].Tr.PathPaymentStrictReceiveResult.Success
	success.Last.Asset = payment.SendAsset
	success.Offers = nil
	updated := testutil.Result(envelope, true)
	result.TransactionHash = updated.TransactionHash
	transaction, err := decoder.Transaction(envelope, result,
		xdr.TransactionMeta{V: 2, V2: &xdr.TransactionMetaV2{}}, testutil.Ledger(10), 1)
	if err != nil {
		t.Fatal(err)
	}

	action := transaction.Interpretation.Actions[0]
	if action.Kind != "path_payment" || action.PathPayment.Execution == nil ||
		action.PathPayment.Execution.Input.Amount != "10000000" ||
		len(action.PathPayment.Execution.Trades) != 0 {
		t.Fatal("same-asset payment gained a conversion", action)
	}
}

func TestSwapNetsRepeatedAssetsAndRejectsInputOverflow(t *testing.T) {
	envelope, result := testutil.PathPaymentSource(t, false)
	success := (*result.Result.Result.Results)[0].Tr.PathPaymentStrictReceiveResult.Success
	last := *success.Offers[0].OrderBook
	assets, err := xdr.BuildAssets("EUR:" + testutil.Address(5))
	if err != nil {
		t.Fatal(err)
	}

	claim := func(input, output xdr.Asset, paid, received int64) xdr.ClaimAtom {
		return xdr.ClaimAtom{
			Type: xdr.ClaimAtomTypeClaimAtomTypeOrderBook,
			OrderBook: &xdr.ClaimOfferAtom{
				SellerId:     last.SellerId,
				AssetBought:  input,
				AmountBought: xdr.Int64(paid),
				AssetSold:    output,
				AmountSold:   xdr.Int64(received),
			},
		}
	}
	success.Offers = []xdr.ClaimAtom{
		claim(last.AssetBought, assets[0], 20000000, 40000000),
		claim(assets[0], last.AssetBought, 40000000, 30000000),
		claim(last.AssetBought, last.AssetSold, 30000000, 10000000),
	}
	transaction, err := decoder.Transaction(envelope, result,
		xdr.TransactionMeta{V: 2, V2: &xdr.TransactionMetaV2{}}, testutil.Ledger(10), 1)
	if err != nil || transaction.Interpretation.Actions[0].PathPayment.Execution == nil ||
		transaction.Interpretation.Actions[0].PathPayment.Execution.Input.Amount != "20000000" {
		t.Fatal("repeated source asset was added to the initial input", err)
	}

	success.Offers = []xdr.ClaimAtom{
		claim(last.AssetBought, last.AssetSold, 9223372036854775807, 5000000),
		claim(last.AssetBought, last.AssetSold, 1, 5000000),
	}
	transaction, err = decoder.Transaction(envelope, result,
		xdr.TransactionMeta{V: 2, V2: &xdr.TransactionMetaV2{}}, testutil.Ledger(10), 1)
	if err != nil || transaction.Interpretation.Actions[0].PathPayment.Execution != nil {
		t.Fatal("input overflow was wrapped into an executed amount", err)
	}
}

func TestPathPaymentMovementDeduplicationRequiresReconciledExecution(t *testing.T) {
	envelope, result := testutil.PathPaymentSource(t, false)
	payment := envelope.V1.Tx.Operations[0].Body.MustPathPaymentStrictReceiveOp()
	meta := xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			Operations: []xdr.OperationMetaV2{
				{
					Events: []xdr.ContractEvent{
						testutil.TokenEvent(
							testutil.SACContract(payment.SendAsset),
							"transfer",
							[]string{
								testutil.Address(1),
								testutil.Address(4),
							},
							testutil.I128(0, 20000000),
							testutil.String(payment.SendAsset.StringCanonical()),
						),
						testutil.TokenEvent(
							testutil.SACContract(payment.DestAsset),
							"transfer",
							[]string{
								testutil.Address(4),
								testutil.Address(2),
							},
							testutil.I128(0, 10000000),
							testutil.String(payment.DestAsset.StringCanonical()),
						),
					},
				},
			},
		},
	}
	transaction, err := decoder.Transaction(envelope, result, meta, testutil.Ledger(10), 1)
	if err != nil || len(transaction.Interpretation.Actions) != 1 ||
		transaction.Interpretation.Actions[0].Kind != "swap" ||
		len(transaction.Interpretation.Actions[0].EventIndexes) != 2 {
		t.Fatal("verified path payment and its movements were counted twice", err)
	}

	(*result.Result.Result.Results)[0].Tr.PathPaymentStrictReceiveResult.Success.Offers = nil
	transaction, err = decoder.Transaction(envelope, result, meta, testutil.Ledger(10), 1)
	if err != nil || len(transaction.Interpretation.Actions) != 3 ||
		transaction.Interpretation.Actions[0].Kind != "path_payment" ||
		transaction.Interpretation.Actions[1].Kind != "transfer" {
		t.Fatal("unreconciled path payment hid committed token evidence", err)
	}
}
