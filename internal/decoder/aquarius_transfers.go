package decoder

import (
	"math/big"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func aquariusTransfers(
	swap *domain.AquariusSwap,
	requests []aquariusHopRequest,
	routers []aquariusRouterEvent,
	movements []domain.TokenTransfer,
	events []xdr.ContractEvent,
) bool {
	viaRouter := swap.Routing != "direct"
	allMovements := movements
	if !aquariusTokenEvidence(movements, events) {
		return false
	}

	var refundIndex *int
	refunded := "0"
	if swap.Mode == "exact_output" {
		var valid bool
		movements, refunded, refundIndex, valid = aquariusRefund(swap, routers, movements)
		if !valid {
			return false
		}
	}

	expectedCount := 2 * len(requests)
	if viaRouter {
		expectedCount += 2
	}

	if len(movements) != expectedCount {
		return false
	}

	settlement := swap.Trader
	offset := 0
	if viaRouter {
		settlement = aquariusRouter
		offset = 1
		funding := movements[0]
		if !aquariusMovement(
			funding,
			swap.Trader,
			settlement,
			requests[0].inputToken,
			swap.InputLimit,
		) {
			return false
		}
	}

	trades := []int{}
	for index, event := range events {
		if event.Type != xdr.ContractEventTypeContract || event.Body.V != 0 ||
			event.Body.V0 == nil || len(event.Body.V0.Topics) == 0 {
			continue
		}

		name, valid := event.Body.V0.Topics[0].GetSym()
		if valid && name == "trade" {
			trades = append(trades, index)
		}
	}

	if len(trades) != len(requests) {
		return false
	}

	previous := -1
	for index, request := range requests {
		router := routers[index]
		input, output := movements[offset+2*index], movements[offset+2*index+1]
		if !aquariusMovement(
			input,
			settlement,
			router.pool,
			request.inputToken,
			router.inputAmount,
		) ||
			!aquariusMovement(
				output,
				router.pool,
				settlement,
				request.outputToken,
				router.outputAmount,
			) ||
			input.EventIndex <= previous ||
			output.EventIndex <= input.EventIndex ||
			trades[index] <= output.EventIndex ||
			router.index <= trades[index] ||
			!aquariusPoolTrade(events[trades[index]], request, router, settlement) {
			return false
		}

		if index > 0 && request.routeIndex == requests[index-1].routeIndex &&
			router.inputAmount != routers[index-1].outputAmount {
			return false
		}

		previous = router.index
		swap.Hops = append(swap.Hops, domain.AquariusSwapHop{
			Index:            index,
			PoolTokens:       request.tokens,
			PoolIndex:        request.poolIndex,
			PoolContract:     router.pool,
			Input:            domain.ExchangeLeg{Asset: input.Asset, Amount: input.Amount},
			Output:           domain.ExchangeLeg{Asset: output.Asset, Amount: output.Amount},
			InputEventIndex:  input.EventIndex,
			OutputEventIndex: output.EventIndex,
			TradeEventIndex:  trades[index],
			RouterEventIndex: router.index,
		})
	}

	swap.Input = swap.Hops[0].Input
	swap.Output = swap.Hops[len(swap.Hops)-1].Output
	if viaRouter {
		funding, received := movements[0], movements[len(movements)-1]
		if swap.Routing == "split" && !aquariusSplitTotals(swap, funding, received) {
			return false
		}

		if funding.EventIndex >= swap.Hops[0].InputEventIndex ||
			received.EventIndex <= previous ||
			!aquariusMovement(received, settlement, swap.Trader,
				requests[len(requests)-1].outputToken, swap.Output.Amount) {
			return false
		}

		swap.Settlement = &domain.AquariusSettlement{
			FundedAmount:      funding.Amount,
			RefundedAmount:    refunded,
			FundingEventIndex: funding.EventIndex,
			RefundEventIndex:  refundIndex,
			PayoutEventIndex:  received.EventIndex,
		}
	}

	for _, movement := range allMovements {
		swap.TransferEventIndexes = append(swap.TransferEventIndexes, movement.EventIndex)
	}

	return true
}

func aquariusRefund(
	swap *domain.AquariusSwap,
	routers []aquariusRouterEvent,
	movements []domain.TokenTransfer,
) ([]domain.TokenTransfer, string, *int, bool) {
	maximum, _ := new(big.Int).SetString(swap.InputLimit, 10)
	spent, _ := new(big.Int).SetString(routers[0].inputAmount, 10)
	refund := new(big.Int).Sub(maximum, spent)
	if swap.Routing != "chained" || refund.Sign() < 0 {
		return nil, "", nil, false
	}

	if refund.Sign() == 0 {
		return movements, "0", nil, true
	}

	// Retained current executions return unused input after the final payout.
	// Earlier or intermediate refunds require separate evidence rules.
	if len(movements) != 2*len(routers)+3 {
		return nil, "", nil, false
	}

	position := len(movements) - 1
	movement := movements[position]
	if !aquariusMovement(movement, aquariusRouter, swap.Trader, swap.Path[0], refund.String()) {
		return nil, "", nil, false
	}

	filtered := append([]domain.TokenTransfer{}, movements[:position]...)
	index := movement.EventIndex

	return filtered, refund.String(), &index, true
}

func aquariusMovement(
	movement domain.TokenTransfer,
	from, to, token, amount string,
) bool {
	return movement.From == from && movement.To == to &&
		movement.Asset.ContractID == token && movement.Amount == amount
}

func aquariusTokenEvidence(movements []domain.TokenTransfer, events []xdr.ContractEvent) bool {
	byIndex := map[int]bool{}
	previous := -1
	for _, movement := range movements {
		if movement.OpIndex != 0 || movement.Kind != "transfer" || movement.ToMuxed != nil ||
			movement.EventIndex <= previous {
			return false
		}

		previous = movement.EventIndex
		byIndex[movement.EventIndex] = true
	}

	for index, event := range events {
		if event.Type != xdr.ContractEventTypeContract || event.Body.V != 0 ||
			event.Body.V0 == nil || len(event.Body.V0.Topics) == 0 {
			continue
		}

		name, valid := event.Body.V0.Topics[0].GetSym()
		if valid &&
			(name == "transfer" || name == "mint" || name == "burn" || name == "clawback") &&
			!byIndex[index] {
			return false
		}
	}

	return true
}
