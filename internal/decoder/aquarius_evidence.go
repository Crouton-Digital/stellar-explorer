package decoder

import (
	"encoding/hex"
	"slices"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type aquariusRouterEvent struct {
	index        int
	pool         string
	inputAmount  string
	outputAmount string
}

func aquariusRouterEvents(
	events []xdr.ContractEvent,
	trader string,
	requests []aquariusHopRequest,
) ([]aquariusRouterEvent, bool) {
	matched := []aquariusRouterEvent{}
	pools := map[string]bool{}
	for index, event := range events {
		if event.ContractId == nil ||
			strkey.MustEncode(strkey.VersionByteContract, event.ContractId[:]) != aquariusRouter ||
			event.Type != xdr.ContractEventTypeContract || event.Body.V != 0 || event.Body.V0 == nil {
			continue
		}

		topics := event.Body.V0.Topics
		if len(topics) == 0 {
			continue
		}

		name, valid := topics[0].GetSym()
		if !valid || name != "swap" {
			continue
		}

		if len(topics) != 3 || len(matched) >= len(requests) {
			return nil, false
		}

		request := requests[len(matched)]
		tokens, valid := aquariusPoolTokens(topics[1])
		if !valid || !slices.Equal(tokens, request.tokens) {
			return nil, false
		}

		actor, valid := contractAddressValue(topics[2])
		if !valid || actor != trader {
			return nil, false
		}

		data, valid := event.Body.V0.Data.GetVec()
		if !valid || data == nil || len(*data) != 5 {
			return nil, false
		}

		pool, valid := aquariusContract((*data)[0])
		if !valid || pool == trader || pool == aquariusRouter || pools[pool] {
			return nil, false
		}

		input, valid := aquariusContract((*data)[1])
		if !valid || input != request.inputToken {
			return nil, false
		}

		output, valid := aquariusContract((*data)[2])
		if !valid || output != request.outputToken {
			return nil, false
		}

		inputAmount, valid := aquariusQuantity((*data)[3], true)
		if !valid {
			return nil, false
		}

		outputAmount, valid := aquariusQuantity((*data)[4], true)
		if !valid {
			return nil, false
		}

		pools[pool] = true
		matched = append(matched, aquariusRouterEvent{
			index:        index,
			pool:         pool,
			inputAmount:  inputAmount,
			outputAmount: outputAmount,
		})
	}

	return matched, len(matched) == len(requests)
}

func aquariusPoolTrade(
	event xdr.ContractEvent,
	request aquariusHopRequest,
	router aquariusRouterEvent,
	settlement string,
) bool {
	if event.ContractId == nil ||
		strkey.MustEncode(strkey.VersionByteContract, event.ContractId[:]) != router.pool ||
		event.Type != xdr.ContractEventTypeContract || event.Body.V != 0 || event.Body.V0 == nil {
		return false
	}

	topics := event.Body.V0.Topics
	if len(topics) != 4 {
		return false
	}

	name, valid := topics[0].GetSym()
	if !valid || name != "trade" {
		return false
	}

	for index, expected := range []string{request.inputToken, request.outputToken, settlement} {
		address, valid := contractAddressValue(topics[index+1])
		if !valid || address != expected {
			return false
		}
	}

	data, valid := event.Body.V0.Data.GetVec()
	if !valid || data == nil || len(*data) != 3 {
		return false
	}

	input, valid := contractI128Quantity((*data)[0], true)
	if !valid || input != router.inputAmount {
		return false
	}

	output, valid := contractI128Quantity((*data)[1], true)
	if !valid || output != router.outputAmount {
		return false
	}

	// The final field is preserved in raw evidence; its fee denomination varies by pool ABI.
	_, valid = contractI128Quantity((*data)[2], false)
	return valid
}

func aquariusCodeEvidence(meta xdr.TransactionMeta) (*string, bool) {
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
			case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
				if key := change.Removed; key != nil &&
					key.Type == xdr.LedgerEntryTypeContractData {
					data := key.MustContractData()
					address, err := data.Contract.String()
					if err == nil && address == aquariusRouter &&
						data.Key.Type == xdr.ScValTypeScvLedgerKeyContractInstance {
						return nil, false
					}
				}
			}

			if entry == nil || entry.Data.Type != xdr.LedgerEntryTypeContractData {
				continue
			}

			data := entry.Data.MustContractData()
			address, err := data.Contract.String()
			if err != nil || address != aquariusRouter ||
				data.Key.Type != xdr.ScValTypeScvLedgerKeyContractInstance {
				continue
			}

			if data.Durability != xdr.ContractDataDurabilityPersistent ||
				data.Val.Type != xdr.ScValTypeScvContractInstance ||
				data.Val.MustInstance().Executable.Type != xdr.ContractExecutableTypeContractExecutableWasm {
				return nil, false
			}

			hash := data.Val.MustInstance().Executable.MustWasmHash()
			encoded := hex.EncodeToString(hash[:])
			if observed != nil && *observed != encoded {
				return nil, false
			}

			observed = &encoded
		}
	}

	return observed, true
}
