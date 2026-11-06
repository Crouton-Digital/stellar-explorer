package testutil

import (
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func PathPaymentSource(
	t *testing.T,
	strictSend bool,
) (xdr.TransactionEnvelope, xdr.TransactionResultPair) {
	t.Helper()
	assets, err := xdr.BuildAssets("USD:" + Address(3))
	if err != nil {
		t.Fatal(err)
	}

	asset := assets[0]
	claim := xdr.ClaimAtom{
		Type: xdr.ClaimAtomTypeClaimAtomTypeOrderBook,
		OrderBook: &xdr.ClaimOfferAtom{
			SellerId:     xdr.MustAddress(Address(4)),
			OfferId:      1,
			AssetBought:  xdr.MustNewNativeAsset(),
			AmountBought: 20000000,
			AssetSold:    asset,
			AmountSold:   10000000,
		},
	}
	last := xdr.SimplePaymentResult{
		Destination: xdr.MustAddress(Address(2)),
		Asset:       asset,
		Amount:      10000000,
	}
	envelope := PaymentEnvelope()
	body := xdr.OperationBody{
		Type: xdr.OperationTypePathPaymentStrictReceive,
		PathPaymentStrictReceiveOp: &xdr.PathPaymentStrictReceiveOp{
			SendAsset:   xdr.MustNewNativeAsset(),
			SendMax:     30000000,
			Destination: xdr.MustMuxedAddress(Address(2)),
			DestAsset:   asset,
			DestAmount:  10000000,
		},
	}
	operationResult := xdr.OperationResult{
		Code: xdr.OperationResultCodeOpInner,
		Tr: &xdr.OperationResultTr{
			Type: body.Type,
			PathPaymentStrictReceiveResult: &xdr.PathPaymentStrictReceiveResult{
				Code: xdr.PathPaymentStrictReceiveResultCodePathPaymentStrictReceiveSuccess,
				Success: &xdr.PathPaymentStrictReceiveResultSuccess{
					Offers: []xdr.ClaimAtom{claim},
					Last:   last,
				},
			},
		},
	}
	if strictSend {
		body = xdr.OperationBody{
			Type: xdr.OperationTypePathPaymentStrictSend,
			PathPaymentStrictSendOp: &xdr.PathPaymentStrictSendOp{
				SendAsset:   xdr.MustNewNativeAsset(),
				SendAmount:  20000000,
				Destination: xdr.MustMuxedAddress(Address(2)),
				DestAsset:   asset,
				DestMin:     9000000,
			},
		}
		operationResult.Tr = &xdr.OperationResultTr{
			Type: body.Type,
			PathPaymentStrictSendResult: &xdr.PathPaymentStrictSendResult{
				Code: xdr.PathPaymentStrictSendResultCodePathPaymentStrictSendSuccess,
				Success: &xdr.PathPaymentStrictSendResultSuccess{
					Offers: []xdr.ClaimAtom{claim},
					Last:   last,
				},
			},
		}
	}

	envelope.V1.Tx.Operations[0].Body = body
	result := Result(envelope, true)
	results := []xdr.OperationResult{operationResult}
	result.Result.Result.Results = &results

	return envelope, result
}

func SwapBundle(t *testing.T, sequence uint32) domain.Bundle {
	t.Helper()
	envelope, result := PathPaymentSource(t, false)
	envelope.V1.Tx.SeqNum = xdr.SequenceNumber(sequence)
	updated := Result(envelope, true)
	result.TransactionHash = updated.TransactionHash
	ledger := Ledger(sequence)
	transaction, err := decoder.Transaction(envelope, result,
		xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{}}, ledger, 1)
	if err != nil {
		t.Fatal(err)
	}

	ledger.TransactionCount = 1

	return domain.Bundle{
		Ledger:       ledger,
		Transactions: []domain.Transaction{transaction},
	}
}
