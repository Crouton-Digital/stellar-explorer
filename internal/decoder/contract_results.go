package decoder

import (
	"crypto/sha256"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func contractInvocationReturn(meta xdr.TransactionMeta) *xdr.ScVal {
	switch meta.V {
	case 3:
		if soroban := meta.MustV3().SorobanMeta; soroban != nil {
			return &soroban.ReturnValue
		}
	case 4:
		if soroban := meta.MustV4().SorobanMeta; soroban != nil {
			return soroban.ReturnValue
		}
	}

	return nil
}

func contractInvocationResultMatches(
	meta xdr.TransactionMeta,
	result xdr.OperationResult,
	events []xdr.ContractEvent,
) bool {
	returned := contractInvocationReturn(meta)
	if returned == nil {
		return false
	}

	preimage := xdr.InvokeHostFunctionSuccessPreImage{
		ReturnValue: *returned,
		Events:      events,
	}
	raw, err := preimage.MarshalBinary()
	if err != nil {
		return false
	}

	hash := sha256.Sum256(raw)
	return result.Tr.MustInvokeHostFunctionResult().MustSuccess() == xdr.Hash(hash)
}
