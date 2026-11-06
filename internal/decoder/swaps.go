package decoder

import (
	"math/big"
	"strconv"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func pathPaymentAction(
	action *domain.Action,
	operation xdr.Operation,
	result *xdr.OperationResult,
	ledger uint32,
) error {
	var sendAsset, destinationAsset xdr.Asset
	var sendLimit, destinationLimit xdr.Int64
	var destination xdr.MuxedAccount
	var path []xdr.Asset
	mode := "strict_receive"
	if operation.Body.Type == xdr.OperationTypePathPaymentStrictReceive {
		payment := operation.Body.MustPathPaymentStrictReceiveOp()
		sendAsset, destinationAsset = payment.SendAsset, payment.DestAsset
		sendLimit, destinationLimit = payment.SendMax, payment.DestAmount
		destination, path = payment.Destination, payment.Path
	} else {
		payment := operation.Body.MustPathPaymentStrictSendOp()
		mode = "strict_send"
		sendAsset, destinationAsset = payment.SendAsset, payment.DestAsset
		sendLimit, destinationLimit = payment.SendAmount, payment.DestMin
		destination, path = payment.Destination, payment.Path
	}

	input, err := actionAsset(sendAsset, ledger)
	if err != nil {
		return err
	}

	output, err := actionAsset(destinationAsset, ledger)
	if err != nil {
		return err
	}

	requested := &domain.PathPayment{
		Mode:             mode,
		SendAsset:        input,
		DestinationAsset: output,
		SendLimit:        strconv.FormatInt(int64(sendLimit), 10),
		DestinationLimit: strconv.FormatInt(int64(destinationLimit), 10),
		Path:             []domain.Asset{},
	}
	for _, hop := range path {
		asset, err := actionAsset(hop, ledger)
		if err != nil {
			return err
		}

		requested.Path = append(requested.Path, asset)
	}

	action.Kind, action.Title = "path_payment", "Path payment"
	action.To = destination.ToAccountId().Address()
	action.Description = "Send assets through the requested path, subject to the specified limits."
	action.PathPayment = requested
	if destination.Address() != action.To {
		action.ToMuxed = map[string]any{"address": destination.Address()}
	}

	if action.Status != "applied" || result == nil || result.Tr == nil ||
		result.Code != xdr.OperationResultCodeOpInner || result.Tr.Type != operation.Body.Type {
		return nil
	}

	var offers []xdr.ClaimAtom
	var last xdr.SimplePaymentResult
	if mode == "strict_receive" {
		executed, ok := result.Tr.MustPathPaymentStrictReceiveResult().GetSuccess()
		if !ok {
			return nil
		}

		offers, last = executed.Offers, executed.Last
	} else {
		executed, ok := result.Tr.MustPathPaymentStrictSendResult().GetSuccess()
		if !ok {
			return nil
		}

		offers, last = executed.Offers, executed.Last
	}

	if last.Destination.Address() != action.To || !last.Asset.Equals(destinationAsset) ||
		last.Amount <= 0 || sendLimit <= 0 || destinationLimit <= 0 ||
		(mode == "strict_receive" && last.Amount != destinationLimit) ||
		(mode == "strict_send" && last.Amount < destinationLimit) {
		return nil
	}

	exchange, err := reconcileExchange(offers, input, output, last.Amount, ledger)
	if err != nil || exchange == nil {
		return err
	}

	actualInput, _ := new(big.Int).SetString(exchange.Input.Amount, 10)
	if (mode == "strict_receive" && actualInput.Cmp(big.NewInt(int64(sendLimit))) > 0) ||
		(mode == "strict_send" && actualInput.Cmp(big.NewInt(int64(sendLimit))) != 0) {
		return nil
	}

	requested.Execution = exchange
	if len(exchange.Trades) > 0 && !sendAsset.Equals(destinationAsset) {
		action.Kind, action.Title = "swap", "Swap assets"
		action.Description = "Exchange assets through the Stellar DEX and deliver the output to the destination. Amounts come from the successful operation result and exclude transaction fees."
	} else {
		action.Description = "Deliver the asset to the destination without a conversion. Amounts exclude transaction fees."
	}

	return nil
}

// Claims are expressed from the maker's perspective: bought is the taker's input.
// Netting every asset avoids counting intermediate hops or repeated assets twice.
func reconcileExchange(
	claims []xdr.ClaimAtom,
	input, output domain.Asset,
	delivered xdr.Int64,
	ledger uint32,
) (*domain.Exchange, error) {
	exchange := &domain.Exchange{
		Protocol: "stellar_dex",
		Input:    domain.ExchangeLeg{Asset: input},
		Output: domain.ExchangeLeg{
			Asset:  output,
			Amount: strconv.FormatInt(int64(delivered), 10),
		},
		Trades: []domain.Trade{},
	}
	if len(claims) == 0 && input.ContractID == output.ContractID {
		exchange.Input.Amount = exchange.Output.Amount

		return exchange, nil
	}

	if input.ContractID == output.ContractID {
		return nil, nil
	}

	flows := map[string]*big.Int{}
	add := func(asset string, amount *big.Int) {
		if flows[asset] == nil {
			flows[asset] = new(big.Int)
		}

		flows[asset].Add(flows[asset], amount)
	}
	for index, claim := range claims {
		if claim.Type != xdr.ClaimAtomTypeClaimAtomTypeV0 &&
			claim.Type != xdr.ClaimAtomTypeClaimAtomTypeOrderBook &&
			claim.Type != xdr.ClaimAtomTypeClaimAtomTypeLiquidityPool {
			return nil, nil
		}

		bought, sold := claim.AmountBought(), claim.AmountSold()
		if bought == 0 && sold == 0 {
			continue
		}

		if bought <= 0 || sold <= 0 {
			return nil, nil
		}

		paidAsset, err := actionAsset(claim.AssetBought(), ledger)
		if err != nil {
			return nil, err
		}

		receivedAsset, err := actionAsset(claim.AssetSold(), ledger)
		if err != nil {
			return nil, err
		}

		add(paidAsset.ContractID, big.NewInt(int64(bought)))
		add(receivedAsset.ContractID, new(big.Int).Neg(big.NewInt(int64(sold))))
		venue := "order_book"
		if claim.Type == xdr.ClaimAtomTypeClaimAtomTypeLiquidityPool {
			venue = "liquidity_pool"
		}

		exchange.Trades = append(exchange.Trades, domain.Trade{
			ClaimIndex: index,
			Venue:      venue,
			Input: domain.ExchangeLeg{
				Asset:  paidAsset,
				Amount: strconv.FormatInt(int64(bought), 10),
			},
			Output: domain.ExchangeLeg{
				Asset:  receivedAsset,
				Amount: strconv.FormatInt(int64(sold), 10),
			},
		})
	}

	paid, received := flows[input.ContractID], flows[output.ContractID]
	if paid == nil || received == nil || paid.Sign() <= 0 || !paid.IsInt64() ||
		received.Cmp(new(big.Int).Neg(big.NewInt(int64(delivered)))) != 0 {
		return nil, nil
	}

	for asset, flow := range flows {
		if asset != input.ContractID && asset != output.ContractID && flow.Sign() != 0 {
			return nil, nil
		}
	}

	exchange.Input.Amount = paid.String()

	return exchange, nil
}
