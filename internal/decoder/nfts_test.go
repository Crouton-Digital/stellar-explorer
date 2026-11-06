package decoder_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestNFTUnsignedIdentifiersRemainExactAcrossSupportedFormats(t *testing.T) {
	for _, identifier := range []struct {
		name     string
		value    xdr.ScVal
		expected string
		kind     string
	}{
		{
			name:     "zero",
			value:    testutil.U32(0),
			expected: "0",
			kind:     "u32",
		},
		{
			name:     "u32_maximum",
			value:    testutil.U32(4294967295),
			expected: "4294967295",
			kind:     "u32",
		},
		{
			name:     "u64_maximum",
			value:    testutil.U64(18446744073709551615),
			expected: "18446744073709551615",
			kind:     "u64",
		},
		{
			name:     "u128_maximum",
			value:    testutil.U128(18446744073709551615, 18446744073709551615),
			expected: "340282366920938463463374607431768211455",
			kind:     "u128",
		},
		{
			name: "u256_maximum",
			value: testutil.U256(18446744073709551615, 18446744073709551615,
				18446744073709551615, 18446744073709551615),
			expected: "115792089237316195423570985008687907853269984665640564039457584007913129639935",
			kind:     "u256",
		},
	} {
		for _, mapped := range []bool{false, true} {
			for _, version := range []int32{3, 4} {
				name := fmt.Sprintf("%s_map_%t_metadata_%d", identifier.name, mapped, version)
				t.Run(name, func(t *testing.T) {
					event := testutil.NFTEvent(
						testutil.Contract(30),
						"transfer",
						[]string{
							testutil.Address(2),
							testutil.Contract(3),
						},
						identifier.value,
						mapped,
					)
					envelope, result, meta := testutil.NFTSource(t, event)
					if version == 3 {
						meta = aquariusMetadataV3(meta)
					}

					transaction := decodeNFT(t, envelope, result, meta)
					if len(transaction.NFTTransfers) != 1 || len(transaction.TokenTransfers) != 0 {
						t.Fatal("NFT identifier was lost or interpreted as a fungible amount")
					}

					movement := transaction.NFTTransfers[0]
					format := "scalar"
					if mapped {
						format = "token_id_map"
					}

					if movement.TokenID != identifier.expected ||
						movement.TokenIDType != identifier.kind ||
						movement.From != testutil.Address(2) ||
						movement.To != testutil.Contract(3) ||
						movement.ContractID != testutil.Contract(30) ||
						movement.EventIndex != 0 ||
						movement.OpIndex != 0 ||
						movement.TransactionHash != transaction.Hash ||
						movement.LedgerSequence != transaction.LedgerSequence ||
						movement.TxIndex != transaction.TxIndex ||
						movement.CreatedAt != transaction.CreatedAt ||
						movement.Kind != "transfer" ||
						movement.Format != format ||
						movement.EventStandard != "sep50" ||
						movement.Verification != "event_shape" {
						t.Fatal(
							"NFT identity, exact ID or committed provenance was altered",
							movement,
						)
					}

					if !slices.Contains(transaction.Participants, testutil.Address(2)) ||
						slices.Contains(transaction.Participants, testutil.Contract(3)) {
						t.Fatal("NFT participants did not retain only genuine account IDs")
					}
				})
			}
		}
	}
}

func TestNFTMintTransferAndBurnKeepTheirDistinctMeaning(t *testing.T) {
	collection := testutil.Contract(30)
	events := []xdr.ContractEvent{
		testutil.NFTEvent(collection, "mint", []string{testutil.Address(2)}, testutil.U32(0), true),
		testutil.NFTEvent(
			collection,
			"transfer",
			[]string{testutil.Address(2), testutil.Contract(3)},
			testutil.U32(0),
			false,
		),
		testutil.NFTEvent(
			collection,
			"burn",
			[]string{testutil.Contract(3)},
			testutil.U32(0),
			true,
		),
	}
	transaction := testutil.NFTBundle(t, 10, events...).Transactions[0]
	if len(transaction.NFTTransfers) != 3 || len(transaction.TokenTransfers) != 0 {
		t.Fatal("NFT movements overlapped fungible transfer accounting")
	}

	mint, transfer, burn := transaction.NFTTransfers[0], transaction.NFTTransfers[1], transaction.NFTTransfers[2]
	if mint.Kind != "mint" || mint.From != "" || mint.To != testutil.Address(2) ||
		mint.TokenID != "0" || mint.EventStandard != "sep50" ||
		transfer.Kind != "transfer" || transfer.EventIndex != 1 ||
		burn.Kind != "burn" || burn.From != testutil.Contract(3) || burn.To != "" ||
		burn.EventStandard != "openzeppelin_burnable" || burn.EventIndex != 2 {
		t.Fatal("mint/burn invented zero-address transfers or conflated extension provenance")
	}

	// A valid all-zero account address is an address, not an Ethereum-style mint sentinel.
	zero := testutil.Address(0)
	self := testutil.NFTEvent(collection, "transfer", []string{zero, zero}, testutil.U32(0), false)
	transaction = testutil.NFTBundle(t, 11, self).Transactions[0]
	if len(transaction.NFTTransfers) != 1 || transaction.NFTTransfers[0].Kind != "transfer" ||
		transaction.NFTTransfers[0].From != zero || transaction.NFTTransfers[0].To != zero {
		t.Fatal("self-transfer or a real zero-valued account was misclassified as mint/burn")
	}
}

func TestNFTConstructorMintDoesNotRequireAMintEntryPoint(t *testing.T) {
	collection := "CCHRSGODYCFJQZVGMLL3DA2OQ46PWFFJAXAT6FVAI7KBD4SLNPP7YROQ"
	event := testutil.NFTEvent(
		collection,
		"mint",
		[]string{testutil.Address(2)},
		testutil.U32(0),
		true,
	)
	envelope, result, meta := testutil.NFTSource(t, event)
	creator := xdr.MustAddress(testutil.Address(7))
	envelope.V1.Tx.Operations[0].Body.InvokeHostFunctionOp.HostFunction = xdr.HostFunction{
		Type: xdr.HostFunctionTypeHostFunctionTypeCreateContractV2,
		CreateContractV2: &xdr.CreateContractArgsV2{
			ContractIdPreimage: xdr.ContractIdPreimage{
				Type: xdr.ContractIdPreimageTypeContractIdPreimageFromAddress,
				FromAddress: &xdr.ContractIdPreimageFromAddress{
					Address: xdr.ScAddress{
						Type:      xdr.ScAddressTypeScAddressTypeAccount,
						AccountId: &creator,
					},
				},
			},
			Executable:      testutil.WasmExecutable(1),
			ConstructorArgs: []xdr.ScVal{testutil.AddressValue(testutil.Address(2))},
		},
	}
	returned := testutil.AddressValue(collection)
	meta.V4.SorobanMeta.ReturnValue = &returned
	testutil.NFTCommitment(t, &result, meta)
	transaction := decodeNFT(t, envelope, result, meta)
	if len(transaction.NFTTransfers) != 1 || transaction.NFTTransfers[0].Kind != "mint" ||
		transaction.NFTTransfers[0].ContractID != collection ||
		transaction.NFTTransfers[0].To != testutil.Address(2) ||
		len(transaction.Calls) != 0 {
		t.Fatal("committed constructor mint was lost or represented as a called mint method")
	}
}

func TestNFTAndFungibleActivityShareRawEventOrderWithoutSharingAmounts(t *testing.T) {
	asset := xdr.MustNewNativeAsset()
	events := []xdr.ContractEvent{
		testutil.TokenEvent(
			testutil.SACContract(asset),
			"transfer",
			[]string{
				testutil.Address(4),
				testutil.Address(5),
			},
			testutil.I128(0, 7),
			testutil.String("native"),
		),
		testutil.NFTEvent(testutil.Contract(30), "transfer",
			[]string{testutil.Address(2), testutil.Address(3)}, testutil.U32(7), false),
	}
	transaction := testutil.NFTBundle(t, 10, events...).Transactions[0]
	if len(transaction.TokenTransfers) != 1 || transaction.TokenTransfers[0].EventIndex != 0 ||
		len(transaction.NFTTransfers) != 1 || transaction.NFTTransfers[0].EventIndex != 1 ||
		transaction.TokenTransfers[0].Amount != transaction.NFTTransfers[0].TokenID {
		t.Fatal("same numeric value confused the separate fungible and NFT event types")
	}

	if !slices.Contains(transaction.Participants, testutil.Address(3)) ||
		!slices.Contains(transaction.Participants, testutil.Address(5)) {
		t.Fatal("live ingestion omitted NFT or fungible account participants")
	}
}

func TestNFTRebuildUsesOnlyValidatedSourceAndPreservesOtherPayload(t *testing.T) {
	transaction := testutil.NFTBundle(t, 10).Transactions[0]
	expected := append([]domain.NFTTransfer{}, transaction.NFTTransfers...)
	transaction.Calls = nil
	transaction.Events = nil
	transaction.TokenTransfers = nil
	transaction.Contracts = nil
	transaction.Participants = []string{testutil.Address(8)}
	transaction.ReturnValue = "untrusted saved value"
	transaction.NFTTransfers = nil
	before := transaction
	if err := decoder.RebuildNFTActivity(&transaction); err != nil ||
		!reflect.DeepEqual(transaction.NFTTransfers, expected) {
		t.Fatal("NFT rebuild trusted saved events or failed to recover raw source", err)
	}

	transaction.NFTTransfers = nil
	if !reflect.DeepEqual(before, transaction) {
		t.Fatal("NFT rebuild changed participants, interpretation or other derived payload")
	}

	for _, test := range []struct {
		name  string
		alter func(*domain.Transaction)
	}{
		{
			name: "network",
			alter: func(transaction *domain.Transaction) {
				transaction.Network = "mainnet"
			},
		},
		{
			name: "envelope_xdr",
			alter: func(transaction *domain.Transaction) {
				transaction.EnvelopeXDR = "invalid"
			},
		},
		{
			name: "result_xdr",
			alter: func(transaction *domain.Transaction) {
				transaction.ResultXDR = "invalid"
			},
		},
		{
			name: "meta_xdr",
			alter: func(transaction *domain.Transaction) {
				transaction.MetaXDR = "invalid"
			},
		},
		{
			name: "hash",
			alter: func(transaction *domain.Transaction) {
				transaction.Hash = strings.Repeat("0", 64)
			},
		},
		{
			name: "successful",
			alter: func(transaction *domain.Transaction) {
				transaction.Successful = false
			},
		},
		{
			name: "source_account",
			alter: func(transaction *domain.Transaction) {
				transaction.SourceAccount = testutil.Address(9)
			},
		},
		{
			name: "fee_account",
			alter: func(transaction *domain.Transaction) {
				transaction.FeeAccount = testutil.Address(9)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			broken := before
			broken.NFTTransfers = expected
			test.alter(&broken)
			snapshot, err := json.Marshal(broken)
			if err != nil {
				t.Fatal(err)
			}

			if err = decoder.RebuildNFTActivity(&broken); err == nil {
				t.Fatal("NFT rebuild accepted corrupt transaction source")
			}

			after, err := json.Marshal(broken)
			if err != nil || string(snapshot) != string(after) {
				t.Fatal("rejected source changed a retained transaction", err)
			}
		})
	}
}

func TestNFTCommitmentSeparatesSystemDiagnosticsAndTransactionFees(t *testing.T) {
	envelope, result, meta := testutil.NFTSource(t)
	nft := meta.V4.Operations[0].Events[0]
	system := xdr.ContractEvent{
		Type: xdr.ContractEventTypeSystem,
		Body: xdr.ContractEventBody{
			V: 0,
			V0: &xdr.ContractEventV0{
				Data: xdr.ScVal{Type: xdr.ScValTypeScvVoid},
			},
		},
	}
	meta.V4.Operations[0].Events = []xdr.ContractEvent{system, nft}
	meta.V4.DiagnosticEvents = []xdr.DiagnosticEvent{
		{InSuccessfulContractCall: true, Event: nft},
	}
	meta.V4.Events = []xdr.TransactionEvent{
		testutil.FeeEvent(testutil.Address(1), 100,
			xdr.TransactionEventStageTransactionEventStageBeforeAllTxs),
		{
			Stage: xdr.TransactionEventStageTransactionEventStageAfterAllTxs,
			Event: nft,
		},
	}
	testutil.NFTCommitment(t, &result, meta)
	transaction := decodeNFT(t, envelope, result, meta)
	if len(transaction.NFTTransfers) != 1 || transaction.NFTTransfers[0].EventIndex != 1 ||
		len(transaction.Events) != 4 || len(transaction.TokenTransfers) != 0 {
		t.Fatal("system, diagnostic or transaction-scoped NFT-shaped data created a movement")
	}

	meta.V4.Operations[0].Events[0].Body.V0.Data = testutil.U32(1)
	transaction = decodeNFT(t, envelope, result, meta)
	if len(transaction.NFTTransfers) != 0 {
		t.Fatal("NFT result commitment ignored a changed system event")
	}
}

func TestNFTFailureAndMissingCommitmentDoNotCreateMovements(t *testing.T) {
	for _, scenario := range []string{"failed", "wrong_hash", "diagnostics_only", "missing_return"} {
		t.Run(scenario, func(t *testing.T) {
			envelope, result, meta := testutil.NFTSource(t)
			switch scenario {
			case "failed":
				result.Result.Result.Code = xdr.TransactionResultCodeTxFailed
			case "wrong_hash":
				hash := (*result.Result.Result.Results)[0].Tr.InvokeHostFunctionResult.Success
				hash[0] ^= 1
			case "diagnostics_only":
				meta.V4.DiagnosticEvents = []xdr.DiagnosticEvent{
					{InSuccessfulContractCall: true, Event: meta.V4.Operations[0].Events[0]},
				}
				meta.V4.Operations[0].Events = nil
				testutil.NFTCommitment(t, &result, meta)
			case "missing_return":
				meta.V4.SorobanMeta.ReturnValue = nil
			}

			transaction := decodeNFT(t, envelope, result, meta)
			if len(transaction.NFTTransfers) != 0 {
				t.Fatal("uncommitted or failed NFT event changed the movement history")
			}
		})
	}
}

func decodeNFT(
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
