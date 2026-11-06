package decoder

import (
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func soroswapPath(value xdr.ScVal) ([]string, bool) {
	items, valid := value.GetVec()
	if !valid || items == nil || len(*items) < 2 || len(*items) > soroswapMaximumPath {
		return nil, false
	}

	path := []string{}
	seen := map[string]bool{}
	for _, item := range *items {
		address, valid := contractAddressValue(item)
		if !valid || !strkey.IsValidContractAddress(address) || seen[address] {
			return nil, false
		}

		path = append(path, address)
		seen[address] = true
	}

	return path, true
}

func soroswapAmounts(value xdr.ScVal) ([]string, bool) {
	items, valid := value.GetVec()
	if !valid || items == nil || len(*items) < 2 || len(*items) > soroswapMaximumPath {
		return nil, false
	}

	amounts := []string{}
	for _, item := range *items {
		quantity, valid := contractI128Quantity(item, true)
		if !valid {
			return nil, false
		}

		amounts = append(amounts, quantity)
	}

	return amounts, true
}

func soroswapReturn(meta xdr.TransactionMeta) ([]string, bool) {
	switch meta.V {
	case 3:
		if soroban := meta.MustV3().SorobanMeta; soroban != nil {
			return soroswapAmounts(soroban.ReturnValue)
		}
	case 4:
		if soroban := meta.MustV4().SorobanMeta; soroban != nil && soroban.ReturnValue != nil {
			return soroswapAmounts(*soroban.ReturnValue)
		}
	}

	return nil, false
}

func soroswapEvent(events []xdr.ContractEvent) (map[string]xdr.ScVal, int, bool) {
	var found map[string]xdr.ScVal
	index := -1
	for offset, event := range events {
		if event.ContractId == nil ||
			strkey.MustEncode(strkey.VersionByteContract, event.ContractId[:]) != soroswapRouter ||
			event.Type != xdr.ContractEventTypeContract || event.Body.V != 0 || event.Body.V0 == nil {
			continue
		}

		topics := event.Body.V0.Topics
		if len(topics) < 2 {
			continue
		}

		name, valid := topics[1].GetSym()
		if !valid || name != "swap" {
			continue
		}

		// Any additional router swap, even malformed, makes attribution ambiguous.
		if index != -1 {
			return nil, -1, false
		}

		prefix, valid := topics[0].GetStr()
		if !valid || prefix != "SoroswapRouter" || len(topics) != 2 {
			return nil, -1, false
		}

		entries, valid := event.Body.V0.Data.GetMap()
		if !valid || entries == nil || len(*entries) != 3 {
			return nil, -1, false
		}

		found = map[string]xdr.ScVal{}
		for _, entry := range *entries {
			key, valid := entry.Key.GetSym()
			if !valid || (key != "amounts" && key != "path" && key != "to") {
				return nil, -1, false
			}

			if _, exists := found[string(key)]; exists {
				return nil, -1, false
			}

			found[string(key)] = entry.Val
		}

		index = offset
	}

	return found, index, index >= 0
}
