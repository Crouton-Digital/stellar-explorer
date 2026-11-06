package decoder

import (
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const InterpretationVersion = 5

// RebuildInterpretation enriches retained source data without rebuilding other derived fields.
func RebuildInterpretation(t *domain.Transaction) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("invalid interpretation XDR: %v", recovered)
		}
	}()

	if t.Network != domain.Network {
		return fmt.Errorf("interpretation source network is not %s", domain.Network)
	}

	var envelope xdr.TransactionEnvelope
	if err = xdr.SafeUnmarshalBase64(t.EnvelopeXDR, &envelope); err != nil {
		return fmt.Errorf("interpretation envelope: %w", err)
	}

	var result xdr.TransactionResult
	if err = xdr.SafeUnmarshalBase64(t.ResultXDR, &result); err != nil {
		return fmt.Errorf("interpretation result: %w", err)
	}

	var meta xdr.TransactionMeta
	if err = xdr.SafeUnmarshalBase64(t.MetaXDR, &meta); err != nil {
		return fmt.Errorf("interpretation metadata: %w", err)
	}

	hash, err := network.HashTransactionInEnvelope(envelope, domain.Passphrase)
	if err != nil || hex.EncodeToString(hash[:]) != t.Hash || result.Successful() != t.Successful {
		return fmt.Errorf("interpretation source does not match transaction identity or status")
	}

	if envelope.SourceAccount().ToAccountId().Address() != t.SourceAccount ||
		envelope.FeeAccount().ToAccountId().Address() != t.FeeAccount {
		return fmt.Errorf("interpretation source accounts do not match the envelope")
	}

	return interpretTransaction(t, envelope, result, meta)
}

func interpretTransaction(
	t *domain.Transaction,
	envelope xdr.TransactionEnvelope,
	result xdr.TransactionResult,
	meta xdr.TransactionMeta,
) error {
	// Derive movement evidence from source metadata, including during upgrades.
	evidence := *t
	evidence.TokenTransfers = []domain.TokenTransfer{}
	if t.Successful {
		eventIndex := 0
		for index, events := range operationEvents(meta) {
			for _, raw := range events {
				event := domain.Event{
					TransactionHash: t.Hash,
					LedgerSequence:  t.LedgerSequence,
					Index:           eventIndex,
					OpIndex:         pointer(index),
					Scope:           "operation",
				}
				appendTokenActivity(&evidence, event, raw, meta.V, map[string]bool{})
				eventIndex++
			}
		}
	}

	interpretation := domain.Interpretation{
		Version:          InterpretationVersion,
		Summary:          "Transaction succeeded.",
		Actions:          []domain.Action{},
		DiagnosticErrors: []domain.DiagnosticError{},
	}
	results, _ := result.OperationResults()
	executedPaths := map[int]bool{}
	if !result.Successful() {
		interpretation.Summary = "Transaction failed. Operation changes were not applied; fees may still be charged."
		interpretation.Failure = transactionFailure(result)
	}

	for index, operation := range envelope.Operations() {
		action, err := operationAction(&evidence, operation, index)
		if err != nil {
			return err
		}

		if index < len(results) {
			code, failed := operationResultCode(results[index], operation.Body.Type)
			action.ResultCode = code
			if failed && interpretation.Failure != nil {
				action.Status = "failed"
				interpretation.Failure.Operations = append(interpretation.Failure.Operations,
					domain.OperationFailure{
						OpIndex: index,
						Code:    *code,
						Message: operationFailureMessage(*code),
					})
			}
		}

		if operation.Body.Type == xdr.OperationTypePathPaymentStrictReceive ||
			operation.Body.Type == xdr.OperationTypePathPaymentStrictSend {
			var operationResult *xdr.OperationResult
			if index < len(results) {
				operationResult = &results[index]
			}

			if err = pathPaymentAction(&action, operation, operationResult, t.LedgerSequence); err != nil {
				return err
			}

			executedPaths[index] = action.PathPayment.Execution != nil
		}

		if index == 0 {
			if swap := interpretContractSwap(&evidence, envelope, result, meta); swap != nil {
				action.Kind = "swap"
				action.Title = "Soroswap exchange"
				action.Description = "Exchange tokens through the registered Soroswap router. Amounts agree with the committed router event, return value and route transfers."
				action.ContractSwap = swap
				action.SourceAccount = action.From
				action.From = swap.Trader
				action.To = swap.Trader
				action.EventIndexes = append(action.EventIndexes, swap.RouterEventIndex)
			}

			if swap := interpretAquariusSwap(&evidence, envelope, result, meta); swap != nil {
				action.Kind = "swap"
				action.Title = "Aquarius exchange"
				action.Description = "Exchange tokens through the registered Aquarius router. Request limits, return amount, router and pool events agree with the committed token transfers."
				action.AquariusSwap = swap
				action.SourceAccount = action.From
				action.From = swap.Trader
				action.To = swap.Trader
				for _, hop := range swap.Hops {
					action.EventIndexes = append(
						action.EventIndexes,
						hop.TradeEventIndex,
						hop.RouterEventIndex,
					)
				}

				slices.Sort(action.EventIndexes)
			}
		}

		interpretation.Actions = append(interpretation.Actions, action)
	}

	// Classic payments/account operations already describe the value movement once.
	// Contract calls and their committed token effects are distinct actions.
	for _, movement := range evidence.TokenTransfers {
		if !t.Successful {
			continue
		}

		if movement.OpIndex >= 0 && movement.OpIndex < len(envelope.Operations()) {
			kind := envelope.Operations()[movement.OpIndex].Body.Type
			if kind == xdr.OperationTypePayment || kind == xdr.OperationTypeCreateAccount ||
				kind == xdr.OperationTypeAccountMerge ||
				executedPaths[movement.OpIndex] {
				continue
			}
		}

		interpretation.Actions = append(interpretation.Actions, movementAction(movement))
	}

	if interpretation.Failure != nil && len(interpretation.Failure.Operations) == 1 {
		failure := interpretation.Failure.Operations[0]
		interpretation.Failure.Message = fmt.Sprintf("Operation %d failed: %s",
			failure.OpIndex+1, failure.Message)
	}

	if err := interpretDiagnostics(&interpretation, meta); err != nil {
		return err
	}

	var err error
	interpretation.StateChanges, err = interpretState(meta, t.Successful)
	if err != nil {
		return err
	}

	t.Interpretation = interpretation

	return nil
}

func operationAction(
	t *domain.Transaction,
	operation xdr.Operation,
	index int,
) (domain.Action, error) {
	source := t.SourceAccount
	if operation.SourceAccount != nil {
		source = operation.SourceAccount.ToAccountId().Address()
	}

	action := domain.Action{
		ID:           fmt.Sprintf("operation:%d", index),
		Kind:         "unknown_operation",
		Title:        "Operation details unavailable",
		Description:  "This operation has no plain-language interpretation yet. Inspect its decoded details and XDR.",
		Status:       "not_applied",
		Source:       "operation",
		OpIndex:      index,
		EventIndexes: []int{},
		From:         source,
	}
	if t.Successful {
		action.Status = "applied"
	}

	for _, movement := range t.TokenTransfers {
		if t.Successful && movement.OpIndex == index {
			action.EventIndexes = append(action.EventIndexes, movement.EventIndex)
		}
	}

	switch operation.Body.Type {
	case xdr.OperationTypePayment:
		payment := operation.Body.MustPaymentOp()
		asset, err := actionAsset(payment.Asset, t.LedgerSequence)
		if err != nil {
			return action, err
		}

		action.Kind = "payment"
		action.Title = "Payment"
		action.To = payment.Destination.ToAccountId().Address()
		action.Amount = pointer(strconv.FormatInt(int64(payment.Amount), 10))
		action.Asset = &asset
		if payment.Destination.Address() != action.To {
			action.ToMuxed = map[string]any{
				"address": payment.Destination.Address(),
			}
		}
		action.Description = "Send the specified asset from the operation source to the destination."

	case xdr.OperationTypeCreateAccount:
		creation := operation.Body.MustCreateAccountOp()
		asset, err := actionAsset(xdr.MustNewNativeAsset(), t.LedgerSequence)
		if err != nil {
			return action, err
		}

		action.Kind = "create_account"
		action.Title = "Create account"
		action.To = creation.Destination.Address()
		action.Amount = pointer(strconv.FormatInt(int64(creation.StartingBalance), 10))
		action.Asset = &asset
		action.Description = "Create the destination account and fund its starting XLM balance."

	case xdr.OperationTypeAccountMerge:
		action.Kind = "account_merge"
		action.Title = "Merge account"
		action.To = operation.Body.MustDestination().ToAccountId().Address()
		action.Description = "Close the source account and move its remaining XLM to the destination."

	case xdr.OperationTypeInvokeHostFunction:
		function := operation.Body.MustInvokeHostFunctionOp().HostFunction
		if function.Type == xdr.HostFunctionTypeHostFunctionTypeInvokeContract {
			call := function.MustInvokeContract()
			contract, err := call.ContractAddress.String()
			if err != nil {
				return action, err
			}

			action.Kind = "contract_call"
			action.Title = "Call " + string(call.FunctionName)
			action.ContractID = contract
			action.Function = string(call.FunctionName)
			action.Description = "Invoke this contract function. Committed token effects are listed separately."
		}
	}

	return action, nil
}

func actionAsset(asset xdr.Asset, ledger uint32) (domain.Asset, error) {
	id, err := asset.ContractID(domain.Passphrase)
	if err != nil {
		return domain.Asset{}, err
	}

	canonical := asset.StringCanonical()
	code := strings.TrimRight(asset.GetCode(), "\x00")
	if asset.IsNative() {
		code = "XLM"
	}

	metadata := domain.Asset{
		Network:             domain.Network,
		ContractID:          strkey.MustEncode(strkey.VersionByteContract, id[:]),
		Kind:                "sac",
		Verification:        "contract_id",
		ClassicAsset:        &canonical,
		Code:                &code,
		Decimals:            pointer(uint32(7)),
		FirstObservedLedger: ledger,
	}
	if !asset.IsNative() {
		metadata.Issuer = pointer(asset.GetIssuer())
	}

	return metadata, nil
}

func movementAction(movement domain.TokenTransfer) domain.Action {
	title := map[string]string{
		"transfer": "Token transfer",
		"mint":     "Mint tokens",
		"burn":     "Burn tokens",
		"clawback": "Claw back tokens",
	}[movement.Kind]
	description := map[string]string{
		"transfer": "Move tokens from the sender to the recipient.",
		"mint":     "Issue tokens to the recipient.",
		"burn":     "Destroy tokens held by the sender.",
		"clawback": "Remove and burn tokens from the holder through an issuer clawback.",
	}[movement.Kind]

	return domain.Action{
		ID:           fmt.Sprintf("event:%d", movement.EventIndex),
		Kind:         movement.Kind,
		Title:        title,
		Description:  description,
		Status:       "applied",
		Source:       "token_event",
		OpIndex:      movement.OpIndex,
		EventIndexes: []int{movement.EventIndex},
		From:         movement.From,
		To:           movement.To,
		Amount:       &movement.Amount,
		Asset:        &movement.Asset,
		ToMuxed:      movement.ToMuxed,
	}
}
