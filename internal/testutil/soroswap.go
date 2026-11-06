package testutil

import (
	"crypto/sha256"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const SoroswapRouter = "CCJUD55AG6W5HAI5LRVNKAE5WDP5XGZBUDS5WNTIVDU7O264UZZE7BRD"

// The pool vector was computed independently with Python struct/hashlib from
// the official library-0.3.0 deployment recipe for native and USD:Address(3).
const SoroswapPool = "CDZL2FR7LYCL5ILCME4LSNUYREETR3U657AIAL53TDFSUF4JM5TIKNR6"

func Vec(values ...xdr.ScVal) xdr.ScVal {
	items := xdr.ScVec(values)
	ref := &items

	return xdr.ScVal{
		Type: xdr.ScValTypeScvVec,
		Vec:  &ref,
	}
}

func AddressValue(encoded string) xdr.ScVal {
	var address xdr.ScAddress
	if len(encoded) > 0 && encoded[0] == 'G' {
		account := xdr.MustAddress(encoded)
		address = xdr.ScAddress{
			Type:      xdr.ScAddressTypeScAddressTypeAccount,
			AccountId: &account,
		}
	} else {
		address = ContractAddress(encoded)
	}

	return xdr.ScVal{
		Type:    xdr.ScValTypeScvAddress,
		Address: &address,
	}
}

func SoroswapSource(
	t *testing.T,
	exactOutput bool,
) (xdr.TransactionEnvelope, xdr.TransactionResultPair, xdr.TransactionMeta) {
	t.Helper()
	assets, err := xdr.BuildAssets("USD:" + Address(3))
	if err != nil {
		t.Fatal(err)
	}

	native, usd := xdr.MustNewNativeAsset(), assets[0]
	path := Vec(AddressValue(SACContract(native)), AddressValue(SACContract(usd)))
	deadline := xdr.Uint64(2000000000)
	actor := AddressValue(Address(2))
	envelope := InvocationEnvelope("swap_exact_tokens_for_tokens")
	call := envelope.V1.Tx.Operations[0].Body.InvokeHostFunctionOp.HostFunction.InvokeContract
	call.ContractAddress = ContractAddress(SoroswapRouter)
	call.Args = []xdr.ScVal{
		I128(0, 20000000),
		I128(0, 9000000),
		path,
		actor,
		{Type: xdr.ScValTypeScvU64, U64: &deadline},
	}
	if exactOutput {
		call.FunctionName = "swap_tokens_for_exact_tokens"
		call.Args[0] = I128(0, 10000000)
		call.Args[1] = I128(0, 30000000)
	}

	returned := Vec(I128(0, 20000000), I128(0, 10000000))
	routerID := *ContractAddress(SoroswapRouter).ContractId
	events := []xdr.ContractEvent{
		TokenEvent(SACContract(native), "transfer", []string{Address(2), SoroswapPool},
			I128(0, 20000000), String(native.StringCanonical())),
		TokenEvent(SACContract(usd), "transfer", []string{SoroswapPool, Address(2)},
			I128(0, 10000000), String(usd.StringCanonical())),
		{
			ContractId: &routerID,
			Type:       xdr.ContractEventTypeContract,
			Body: xdr.ContractEventBody{
				V: 0,
				V0: &xdr.ContractEventV0{
					Topics: []xdr.ScVal{String("SoroswapRouter"), Symbol("swap")},
					Data: Map(xdr.ScMap{
						{Key: Symbol("amounts"), Val: Vec(I128(0, 20000000), I128(0, 10000000))},
						{Key: Symbol("path"), Val: path},
						{Key: Symbol("to"), Val: actor},
					}),
				},
			},
		},
	}
	meta := xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			Operations:  []xdr.OperationMetaV2{{Events: events}},
			SorobanMeta: &xdr.SorobanTransactionMetaV2{ReturnValue: &returned},
		},
	}
	result := Result(envelope, true)
	SetSoroswapCommitment(t, &result, meta)

	return envelope, result, meta
}

func SetSoroswapCommitment(
	t *testing.T,
	result *xdr.TransactionResultPair,
	meta xdr.TransactionMeta,
) {
	t.Helper()
	var events []xdr.ContractEvent
	var returned xdr.ScVal
	switch meta.V {
	case 3:
		events = meta.V3.SorobanMeta.Events
		returned = meta.V3.SorobanMeta.ReturnValue
	case 4:
		events = meta.V4.Operations[0].Events
		returned = *meta.V4.SorobanMeta.ReturnValue
	default:
		t.Fatal("Soroswap test requires metadata v3 or v4")
	}

	preimage := xdr.InvokeHostFunctionSuccessPreImage{
		ReturnValue: returned,
		Events:      events,
	}
	raw, err := preimage.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	hash := xdr.Hash(sha256.Sum256(raw))
	operation := InvocationResult(xdr.InvokeHostFunctionResultCodeInvokeHostFunctionSuccess)
	operation.Tr.InvokeHostFunctionResult.Success = &hash
	results := []xdr.OperationResult{operation}
	result.Result.Result.Results = &results
}

func SoroswapBundle(t *testing.T, sequence uint32) domain.Bundle {
	t.Helper()
	envelope, result, meta := SoroswapSource(t, false)
	envelope.V1.Tx.SeqNum = xdr.SequenceNumber(sequence)
	identity := Result(envelope, true)
	result.TransactionHash = identity.TransactionHash
	ledger := Ledger(sequence)
	transaction, err := decoder.Transaction(envelope, result, meta, ledger, 1)
	if err != nil {
		t.Fatal(err)
	}

	ledger.TransactionCount = 1

	return domain.Bundle{
		Ledger:       ledger,
		Transactions: []domain.Transaction{transaction},
	}
}
