package decoder_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestPaymentInterpretationPreservesExactAmountAndFailure(t *testing.T) {
	for _, successful := range []bool{true, false} {
		envelope := testutil.PaymentEnvelope()
		result := testutil.Result(envelope, successful)
		code := xdr.PaymentResultCodePaymentSuccess
		if !successful {
			code = xdr.PaymentResultCodePaymentUnderfunded
		}

		results := []xdr.OperationResult{testutil.PaymentResult(code)}
		result.Result.Result.Results = &results
		transaction, err := decoder.Transaction(envelope, result,
			xdr.TransactionMeta{V: 2, V2: &xdr.TransactionMetaV2{}}, testutil.Ledger(10), 1)
		if err != nil {
			t.Fatal(err)
		}

		action := transaction.Interpretation.Actions[0]
		if action.Kind != "payment" || *action.Amount != "9223372036854775807" ||
			action.From != testutil.Address(1) || action.To != testutil.Address(2) ||
			*action.Asset.Code != "XLM" || *action.Asset.Decimals != 7 {
			t.Fatalf("payment interpretation lost evidence: %+v", action)
		}

		if successful {
			if action.Status != "applied" || transaction.Interpretation.Failure != nil {
				t.Fatal("successful payment was described as failed")
			}
		} else if action.Status != "failed" || transaction.Interpretation.Failure == nil ||
			!strings.Contains(transaction.Interpretation.Failure.Message, "insufficient available funds") {
			t.Fatal("payment failure was not explained")
		}
	}
}

func TestFailedMultiOperationTransactionNeverClaimsCompletedPayment(t *testing.T) {
	envelope := testutil.PaymentEnvelope()
	envelope.V1.Tx.Operations = append(envelope.V1.Tx.Operations, envelope.V1.Tx.Operations[0])
	result := testutil.Result(envelope, false)
	results := []xdr.OperationResult{
		testutil.PaymentResult(xdr.PaymentResultCodePaymentSuccess),
		testutil.PaymentResult(xdr.PaymentResultCodePaymentNoDestination),
	}
	result.Result.Result.Results = &results
	transaction, err := decoder.Transaction(envelope, result,
		xdr.TransactionMeta{V: 2, V2: &xdr.TransactionMetaV2{}}, testutil.Ledger(10), 1)
	if err != nil {
		t.Fatal(err)
	}

	actions := transaction.Interpretation.Actions
	if actions[0].Status != "not_applied" || actions[1].Status != "failed" ||
		transaction.Interpretation.Failure.Operations[0].OpIndex != 1 ||
		!strings.Contains(
			transaction.Interpretation.Failure.Message,
			"destination account does not exist",
		) {
		t.Fatal("atomic rollback or failing operation was misrepresented")
	}
}

func TestContractActionsKeepHandledDiagnosticsSeparateFromOutcome(t *testing.T) {
	for _, successful := range []bool{true, false} {
		transaction := testutil.InterpretationBundle(t, 10, successful).Transactions[0]
		interpretation := transaction.Interpretation
		if len(interpretation.DiagnosticErrors) != 1 ||
			interpretation.DiagnosticErrors[0].Code != "7" ||
			!strings.Contains(
				interpretation.DiagnosticErrors[0].Message,
				"depends on the contract",
			) {
			t.Fatal("contract-defined error was omitted or assigned a guessed meaning")
		}

		if successful {
			if interpretation.Failure != nil || len(interpretation.Actions) != 2 ||
				interpretation.Actions[0].Kind != "contract_call" ||
				interpretation.Actions[1].Kind != "transfer" ||
				interpretation.Actions[1].Source != "token_event" {
				t.Fatal("handled diagnostic changed success or token effect was lost")
			}
		} else if len(interpretation.Actions) != 1 || interpretation.Actions[0].Status != "failed" ||
			!strings.Contains(interpretation.Failure.Message, "trap") || transaction.Fees.Status != "complete" {
			t.Fatal("failed contract gained a token action or lost its committed fee")
		}
	}
}

func TestInterpretationRebuildUsesSourceInsteadOfDerivedPayload(t *testing.T) {
	transaction := testutil.InterpretationBundle(t, 10, true).Transactions[0]
	expected := transaction.Interpretation
	transaction.TokenTransfers = nil
	transaction.Events = nil
	if err := decoder.RebuildInterpretation(&transaction); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(expected, transaction.Interpretation) ||
		transaction.TokenTransfers != nil ||
		transaction.Events != nil {
		t.Fatal("interpretation trusted or overwrote unrelated derived payload")
	}

	for _, corrupt := range []func(*domain.Transaction){
		func(transaction *domain.Transaction) {
			transaction.Network = "mainnet"
		},
		func(transaction *domain.Transaction) {
			transaction.EnvelopeXDR = "invalid"
		},
		func(transaction *domain.Transaction) {
			transaction.ResultXDR = "invalid"
		},
		func(transaction *domain.Transaction) {
			transaction.MetaXDR = "invalid"
		},
		func(transaction *domain.Transaction) {
			transaction.Hash = strings.Repeat("0", 64)
		},
		func(transaction *domain.Transaction) {
			transaction.Successful = false
		},
		func(transaction *domain.Transaction) {
			transaction.SourceAccount = testutil.Address(8)
		},
	} {
		broken := transaction
		corrupt(&broken)
		if err := decoder.RebuildInterpretation(&broken); err == nil {
			t.Fatal("corrupt interpretation source was accepted")
		}
	}
}

func TestRealFixtureInterpretationAndDeterministicRebuild(t *testing.T) {
	bundle := testutil.RealBundle(t)
	for _, transaction := range bundle.Transactions {
		if transaction.Interpretation.Version != decoder.InterpretationVersion ||
			len(transaction.Interpretation.Actions) == 0 ||
			(transaction.Interpretation.Failure == nil) != transaction.Successful {
			t.Fatal("real transaction missing interpretation", transaction.Hash)
		}

		before, _ := json.Marshal(transaction)
		if err := decoder.RebuildInterpretation(&transaction); err != nil {
			t.Fatal(err)
		}

		after, _ := json.Marshal(transaction)
		if string(before) != string(after) {
			t.Fatal("source-based rebuild changed the transaction", transaction.Hash)
		}
	}
}

func TestClassicMovementIsNotCountedTwice(t *testing.T) {
	transaction := testutil.FeeBundle(t, 10, true).Transactions[0]
	if len(transaction.Interpretation.Actions) != 1 ||
		transaction.Interpretation.Actions[0].Kind != "payment" ||
		len(transaction.Interpretation.Actions[0].EventIndexes) != 1 {
		t.Fatal("classic payment and its committed movement were counted twice")
	}
}

func TestTokenActionsUseVerifiedAssetAndDoNotInferSwaps(t *testing.T) {
	assets, err := xdr.BuildAssets("USD:" + testutil.Address(3))
	if err != nil {
		t.Fatal(err)
	}

	for _, kind := range []string{"transfer", "mint", "burn", "clawback"} {
		t.Run(kind, func(t *testing.T) {
			addresses := []string{testutil.Address(1)}
			if kind == "transfer" {
				addresses = append(addresses, testutil.Address(2))
			}

			event := testutil.TokenEvent(testutil.SACContract(assets[0]), kind, addresses,
				testutil.I128(9223372036854775807, 18446744073709551615),
				testutil.String(assets[0].StringCanonical()))
			envelope := testutil.InvocationEnvelope("swap")
			meta := xdr.TransactionMeta{
				V: 4,
				V4: &xdr.TransactionMetaV4{
					Operations: []xdr.OperationMetaV2{{Events: []xdr.ContractEvent{event}}},
				},
			}
			transaction, err := decoder.Transaction(envelope, testutil.Result(envelope, true),
				meta, testutil.Ledger(10), 1)
			if err != nil {
				t.Fatal(err)
			}

			actions := transaction.Interpretation.Actions
			if len(actions) != 2 || actions[0].Kind != "contract_call" || actions[1].Kind != kind ||
				*actions[1].Amount != "170141183460469231731687303715884105727" ||
				*actions[1].Asset.Code != "USD" || actions[1].EventIndexes[0] != 0 {
				t.Fatal("token action lost amount/provenance or inferred a swap")
			}
		})
	}
}

func TestPreconditionFailureDoesNotInventFailedOperation(t *testing.T) {
	for _, scenario := range []struct {
		code xdr.TransactionResultCode
		text string
	}{
		{xdr.TransactionResultCodeTxBadSeq, "sequence number"},
		{xdr.TransactionResultCodeTxTooLate, "expired"},
		{xdr.TransactionResultCodeTxInsufficientFee, "fee is insufficient"},
	} {
		envelope := testutil.PaymentEnvelope()
		result := testutil.Result(envelope, false)
		result.Result.Result = xdr.TransactionResultResult{Code: scenario.code}
		transaction, err := decoder.Transaction(envelope, result,
			xdr.TransactionMeta{V: 2, V2: &xdr.TransactionMetaV2{}}, testutil.Ledger(10), 1)
		if err != nil ||
			!strings.Contains(transaction.Interpretation.Failure.Message, scenario.text) ||
			transaction.Interpretation.Actions[0].Status != "not_applied" ||
			len(transaction.Interpretation.Failure.Operations) != 0 {
			t.Fatal("precondition failure attributed to an operation", scenario.code, err)
		}
	}
}

func TestTypedErrorsAndDiagnosticLimits(t *testing.T) {
	value := decoder.Value(testutil.ContractError(4294967295)).(map[string]any)
	if value["decode_status"] != "decoded" ||
		value["value"].(map[string]any)["code"] != "4294967295" {
		t.Fatal("contract error lost its exact code")
	}

	bundle := testutil.InterpretationBundle(t, 10, true)
	transaction := bundle.Transactions[0]
	var meta xdr.TransactionMeta
	if err := xdr.SafeUnmarshalBase64(transaction.MetaXDR, &meta); err != nil {
		t.Fatal(err)
	}

	diagnostic := meta.V4.DiagnosticEvents[0]
	for len(meta.V4.DiagnosticEvents) < 70 {
		meta.V4.DiagnosticEvents = append(meta.V4.DiagnosticEvents, diagnostic)
	}

	transaction.MetaXDR, _ = xdr.MarshalBase64(meta)
	if err := decoder.RebuildInterpretation(&transaction); err != nil ||
		len(transaction.Interpretation.DiagnosticErrors) != 64 ||
		!transaction.Interpretation.DiagnosticsTruncated || transaction.Interpretation.Failure != nil {
		t.Fatal("diagnostic output was unbounded or changed success", err)
	}
}

func TestFeeBumpFailureExplainsInnerPrecondition(t *testing.T) {
	inner := testutil.PaymentEnvelope()
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
	result := testutil.Result(envelope, false)
	result.Result.Result = xdr.TransactionResultResult{
		Code: xdr.TransactionResultCodeTxFeeBumpInnerFailed,
		InnerResultPair: &xdr.InnerTransactionResultPair{
			Result: xdr.InnerTransactionResult{
				Result: xdr.InnerTransactionResultResult{
					Code: xdr.TransactionResultCodeTxBadSeq,
				},
			},
		},
	}
	transaction, err := decoder.Transaction(envelope, result,
		xdr.TransactionMeta{V: 2, V2: &xdr.TransactionMetaV2{}}, testutil.Ledger(10), 1)
	if err != nil ||
		!strings.Contains(transaction.Interpretation.Failure.Message, "sequence number") ||
		*transaction.Interpretation.Failure.InnerCode != xdr.TransactionResultCodeTxBadSeq.String() {
		t.Fatal("fee bump inner failure was not explained", err)
	}
}
