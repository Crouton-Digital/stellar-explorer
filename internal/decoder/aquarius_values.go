package decoder

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"slices"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func aquariusQuantity(value xdr.ScVal, positive bool) (string, bool) {
	parts, valid := value.GetU128()
	if !valid {
		return "", false
	}

	quantity := new(big.Int).SetUint64(uint64(parts.Hi))
	quantity.Lsh(quantity, 64)
	quantity.Add(quantity, new(big.Int).SetUint64(uint64(parts.Lo)))
	if positive && quantity.Sign() == 0 {
		return "", false
	}

	return quantity.String(), true
}

func aquariusContract(value xdr.ScVal) (string, bool) {
	address, valid := contractAddressValue(value)
	return address, valid && strkey.IsValidContractAddress(address)
}

func aquariusPoolTokens(value xdr.ScVal) ([]string, bool) {
	items, valid := value.GetVec()
	if !valid || items == nil || len(*items) < 2 || len(*items) > aquariusMaximumTokens {
		return nil, false
	}

	tokens := []string{}
	var previous []byte
	for _, item := range *items {
		address, valid := aquariusContract(item)
		if !valid {
			return nil, false
		}

		raw, err := strkey.Decode(strkey.VersionByteContract, address)
		if err != nil || (previous != nil && bytes.Compare(previous, raw) >= 0) {
			return nil, false
		}

		previous = raw
		tokens = append(tokens, address)
	}

	return tokens, true
}

func aquariusPoolRequest(
	tokensValue, indexValue, inputValue, outputValue xdr.ScVal,
) (aquariusHopRequest, bool) {
	tokens, valid := aquariusPoolTokens(tokensValue)
	if !valid {
		return aquariusHopRequest{}, false
	}

	index, valid := indexValue.GetBytes()
	if !valid || len(index) != 32 {
		return aquariusHopRequest{}, false
	}

	input, valid := aquariusContract(inputValue)
	if !valid || !slices.Contains(tokens, input) {
		return aquariusHopRequest{}, false
	}

	output, valid := aquariusContract(outputValue)
	if !valid || input == output || !slices.Contains(tokens, output) {
		return aquariusHopRequest{}, false
	}

	return aquariusHopRequest{
		tokens:      tokens,
		poolIndex:   hex.EncodeToString(index),
		inputToken:  input,
		outputToken: output,
	}, true
}

func aquariusChainRequest(chain, input xdr.ScVal) ([]aquariusHopRequest, bool) {
	steps, valid := chain.GetVec()
	if !valid || steps == nil || len(*steps) == 0 || len(*steps) > aquariusMaximumHops {
		return nil, false
	}

	requests := []aquariusHopRequest{}
	for _, step := range *steps {
		fields, valid := step.GetVec()
		if !valid || fields == nil || len(*fields) != 3 {
			return nil, false
		}

		request, valid := aquariusPoolRequest((*fields)[0], (*fields)[1], input, (*fields)[2])
		if !valid {
			return nil, false
		}

		requests = append(requests, request)
		input = (*fields)[2]
	}

	return requests, true
}
