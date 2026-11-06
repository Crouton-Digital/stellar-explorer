package decoder

import (
	"math/big"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func contractAddressValue(value xdr.ScVal) (string, bool) {
	address, valid := value.GetAddress()
	if !valid || (address.Type != xdr.ScAddressTypeScAddressTypeAccount &&
		address.Type != xdr.ScAddressTypeScAddressTypeContract) {
		return "", false
	}

	return tokenAddress(value)
}

func contractI128Quantity(value xdr.ScVal, positive bool) (string, bool) {
	if value.Type != xdr.ScValTypeScvI128 {
		return "", false
	}

	quantity, _, valid := tokenAmount(value)
	if !valid {
		return "", false
	}

	number, _ := new(big.Int).SetString(quantity, 10)
	if positive && number.Sign() <= 0 {
		return "", false
	}

	return quantity, true
}
