package decoder

import (
	"encoding/hex"
	"math/big"
	"slices"
	"strconv"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Pins are from soroswap/core revision 6eade008ee6794075ad941609e0fef0d1124d3eb.
// The published deployment does not establish an execution-time WASM hash.
const (
	soroswapRouter      = "CCJUD55AG6W5HAI5LRVNKAE5WDP5XGZBUDS5WNTIVDU7O264UZZE7BRD"
	soroswapRouterHash  = "4b95bbf9caec2c6e00c786f53c5f392c2fcdb8435ac0a862ab5e0645eb65824c"
	soroswapFactory     = "CDP3HMUH6SMS3S7NPGNDJLULCOXXEPSHY4JKUKMBNQMATHDHWXRRJTBY"
	soroswapMaximumPath = 17
)

func interpretContractSwap(
	t *domain.Transaction,
	envelope xdr.TransactionEnvelope,
	result xdr.TransactionResult,
	meta xdr.TransactionMeta,
) *domain.ContractSwap {
	if t.Network != domain.Network || !t.Successful || !result.Successful() ||
		len(envelope.Operations()) != 1 {
		return nil
	}

	operation := envelope.Operations()[0]
	if operation.Body.Type != xdr.OperationTypeInvokeHostFunction {
		return nil
	}

	function := operation.Body.MustInvokeHostFunctionOp().HostFunction
	if function.Type != xdr.HostFunctionTypeHostFunctionTypeInvokeContract {
		return nil
	}

	call := function.MustInvokeContract()
	address, err := call.ContractAddress.String()
	if err != nil || address != soroswapRouter {
		return nil
	}

	swap := soroswapRequest(t, call)
	if swap == nil {
		return nil
	}

	observedHash, valid := soroswapCodeEvidence(meta)
	if !valid {
		return nil
	}

	swap.RouterWasmHash = observedHash
	operations := operationEvents(meta)
	if len(operations) != 1 {
		return nil
	}

	event, eventIndex, valid := soroswapEvent(operations[0])
	if !valid {
		return nil
	}

	path, valid := soroswapPath(event["path"])
	if !valid || !slices.Equal(path, swap.Path) {
		return nil
	}

	trader, valid := contractAddressValue(event["to"])
	if !valid || trader != swap.Trader {
		return nil
	}

	amounts, valid := soroswapAmounts(event["amounts"])
	if !valid || len(amounts) != len(path) || !soroswapLimits(swap, amounts) {
		return nil
	}

	returned, valid := soroswapReturn(meta)
	if !valid || !slices.Equal(returned, amounts) {
		return nil
	}

	results, hasResults := result.OperationResults()
	if !hasResults || len(results) != 1 || results[0].Code != xdr.OperationResultCodeOpInner ||
		results[0].Tr == nil || results[0].Tr.Type != xdr.OperationTypeInvokeHostFunction {
		return nil
	}

	if results[0].Tr.MustInvokeHostFunctionResult().Code !=
		xdr.InvokeHostFunctionResultCodeInvokeHostFunctionSuccess {
		return nil
	}

	if !contractInvocationResultMatches(meta, results[0], operations[0]) {
		return nil
	}

	swap.Amounts = amounts
	swap.RouterEventIndex = eventIndex
	if !soroswapTransfers(swap, t.TokenTransfers, operations[0]) {
		return nil
	}

	return swap
}

func soroswapRequest(t *domain.Transaction, call xdr.InvokeContractArgs) *domain.ContractSwap {
	mode := "exact_input"
	switch call.FunctionName {
	case "swap_exact_tokens_for_tokens":
	case "swap_tokens_for_exact_tokens":
		mode = "exact_output"
	default:
		return nil
	}

	if len(call.Args) != 5 {
		return nil
	}

	first, valid := contractI128Quantity(call.Args[0], false)
	if !valid {
		return nil
	}

	second, valid := contractI128Quantity(call.Args[1], false)
	if !valid {
		return nil
	}

	path, valid := soroswapPath(call.Args[2])
	if !valid {
		return nil
	}

	trader, valid := contractAddressValue(call.Args[3])
	if !valid {
		return nil
	}

	deadline, valid := call.Args[4].GetU64()
	if !valid || t.CreatedAt.Unix() < 0 || uint64(deadline) <= uint64(t.CreatedAt.Unix()) {
		return nil
	}

	inputLimit, outputLimit := first, second
	if mode == "exact_output" {
		inputLimit, outputLimit = second, first
	}

	return &domain.ContractSwap{
		Protocol:             "soroswap",
		AdapterVersion:       1,
		Verification:         "registered_router_and_transfers",
		RouterContract:       soroswapRouter,
		Mode:                 mode,
		Trader:               trader,
		InputLimit:           inputLimit,
		OutputLimit:          outputLimit,
		Deadline:             strconv.FormatUint(uint64(deadline), 10),
		Path:                 path,
		Hops:                 []domain.ContractSwapHop{},
		TransferEventIndexes: []int{},
	}
}

func soroswapLimits(swap *domain.ContractSwap, amounts []string) bool {
	input, _ := new(big.Int).SetString(amounts[0], 10)
	output, _ := new(big.Int).SetString(amounts[len(amounts)-1], 10)
	inputLimit, _ := new(big.Int).SetString(swap.InputLimit, 10)
	outputLimit, _ := new(big.Int).SetString(swap.OutputLimit, 10)
	if swap.Mode == "exact_input" {
		return input.Cmp(inputLimit) == 0 && output.Cmp(outputLimit) >= 0
	}

	return input.Cmp(inputLimit) <= 0 && output.Cmp(outputLimit) == 0
}

func soroswapCodeEvidence(meta xdr.TransactionMeta) (*string, bool) {
	before, operations, after, err := contractChanges(meta)
	if err != nil {
		return nil, false
	}

	groups := append([]xdr.LedgerEntryChanges{before}, operations...)
	groups = append(groups, after)
	var observed *string
	for _, group := range groups {
		for _, change := range group {
			var entry *xdr.LedgerEntry
			switch change.Type {
			case xdr.LedgerEntryChangeTypeLedgerEntryState:
				entry = change.State
			case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
				entry = change.Created
			case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
				entry = change.Updated
			case xdr.LedgerEntryChangeTypeLedgerEntryRestored:
				entry = change.Restored
			}

			if entry == nil || entry.Data.Type != xdr.LedgerEntryTypeContractData {
				continue
			}

			data := entry.Data.MustContractData()
			address, err := data.Contract.String()
			if err != nil || address != soroswapRouter ||
				data.Key.Type != xdr.ScValTypeScvLedgerKeyContractInstance {
				continue
			}

			if data.Durability != xdr.ContractDataDurabilityPersistent ||
				data.Val.Type != xdr.ScValTypeScvContractInstance {
				return nil, false
			}

			executable := data.Val.MustInstance().Executable
			if executable.Type != xdr.ContractExecutableTypeContractExecutableWasm {
				return nil, false
			}

			hash := executable.MustWasmHash()
			encoded := hex.EncodeToString(hash[:])
			if encoded != soroswapRouterHash {
				return nil, false
			}

			observed = &encoded
		}
	}

	return observed, true
}
