package decoder_test

import (
	"crypto/sha256"
	"reflect"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestNFTRealTestnetSourcesKeepExactEventsAndResultCommitments(t *testing.T) {
	fixtures := testutil.NFTFixtures(t)
	transactions := testutil.RealNFTTransactions(t)
	if len(fixtures) != 6 || len(transactions) != len(fixtures) {
		t.Fatal("unexpected retained NFT source corpus")
	}

	formats := map[string]int{}
	kinds := map[string]int{}
	mixedFungibleActivity := false
	contractRecipient := false
	zeroIdentifier := false
	for index, fixture := range fixtures {
		t.Run(fixture.Hash, func(t *testing.T) {
			transaction := transactions[index]
			expected := append(transaction.NFTTransfers[:0:0], fixture.Expected...)
			for movementIndex := range expected {
				movement := &expected[movementIndex]
				movement.TransactionHash = fixture.Hash
				movement.LedgerSequence = fixture.Ledger.Sequence
				movement.TxIndex = fixture.TxIndex
				movement.CreatedAt = fixture.Ledger.ClosedAt
				movement.Verification = "event_shape"
				formats[movement.Format]++
				kinds[movement.Kind]++
				contractRecipient = contractRecipient ||
					(len(movement.To) > 0 && movement.To[0] == 'C')
				zeroIdentifier = zeroIdentifier || movement.TokenID == "0"
			}

			if !reflect.DeepEqual(transaction.NFTTransfers, expected) {
				t.Fatalf(
					"real NFT source changed: got %+v want %+v",
					transaction.NFTTransfers,
					expected,
				)
			}

			mixedFungibleActivity = mixedFungibleActivity || len(transaction.TokenTransfers) > 0
			var result xdr.TransactionResult
			var meta xdr.TransactionMeta
			if err := xdr.SafeUnmarshalBase64(fixture.ResultXDR, &result); err != nil {
				t.Fatal(err)
			}

			if err := xdr.SafeUnmarshalBase64(fixture.MetaXDR, &meta); err != nil {
				t.Fatal(err)
			}

			operations, valid := result.OperationResults()
			if !result.Successful() || !valid || meta.V != 4 || meta.V4.SorobanMeta == nil ||
				meta.V4.SorobanMeta.ReturnValue == nil {
				t.Fatal("retained NFT fixture has no successful invocation source")
			}

			for _, movement := range expected {
				operation := operations[movement.OpIndex]
				if operation.Code != xdr.OperationResultCodeOpInner || operation.Tr == nil ||
					operation.Tr.Type != xdr.OperationTypeInvokeHostFunction {
					t.Fatal("NFT fixture is not committed by a host invocation result")
				}

				preimage := xdr.InvokeHostFunctionSuccessPreImage{
					ReturnValue: *meta.V4.SorobanMeta.ReturnValue,
					Events:      meta.V4.Operations[movement.OpIndex].Events,
				}
				raw, err := preimage.MarshalBinary()
				if err != nil {
					t.Fatal(err)
				}

				hash := sha256.Sum256(raw)
				invocation := operation.Tr.MustInvokeHostFunctionResult()
				if invocation.Code != xdr.InvokeHostFunctionResultCodeInvokeHostFunctionSuccess ||
					invocation.MustSuccess() != xdr.Hash(hash) {
					t.Fatal(
						"retained NFT event/return source does not match its success commitment",
					)
				}
			}

			transaction.Events = nil
			transaction.Calls = nil
			transaction.TokenTransfers = nil
			transaction.NFTTransfers = nil
			if err := decoder.RebuildNFTActivity(&transaction); err != nil ||
				!reflect.DeepEqual(transaction.NFTTransfers, expected) {
				t.Fatal("real NFT rebuild required saved derived activity", err)
			}

			// Retain the actual result and alter only source metadata, without recomputing its hash.
			eventIndex := 0
			for operation := range meta.V4.Operations {
				for event := range meta.V4.Operations[operation].Events {
					for _, movement := range expected {
						if movement.EventIndex != eventIndex {
							continue
						}

						body := meta.V4.Operations[operation].Events[event].Body.V0
						if entries, mapped := body.Data.GetMap(); mapped {
							(*entries)[0].Val = testutil.U32(123456)
						} else {
							body.Data = testutil.U32(123456)
						}
					}

					eventIndex++
				}
			}
			var err error
			transaction.MetaXDR, err = xdr.MarshalBase64(meta)
			if err != nil {
				t.Fatal(err)
			}

			if err = decoder.RebuildNFTActivity(&transaction); err != nil ||
				len(transaction.NFTTransfers) != 0 {
				t.Fatal("uncommitted real NFT source metadata created an NFT movement", err)
			}
		})
	}

	if formats["scalar"] != 3 || formats["token_id_map"] != 3 ||
		kinds["transfer"] != 2 || kinds["mint"] != 4 || !mixedFungibleActivity ||
		!contractRecipient || !zeroIdentifier {
		t.Fatal(
			"real NFT corpus lost scalar/map, transfer/mint, contract or mixed-activity coverage",
		)
	}
}
