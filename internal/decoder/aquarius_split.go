package decoder

import (
	"math/big"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func aquariusSplitRequest(
	call xdr.InvokeContractArgs,
) (*domain.AquariusSwap, []aquariusHopRequest) {
	if len(call.Args) != 6 {
		return nil, nil
	}

	trader, valid := contractAddressValue(call.Args[0])
	if !valid || trader == aquariusRouter {
		return nil, nil
	}

	inputToken, valid := aquariusContract(call.Args[2])
	if !valid {
		return nil, nil
	}

	outputToken, valid := aquariusContract(call.Args[3])
	if !valid || inputToken == outputToken {
		return nil, nil
	}

	inputLimit, valid := aquariusQuantity(call.Args[4], true)
	if !valid {
		return nil, nil
	}

	outputLimit, valid := aquariusQuantity(call.Args[5], false)
	if !valid {
		return nil, nil
	}

	groups, valid := call.Args[1].GetVec()
	if !valid || groups == nil || len(*groups) != 1 {
		return nil, nil
	}

	branches, valid := (*groups)[0].GetVec()
	if !valid || branches == nil || len(*branches) < 2 || len(*branches) > aquariusMaximumRoutes {
		return nil, nil
	}

	swap := newAquariusSwap(trader, "exact_input", "split", inputLimit, outputLimit)
	requests := []aquariusHopRequest{}
	var totalWeight uint32
	for index, branch := range *branches {
		fields, valid := branch.GetVec()
		if !valid || fields == nil || len(*fields) != 2 {
			return nil, nil
		}

		weight, valid := (*fields)[1].GetU32()
		if !valid || weight == 0 || weight > 10000 {
			return nil, nil
		}

		totalWeight += uint32(weight)
		chain, valid := aquariusChainRequest((*fields)[0], call.Args[2])
		if !valid || len(requests)+len(chain) > aquariusMaximumHops ||
			chain[len(chain)-1].outputToken != outputToken {
			return nil, nil
		}

		path, valid := aquariusPath(chain)
		if !valid {
			return nil, nil
		}

		allocation, valid := aquariusSplitAllocation(inputLimit, uint32(weight))
		if !valid {
			return nil, nil
		}

		hopIndexes := []int{}
		for hop := range chain {
			chain[hop].routeIndex = index
			hopIndexes = append(hopIndexes, len(requests)+hop)
		}

		requests = append(requests, chain...)
		swap.Routes = append(swap.Routes, domain.AquariusSwapRoute{
			Index:      index,
			WeightBps:  uint32(weight),
			Path:       path,
			Input:      domain.ExchangeLeg{Amount: allocation},
			HopIndexes: hopIndexes,
		})
	}

	if totalWeight != 10000 {
		return nil, nil
	}

	return swap, requests
}

func aquariusSplitAllocation(input string, weight uint32) (string, bool) {
	amount, _ := new(big.Int).SetString(input, 10)
	weighted := new(big.Int).Mul(amount, new(big.Int).SetUint64(uint64(weight)))
	allocated, remainder := new(big.Int), new(big.Int)
	allocated.QuoRem(weighted, big.NewInt(10000), remainder)
	if remainder.Sign() != 0 || allocated.Sign() <= 0 {
		return "", false
	}

	return allocated.String(), true
}

func aquariusSplitTotals(
	swap *domain.AquariusSwap,
	funding, payout domain.TokenTransfer,
) bool {
	inputTotal, outputTotal := new(big.Int), new(big.Int)
	for index := range swap.Routes {
		route := &swap.Routes[index]
		first := swap.Hops[route.HopIndexes[0]]
		last := swap.Hops[route.HopIndexes[len(route.HopIndexes)-1]]
		if first.Input.Amount != route.Input.Amount {
			return false
		}

		route.Input = first.Input
		route.Output = last.Output
		input, _ := new(big.Int).SetString(route.Input.Amount, 10)
		output, _ := new(big.Int).SetString(route.Output.Amount, 10)
		inputTotal.Add(inputTotal, input)
		outputTotal.Add(outputTotal, output)
	}

	if funding.Amount != inputTotal.String() || payout.Amount != outputTotal.String() {
		return false
	}

	swap.Input = domain.ExchangeLeg{Asset: funding.Asset, Amount: funding.Amount}
	swap.Output = domain.ExchangeLeg{Asset: payout.Asset, Amount: payout.Amount}

	return true
}
