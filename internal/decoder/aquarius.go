package decoder

import (
	"math/big"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// The Testnet router is published in Aquarius's network prerequisites.
// The registry address does not establish a historical executable or pool registry.
const (
	aquariusRouter        = "CBCFTQSPDBAIZ6R6PJQKSQWKNKWH2QIV3I4J72SHWBIK3ADRRAM5A6GD"
	aquariusMaximumHops   = 16
	aquariusMaximumTokens = 8
	aquariusMaximumRoutes = 8
)

type aquariusHopRequest struct {
	tokens      []string
	poolIndex   string
	inputToken  string
	outputToken string
	routeIndex  int
}

func interpretAquariusSwap(
	t *domain.Transaction,
	envelope xdr.TransactionEnvelope,
	result xdr.TransactionResult,
	meta xdr.TransactionMeta,
) *domain.AquariusSwap {
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
	if err != nil || address != aquariusRouter {
		return nil
	}

	swap, requests := aquariusRequest(call)
	if swap == nil {
		return nil
	}

	operations := operationEvents(meta)
	results, hasResults := result.OperationResults()
	if len(operations) != 1 || !hasResults || len(results) != 1 ||
		results[0].Code != xdr.OperationResultCodeOpInner || results[0].Tr == nil ||
		results[0].Tr.Type != xdr.OperationTypeInvokeHostFunction ||
		results[0].Tr.MustInvokeHostFunctionResult().Code !=
			xdr.InvokeHostFunctionResultCodeInvokeHostFunctionSuccess {
		return nil
	}

	if !contractInvocationResultMatches(meta, results[0], operations[0]) {
		return nil
	}

	observed, valid := aquariusCodeEvidence(meta)
	if !valid {
		return nil
	}

	swap.RouterWasmHash = observed
	events, valid := aquariusRouterEvents(operations[0], swap.Trader, requests)
	if !valid || !aquariusTransfers(swap, requests, events, t.TokenTransfers, operations[0]) {
		return nil
	}

	returned := contractInvocationReturn(meta)
	if returned == nil {
		return nil
	}

	amount, valid := aquariusQuantity(*returned, true)
	if !valid || !aquariusExecutionLimits(swap, amount) {
		return nil
	}

	return swap
}

func aquariusExecutionLimits(swap *domain.AquariusSwap, returned string) bool {
	input, _ := new(big.Int).SetString(swap.Input.Amount, 10)
	output, _ := new(big.Int).SetString(swap.Output.Amount, 10)
	inputLimit, _ := new(big.Int).SetString(swap.InputLimit, 10)
	outputLimit, _ := new(big.Int).SetString(swap.OutputLimit, 10)
	if swap.Mode == "exact_output" {
		return returned == swap.Input.Amount && output.Cmp(outputLimit) == 0 &&
			input.Cmp(inputLimit) <= 0
	}

	return returned == swap.Output.Amount && input.Cmp(inputLimit) == 0 &&
		output.Cmp(outputLimit) >= 0
}

func aquariusRequest(call xdr.InvokeContractArgs) (*domain.AquariusSwap, []aquariusHopRequest) {
	if call.FunctionName == "swap_split" {
		return aquariusSplitRequest(call)
	}

	var requests []aquariusHopRequest
	var input, output xdr.ScVal
	routing := "direct"
	mode := "exact_input"
	switch call.FunctionName {
	case "swap":
		if len(call.Args) != 7 {
			return nil, nil
		}

		request, valid := aquariusPoolRequest(
			call.Args[1],
			call.Args[4],
			call.Args[2],
			call.Args[3],
		)
		if !valid {
			return nil, nil
		}

		requests = []aquariusHopRequest{request}
		input, output = call.Args[5], call.Args[6]
	case "swap_chained", "swap_chained_strict_receive":
		if len(call.Args) != 5 {
			return nil, nil
		}

		var valid bool
		requests, valid = aquariusChainRequest(call.Args[1], call.Args[2])
		if !valid {
			return nil, nil
		}

		routing = "chained"
		input, output = call.Args[3], call.Args[4]
		if call.FunctionName == "swap_chained_strict_receive" {
			mode = "exact_output"
			input, output = call.Args[4], call.Args[3]
		}
	default:
		return nil, nil
	}

	trader, valid := contractAddressValue(call.Args[0])
	if !valid || trader == aquariusRouter {
		return nil, nil
	}

	inputLimit, valid := aquariusQuantity(input, true)
	if !valid {
		return nil, nil
	}

	outputLimit, valid := aquariusQuantity(output, mode == "exact_output")
	if !valid {
		return nil, nil
	}

	path, valid := aquariusPath(requests)
	if !valid {
		return nil, nil
	}

	swap := newAquariusSwap(trader, mode, routing, inputLimit, outputLimit)
	swap.Path = path

	return swap, requests
}

func aquariusPath(requests []aquariusHopRequest) ([]string, bool) {
	path := []string{requests[0].inputToken}
	seen := map[string]bool{path[0]: true}
	for _, request := range requests {
		if seen[request.outputToken] {
			return nil, false
		}

		seen[request.outputToken] = true
		path = append(path, request.outputToken)
	}

	return path, true
}

func newAquariusSwap(trader, mode, routing, inputLimit, outputLimit string) *domain.AquariusSwap {
	return &domain.AquariusSwap{
		Protocol:             "aquarius",
		AdapterVersion:       2,
		Verification:         "registered_router_events_and_transfers",
		RouterContract:       aquariusRouter,
		Mode:                 mode,
		Routing:              routing,
		Trader:               trader,
		InputLimit:           inputLimit,
		OutputLimit:          outputLimit,
		Path:                 []string{},
		Hops:                 []domain.AquariusSwapHop{},
		TransferEventIndexes: []int{},
	}
}
