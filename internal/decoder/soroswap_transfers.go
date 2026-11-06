package decoder

import (
	"bytes"
	"crypto/sha256"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func soroswapTransfers(
	swap *domain.ContractSwap,
	movements []domain.TokenTransfer,
	events []xdr.ContractEvent,
) bool {
	if len(movements) != len(swap.Path) {
		return false
	}

	byIndex := map[int]bool{}
	for _, movement := range movements {
		if movement.OpIndex != 0 || movement.Kind != "transfer" || movement.ToMuxed != nil {
			return false
		}

		byIndex[movement.EventIndex] = true
	}

	// Unrecognized token-shaped events cannot silently disappear from reconciliation.
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

	pools := []string{}
	for index := 0; index < len(swap.Path)-1; index++ {
		pool, err := soroswapPairContract(swap.Path[index], swap.Path[index+1])
		if err != nil || pool == swap.Trader {
			return false
		}

		pools = append(pools, pool)
	}

	previous := -1
	for index, movement := range movements {
		from, to := swap.Trader, swap.Trader
		if index > 0 {
			from = pools[index-1]
		}

		if index < len(pools) {
			to = pools[index]
		}

		if movement.From != from || movement.To != to ||
			movement.Asset.ContractID != swap.Path[index] || movement.Amount != swap.Amounts[index] ||
			movement.EventIndex <= previous || movement.EventIndex >= swap.RouterEventIndex {
			return false
		}

		previous = movement.EventIndex
		swap.TransferEventIndexes = append(swap.TransferEventIndexes, movement.EventIndex)
	}

	leg := func(index int) domain.ExchangeLeg {
		return domain.ExchangeLeg{
			Asset:  movements[index].Asset,
			Amount: movements[index].Amount,
		}
	}
	for index, pool := range pools {
		swap.Hops = append(swap.Hops, domain.ContractSwapHop{
			Index:            index,
			PoolContract:     pool,
			Input:            leg(index),
			Output:           leg(index + 1),
			InputEventIndex:  movements[index].EventIndex,
			OutputEventIndex: movements[index+1].EventIndex,
		})
	}

	swap.Input = leg(0)
	swap.Output = leg(len(movements) - 1)

	return true
}

// Router dependency soroswap-library 0.3.0 salts the factory deployment with the
// concatenated full ScVal-address XDR, sorted by binary ScAddress ordering.
func soroswapPairContract(first, second string) (string, error) {
	addresses := []xdr.ScAddress{}
	for _, encoded := range []string{first, second, soroswapFactory} {
		raw, err := strkey.Decode(strkey.VersionByteContract, encoded)
		if err != nil {
			return "", err
		}

		var id xdr.ContractId
		copy(id[:], raw)
		addresses = append(addresses, xdr.ScAddress{
			Type:       xdr.ScAddressTypeScAddressTypeContract,
			ContractId: &id,
		})
	}

	if bytes.Compare(addresses[0].ContractId[:], addresses[1].ContractId[:]) > 0 {
		addresses[0], addresses[1] = addresses[1], addresses[0]
	}

	var encoded []byte
	for _, address := range addresses[:2] {
		value := xdr.ScVal{
			Type:    xdr.ScValTypeScvAddress,
			Address: &address,
		}
		raw, err := value.MarshalBinary()
		if err != nil {
			return "", err
		}

		encoded = append(encoded, raw...)
	}

	salt := sha256.Sum256(encoded)

	return contractID(xdr.ContractIdPreimage{
		Type: xdr.ContractIdPreimageTypeContractIdPreimageFromAddress,
		FromAddress: &xdr.ContractIdPreimageFromAddress{
			Address: addresses[2],
			Salt:    salt,
		},
	})
}
