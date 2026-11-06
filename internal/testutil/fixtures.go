package testutil

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/stellar"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func Address(n byte) string {
	raw := make([]byte, 32)
	raw[0] = n
	v, _ := strkey.Encode(strkey.VersionByteAccountID, raw)

	return v
}

func Contract(n byte) string {
	raw := make([]byte, 32)
	raw[0] = n
	v, _ := strkey.Encode(strkey.VersionByteContract, raw)

	return v
}

func PaymentEnvelope() xdr.TransactionEnvelope {
	return xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				SourceAccount: xdr.MustMuxedAddress(Address(1)),
				Fee:           100,
				SeqNum:        1,
				Cond: xdr.Preconditions{
					Type: xdr.PreconditionTypePrecondNone,
				},
				Memo: xdr.Memo{
					Type: xdr.MemoTypeMemoNone,
				},
				Operations: []xdr.Operation{
					{
						Body: xdr.OperationBody{
							Type: xdr.OperationTypePayment,
							PaymentOp: &xdr.PaymentOp{
								Destination: xdr.MustMuxedAddress(Address(2)),
								Asset: xdr.Asset{
									Type: xdr.AssetTypeAssetTypeNative,
								},
								Amount: 9223372036854775807,
							},
						},
					},
				},
			},
		},
	}
}

func Result(env xdr.TransactionEnvelope, successful bool) xdr.TransactionResultPair {
	h, _ := network.HashTransactionInEnvelope(env, domain.Passphrase)
	code := xdr.TransactionResultCodeTxSuccess
	if !successful {
		code = xdr.TransactionResultCodeTxFailed
	}

	results := []xdr.OperationResult{}

	return xdr.TransactionResultPair{
		TransactionHash: h,
		Result: xdr.TransactionResult{
			FeeCharged: 100,
			Result: xdr.TransactionResultResult{
				Code:    code,
				Results: &results,
			},
		},
	}
}

func Ledger(n uint32) domain.Ledger {
	return domain.Ledger{
		Network:         domain.Network,
		Sequence:        n,
		Hash:            fmt.Sprintf("%064x", n),
		PreviousHash:    fmt.Sprintf("%064x", n-1),
		ClosedAt:        time.Unix(1700000000+int64(n), 0).UTC(),
		ProtocolVersion: 23,
	}
}

func Remote(n uint32, envs []xdr.TransactionEnvelope) stellar.RemoteLedger {
	l := Ledger(n)
	var hash, prev xdr.Hash
	raw, _ := hex.DecodeString(l.Hash)
	copy(hash[:], raw)
	raw, _ = hex.DecodeString(l.PreviousHash)
	copy(prev[:], raw)
	processing := []xdr.TransactionResultMeta{}

	for _, env := range envs {
		processing = append(processing, xdr.TransactionResultMeta{
			Result: Result(env, true),
			TxApplyProcessing: xdr.TransactionMeta{
				V:  2,
				V2: &xdr.TransactionMetaV2{},
			},
		})
	}

	meta := xdr.LedgerCloseMeta{
		V: 0,
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Hash: hash,
				Header: xdr.LedgerHeader{
					LedgerSeq:          xdr.Uint32(n),
					LedgerVersion:      23,
					PreviousLedgerHash: prev,
					ScpValue: xdr.StellarValue{
						CloseTime: xdr.TimePoint(l.ClosedAt.Unix()),
					},
				},
			},
			TxSet: xdr.TransactionSet{
				Txs: envs,
			},
			TxProcessing: processing,
		},
	}
	encoded, e := xdr.MarshalBase64(meta)
	if e != nil {
		panic(e)
	}

	return stellar.RemoteLedger{
		Sequence:    n,
		Hash:        l.Hash,
		MetadataXDR: encoded,
	}
}
