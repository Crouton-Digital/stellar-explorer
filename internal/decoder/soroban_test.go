package decoder_test

import (
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestSorobanV3V4ReturnAndCommittedEvents(t *testing.T) {
	contract := xdr.ContractId{1}
	value := xdr.ScVal{
		Type: xdr.ScValTypeScvU64,
		U64:  new(xdr.Uint64),
	}
	*value.U64 = 18446744073709551615
	env := testutil.PaymentEnvelope()
	env.V1.Tx.Operations = []xdr.Operation{
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
							FunctionName: "hello",
							Args:         []xdr.ScVal{value},
						},
					},
				},
			},
		},
	}
	event := xdr.ContractEvent{
		ContractId: &contract,
		Type:       xdr.ContractEventTypeContract,
		Body: xdr.ContractEventBody{
			V: 0,
			V0: &xdr.ContractEventV0{
				Data: value,
			},
		},
	}
	metas := []xdr.TransactionMeta{

		{
			V: 3,
			V3: &xdr.TransactionMetaV3{
				SorobanMeta: &xdr.SorobanTransactionMeta{
					ReturnValue: value,
					Events:      []xdr.ContractEvent{event},
					DiagnosticEvents: []xdr.DiagnosticEvent{
						{
							InSuccessfulContractCall: true,
							Event:                    event,
						},
					},
				},
			},
		},

		{
			V: 4,
			V4: &xdr.TransactionMetaV4{
				SorobanMeta: &xdr.SorobanTransactionMetaV2{
					ReturnValue: &value,
				},
				Operations: []xdr.OperationMetaV2{
					{
						Events: []xdr.ContractEvent{event},
					},
				},
				DiagnosticEvents: []xdr.DiagnosticEvent{
					{
						InSuccessfulContractCall: true,
						Event:                    event,
					},
				},
			},
		},
	}

	for _, meta := range metas {
		tx, e := decoder.Transaction(env, testutil.Result(env, true), meta, testutil.Ledger(10), 1)
		if e != nil {
			t.Fatal(e)
		}

		if len(tx.Calls) != 1 || tx.Calls[0].FunctionName != "hello" || len(tx.Events) != 1 {
			t.Fatalf("incorrect calls/events for meta v%d", meta.V)
		}

		if tx.ReturnValue.(map[string]any)["value"] != "18446744073709551615" {
			t.Fatal("return precision lost")
		}

		if tx.Diagnostics.(map[string]any)["availability"] != "provided" {
			t.Fatal("diagnostics missing")
		}

		tx, e = decoder.Transaction(env, testutil.Result(env, false), meta, testutil.Ledger(10), 1)
		if e != nil {
			t.Fatal(e)
		}

		if len(tx.Events) != 0 || tx.ReturnValue != nil {
			t.Fatal("failed tx exposed committed events/return")
		}
	}
}

func TestEnvelopeOrderDoesNotReplaceApplicationOrder(t *testing.T) {
	first, second := testutil.PaymentEnvelope(), testutil.PaymentEnvelope()
	second.V1.Tx.SeqNum = 2
	r := testutil.Remote(10, []xdr.TransactionEnvelope{first, second})
	var meta xdr.LedgerCloseMeta
	if e := xdr.SafeUnmarshalBase64(r.MetadataXDR, &meta); e != nil {
		t.Fatal(e)
	}

	meta.V0.TxSet.Txs[0], meta.V0.TxSet.Txs[1] = meta.V0.TxSet.Txs[1], meta.V0.TxSet.Txs[0]
	r.MetadataXDR, _ = xdr.MarshalBase64(meta)
	b, e := decoder.Ledger(r)
	if e != nil {
		t.Fatal(e)
	}

	var decoded xdr.TransactionEnvelope
	if e = xdr.SafeUnmarshalBase64(b.Transactions[0].EnvelopeXDR, &decoded); e != nil {
		t.Fatal(e)
	}

	if decoded.SeqNum() != 1 || b.Transactions[0].TxIndex != 1 {
		t.Fatal("txSet order replaced application order")
	}
}
