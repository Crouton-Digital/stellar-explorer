package decoder

import (
	"strings"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func transactionFailure(result xdr.TransactionResult) *domain.Failure {
	code := result.Result.Code
	failure := &domain.Failure{
		Code:       code.String(),
		Message:    transactionFailureMessage(code),
		Operations: []domain.OperationFailure{},
	}
	if inner, ok := result.Result.GetInnerResultPair(); ok {
		innerCode := inner.Result.Result.Code
		failure.InnerCode = pointer(innerCode.String())
		failure.Message = transactionFailureMessage(innerCode)
	}

	return failure
}

func transactionFailureMessage(code xdr.TransactionResultCode) string {
	switch code {
	case xdr.TransactionResultCodeTxFailed:
		return "One or more operations failed. Operation changes were not applied."
	case xdr.TransactionResultCodeTxTooEarly:
		return "The transaction was submitted before its allowed time or ledger range."
	case xdr.TransactionResultCodeTxTooLate:
		return "The transaction expired before it could be applied."
	case xdr.TransactionResultCodeTxMissingOperation:
		return "The transaction contains no operations."
	case xdr.TransactionResultCodeTxBadSeq:
		return "The source account sequence number does not satisfy the transaction requirements."
	case xdr.TransactionResultCodeTxBadAuth:
		return "The transaction does not have enough valid signatures for the source account."
	case xdr.TransactionResultCodeTxInsufficientBalance:
		return "The fee-paying account has insufficient available XLM for the transaction fee."
	case xdr.TransactionResultCodeTxNoAccount:
		return "The transaction source account does not exist."
	case xdr.TransactionResultCodeTxInsufficientFee:
		return "The offered transaction fee is insufficient."
	case xdr.TransactionResultCodeTxBadAuthExtra:
		return "The transaction includes signatures that were not required."
	case xdr.TransactionResultCodeTxInternalError:
		return "The network encountered an internal error while processing this transaction."
	case xdr.TransactionResultCodeTxNotSupported:
		return "This transaction format is not supported by the ledger protocol."
	case xdr.TransactionResultCodeTxFeeBumpInnerFailed:
		return "The inner transaction of the fee bump failed."
	case xdr.TransactionResultCodeTxBadSponsorship:
		return "The transaction left an invalid sponsorship state."
	case xdr.TransactionResultCodeTxBadMinSeqAgeOrGap:
		return "The transaction does not satisfy its minimum sequence age or ledger gap."
	case xdr.TransactionResultCodeTxMalformed:
		return "The transaction is malformed."
	case xdr.TransactionResultCodeTxSorobanInvalid:
		return "The transaction has invalid Soroban resource or footprint data."
	case xdr.TransactionResultCodeTxFrozenKeyAccessed:
		return "The transaction accessed a frozen ledger key."
	default:
		return "The transaction was not applied. Inspect the result code for details."
	}
}

func operationResultCode(result xdr.OperationResult, expected xdr.OperationType) (*string, bool) {
	if result.Code != xdr.OperationResultCodeOpInner {
		return pointer(result.Code.String()), true
	}

	inner, ok := result.GetTr()
	if !ok || inner.Type != expected {
		return nil, false
	}

	code, err := inner.MapOperationResultTr()
	if err != nil || code == "" {
		return nil, false
	}

	return &code, !strings.HasSuffix(code, "Success")
}

func operationFailureMessage(code string) string {
	_, name, _ := strings.Cut(code, "ResultCode")
	messages := map[string]string{
		"OpBadAuth":                                   "The operation lacks enough valid signatures.",
		"OpNoAccount":                                 "The operation source account does not exist.",
		"OpNotSupported":                              "The operation is not supported by this protocol.",
		"OpTooManySubentries":                         "The source account has reached its subentry limit.",
		"OpExceededWorkLimit":                         "The operation exceeded the permitted work limit.",
		"OpTooManySponsoring":                         "The account is sponsoring too many ledger entries.",
		"PaymentMalformed":                            "The payment has invalid parameters.",
		"PaymentUnderfunded":                          "The sender has insufficient available funds for this payment.",
		"PaymentSrcNoTrust":                           "The sender has no trustline for this asset.",
		"PaymentSrcNotAuthorized":                     "The sender is not authorized to transfer this asset.",
		"PaymentNoDestination":                        "The destination account does not exist.",
		"PaymentNoTrust":                              "The destination has no trustline for this asset.",
		"PaymentNotAuthorized":                        "The destination is not authorized to hold this asset.",
		"PaymentLineFull":                             "The payment would exceed the destination trustline limit.",
		"PaymentNoIssuer":                             "The asset issuer does not exist.",
		"CreateAccountMalformed":                      "The account creation has invalid parameters.",
		"CreateAccountUnderfunded":                    "The source has insufficient funds for the starting balance.",
		"CreateAccountLowReserve":                     "The starting balance is below the required minimum reserve.",
		"CreateAccountAlreadyExist":                   "The destination account already exists.",
		"AccountMergeMalformed":                       "The account merge has invalid parameters.",
		"AccountMergeNoAccount":                       "The merge destination account does not exist.",
		"AccountMergeImmutableSet":                    "The source account has the immutable authorization flag set.",
		"AccountMergeHasSubEntries":                   "The source account still has subentries that prevent merging.",
		"AccountMergeSeqnumTooFar":                    "The source account sequence number prevents merging.",
		"AccountMergeDestFull":                        "The destination cannot accept the remaining XLM balance.",
		"AccountMergeIsSponsor":                       "The source account is still sponsoring ledger entries.",
		"InvokeHostFunctionMalformed":                 "The contract invocation has invalid parameters.",
		"InvokeHostFunctionTrapped":                   "Contract execution stopped with a trap. Inspect diagnostics for additional evidence.",
		"InvokeHostFunctionResourceLimitExceeded":     "Contract execution exceeded its declared resource limits.",
		"InvokeHostFunctionEntryArchived":             "A required contract ledger entry is archived.",
		"InvokeHostFunctionInsufficientRefundableFee": "The refundable resource fee is insufficient for contract execution.",
		"ExtendFootprintTtlMalformed":                 "The lifetime extension has invalid parameters.",
		"ExtendFootprintTtlResourceLimitExceeded":     "The lifetime extension exceeded its declared resource limits.",
		"ExtendFootprintTtlInsufficientRefundableFee": "The refundable resource fee is insufficient for the lifetime extension.",
		"RestoreFootprintMalformed":                   "The restoration has invalid parameters.",
		"RestoreFootprintResourceLimitExceeded":       "The restoration exceeded its declared resource limits.",
		"RestoreFootprintInsufficientRefundableFee":   "The refundable resource fee is insufficient for restoration.",
	}
	if message := messages[name]; message != "" {
		return message
	}

	return "The network rejected this operation. Inspect the result code for details."
}
