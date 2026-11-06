package testutil

import (
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func InvocationEnvelope(function string) xdr.TransactionEnvelope {
	envelope := PaymentEnvelope()
	contract := xdr.ContractId{9}
	envelope.V1.Tx.Operations = []xdr.Operation{
		{
			Body: xdr.OperationBody{
				Type: xdr.OperationTypeInvokeHostFunction,
				InvokeHostFunctionOp: &xdr.InvokeHostFunctionOp{
					HostFunction: xdr.HostFunction{
						Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
						InvokeContract: &xdr.InvokeContractArgs{
							ContractAddress: xdr.ScAddress{
								Type:       xdr.ScAddressTypeScAddressTypeContract,
								ContractId: &contract,
							},
							FunctionName: xdr.ScSymbol(function),
						},
					},
				},
			},
		},
	}

	return envelope
}

func PaymentResult(code xdr.PaymentResultCode) xdr.OperationResult {
	return xdr.OperationResult{
		Code: xdr.OperationResultCodeOpInner,
		Tr: &xdr.OperationResultTr{
			Type: xdr.OperationTypePayment,
			PaymentResult: &xdr.PaymentResult{
				Code: code,
			},
		},
	}
}

func InvocationResult(code xdr.InvokeHostFunctionResultCode) xdr.OperationResult {
	result := xdr.InvokeHostFunctionResult{Code: code}
	if code == xdr.InvokeHostFunctionResultCodeInvokeHostFunctionSuccess {
		result.Success = &xdr.Hash{}
	}

	return xdr.OperationResult{
		Code: xdr.OperationResultCodeOpInner,
		Tr: &xdr.OperationResultTr{
			Type:                     xdr.OperationTypeInvokeHostFunction,
			InvokeHostFunctionResult: &result,
		},
	}
}

func ContractError(code uint32) xdr.ScVal {
	value := xdr.Uint32(code)

	return xdr.ScVal{
		Type: xdr.ScValTypeScvError,
		Error: &xdr.ScError{
			Type:         xdr.ScErrorTypeSceContract,
			ContractCode: &value,
		},
	}
}

func InterpretationBundle(t *testing.T, sequence uint32, successful bool) domain.Bundle {
	t.Helper()
	envelope := InvocationEnvelope("transfer")
	envelope.V1.Tx.SeqNum = xdr.SequenceNumber(sequence)
	result := Result(envelope, successful)
	code := xdr.InvokeHostFunctionResultCodeInvokeHostFunctionSuccess
	if !successful {
		code = xdr.InvokeHostFunctionResultCodeInvokeHostFunctionTrapped
	}

	results := []xdr.OperationResult{InvocationResult(code)}
	result.Result.Result.Results = &results
	meta := xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			Operations: []xdr.OperationMetaV2{
				{
					Events: []xdr.ContractEvent{
						TokenEvent(SACContract(xdr.MustNewNativeAsset()), "transfer",
							[]string{Address(1), Address(2)}, I128(0, 10000001), String("native")),
					},
				},
			},
			Events: []xdr.TransactionEvent{
				FeeEvent(Address(1), 100,
					xdr.TransactionEventStageTransactionEventStageBeforeAllTxs),
			},
			DiagnosticEvents: []xdr.DiagnosticEvent{
				{
					InSuccessfulContractCall: successful,
					Event: xdr.ContractEvent{
						Type: xdr.ContractEventTypeDiagnostic,
						Body: xdr.ContractEventBody{
							V: 0,
							V0: &xdr.ContractEventV0{
								Topics: []xdr.ScVal{Symbol("error"), ContractError(7)},
								Data:   xdr.ScVal{Type: xdr.ScValTypeScvVoid},
							},
						},
					},
				},
			},
		},
	}
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
