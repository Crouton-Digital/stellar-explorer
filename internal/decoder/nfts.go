package decoder

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// RebuildNFTActivity derives NFT events from retained source without changing other payload fields.
func RebuildNFTActivity(t *domain.Transaction) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("invalid NFT source XDR: %v", recovered)
		}
	}()

	if t.Network != domain.Network {
		return fmt.Errorf("NFT source network is not %s", domain.Network)
	}

	var envelope xdr.TransactionEnvelope
	if err = xdr.SafeUnmarshalBase64(t.EnvelopeXDR, &envelope); err != nil {
		return fmt.Errorf("NFT envelope: %w", err)
	}

	var result xdr.TransactionResult
	if err = xdr.SafeUnmarshalBase64(t.ResultXDR, &result); err != nil {
		return fmt.Errorf("NFT result: %w", err)
	}

	var meta xdr.TransactionMeta
	if err = xdr.SafeUnmarshalBase64(t.MetaXDR, &meta); err != nil {
		return fmt.Errorf("NFT metadata: %w", err)
	}

	hash, err := network.HashTransactionInEnvelope(envelope, domain.Passphrase)
	if err != nil || hex.EncodeToString(hash[:]) != t.Hash || result.Successful() != t.Successful {
		return fmt.Errorf("NFT source does not match transaction identity or status")
	}

	if envelope.SourceAccount().ToAccountId().Address() != t.SourceAccount ||
		envelope.FeeAccount().ToAccountId().Address() != t.FeeAccount {
		return fmt.Errorf("NFT source accounts do not match the envelope")
	}

	t.NFTTransfers = nftActivity(t, result, meta, nil)

	return nil
}

func nftActivity(
	t *domain.Transaction,
	result xdr.TransactionResult,
	meta xdr.TransactionMeta,
	participants map[string]bool,
) []domain.NFTTransfer {
	movements := []domain.NFTTransfer{}
	if !t.Successful || !result.Successful() || (meta.V != 3 && meta.V != 4) {
		return movements
	}

	results, valid := result.OperationResults()
	if !valid {
		return movements
	}

	eventIndex := 0
	for operation, events := range operationEvents(meta) {
		candidates := []domain.NFTTransfer{}
		for _, event := range events {
			if movement, valid := nftEvent(event); valid {
				movement.TransactionHash = t.Hash
				movement.LedgerSequence = t.LedgerSequence
				movement.TxIndex = t.TxIndex
				movement.EventIndex = eventIndex
				movement.OpIndex = operation
				movement.CreatedAt = t.CreatedAt
				candidates = append(candidates, movement)
			}

			eventIndex++
		}

		if len(candidates) == 0 || operation >= len(results) ||
			!nftResultMatches(meta, results[operation], events) {
			continue
		}

		movements = append(movements, candidates...)
		if participants != nil {
			for _, movement := range candidates {
				for _, address := range []string{movement.From, movement.To} {
					if strkey.IsValidEd25519PublicKey(address) {
						participants[address] = true
					}
				}
			}
		}
	}

	return movements
}

func nftResultMatches(
	meta xdr.TransactionMeta,
	result xdr.OperationResult,
	events []xdr.ContractEvent,
) bool {
	if result.Code != xdr.OperationResultCodeOpInner || result.Tr == nil ||
		result.Tr.Type != xdr.OperationTypeInvokeHostFunction {
		return false
	}

	invocation := result.Tr.MustInvokeHostFunctionResult()
	if invocation.Code != xdr.InvokeHostFunctionResultCodeInvokeHostFunctionSuccess {
		return false
	}

	return contractInvocationResultMatches(meta, result, events)
}

func nftEvent(event xdr.ContractEvent) (domain.NFTTransfer, bool) {
	movement := domain.NFTTransfer{}
	if event.Type != xdr.ContractEventTypeContract || event.ContractId == nil ||
		event.Body.V != 0 || event.Body.V0 == nil {
		return movement, false
	}

	topics := event.Body.V0.Topics
	if len(topics) < 2 {
		return movement, false
	}

	name, valid := topics[0].GetSym()
	if !valid {
		return movement, false
	}

	identifier, identifierType, format, valid := nftIdentifier(event.Body.V0.Data)
	if !valid {
		return movement, false
	}

	movement = domain.NFTTransfer{
		Kind:          string(name),
		ContractID:    strkey.MustEncode(strkey.VersionByteContract, event.ContractId[:]),
		TokenID:       identifier,
		TokenIDType:   identifierType,
		Verification:  "event_shape",
		EventStandard: "sep50",
		Format:        format,
	}
	nativeID, err := xdr.MustNewNativeAsset().ContractID(domain.Passphrase)
	if err != nil || [32]byte(*event.ContractId) == nativeID {
		return movement, false
	}

	switch name {
	case "transfer":
		if len(topics) != 3 {
			return movement, false
		}

		movement.From, valid = contractAddressValue(topics[1])
		if !valid {
			return movement, false
		}

		movement.To, valid = contractAddressValue(topics[2])
	case "mint":
		if len(topics) != 2 {
			return movement, false
		}

		movement.To, valid = contractAddressValue(topics[1])
	case "burn":
		if len(topics) != 2 {
			return movement, false
		}

		// Burn is an OpenZeppelin extension, not an event defined by draft SEP-50.
		movement.EventStandard = "openzeppelin_burnable"
		movement.From, valid = contractAddressValue(topics[1])
	default:
		return movement, false
	}

	return movement, valid
}

func nftIdentifier(value xdr.ScVal) (string, string, string, bool) {
	format := "scalar"
	if entries, isMap := value.GetMap(); isMap {
		if entries == nil || len(*entries) != 1 {
			return "", "", "", false
		}

		key, valid := (*entries)[0].Key.GetSym()
		if !valid || key != "token_id" {
			return "", "", "", false
		}

		value = (*entries)[0].Val
		format = "token_id_map"
	}

	var limbs []uint64
	identifierType := ""
	switch value.Type {
	case xdr.ScValTypeScvU32:
		if value.U32 == nil {
			return "", "", "", false
		}

		return strconv.FormatUint(uint64(*value.U32), 10), "u32", format, true
	case xdr.ScValTypeScvU64:
		if value.U64 == nil {
			return "", "", "", false
		}

		return strconv.FormatUint(uint64(*value.U64), 10), "u64", format, true
	case xdr.ScValTypeScvU128:
		if value.U128 == nil {
			return "", "", "", false
		}

		limbs = []uint64{uint64(value.U128.Hi), uint64(value.U128.Lo)}
		identifierType = "u128"
	case xdr.ScValTypeScvU256:
		if value.U256 == nil {
			return "", "", "", false
		}

		limbs = []uint64{
			uint64(value.U256.HiHi),
			uint64(value.U256.HiLo),
			uint64(value.U256.LoHi),
			uint64(value.U256.LoLo),
		}
		identifierType = "u256"
	default:
		return "", "", "", false
	}

	identifier := new(big.Int)
	for _, limb := range limbs {
		identifier.Lsh(identifier, 64)
		identifier.Add(identifier, new(big.Int).SetUint64(limb))
	}

	return identifier.String(), identifierType, format, true
}
