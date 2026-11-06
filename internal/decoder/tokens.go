package decoder

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/amount"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// RebuildTokenActivity enriches retained transactions without changing their source XDR.
// Only committed operation events are considered; diagnostics never create movements.
func RebuildTokenActivity(t *domain.Transaction) error {
	t.TokenTransfers = []domain.TokenTransfer{}
	if !t.Successful || len(t.Events) == 0 {
		return nil
	}

	var meta xdr.TransactionMeta
	if e := xdr.SafeUnmarshalBase64(t.MetaXDR, &meta); e != nil {
		return fmt.Errorf("token metadata: %w", e)
	}

	participants := map[string]bool{}
	for _, address := range t.Participants {
		participants[address] = true
	}

	for _, event := range t.Events {
		var raw xdr.ContractEvent
		if e := xdr.SafeUnmarshalBase64(event.RawXDR, &raw); e != nil {
			return fmt.Errorf("token event %d: %w", event.Index, e)
		}

		appendTokenActivity(t, event, raw, meta.V, participants)
	}

	t.Participants = []string{}
	for address := range participants {
		t.Participants = append(t.Participants, address)
	}

	sort.Strings(t.Participants)

	return nil
}

func appendTokenActivity(
	t *domain.Transaction,
	event domain.Event,
	raw xdr.ContractEvent,
	metaVersion int32,
	participants map[string]bool,
) {
	movement, ok := tokenEvent(raw, event, metaVersion)
	if !ok {
		return
	}

	movement.TxIndex = t.TxIndex
	movement.CreatedAt = t.CreatedAt
	t.TokenTransfers = append(t.TokenTransfers, movement)

	for _, address := range []string{movement.From, movement.To} {
		if strkey.IsValidEd25519PublicKey(address) {
			participants[address] = true
		}
	}
}

func tokenEvent(
	raw xdr.ContractEvent,
	event domain.Event,
	metaVersion int32,
) (domain.TokenTransfer, bool) {
	movement := domain.TokenTransfer{}
	if event.Scope == "transaction" || event.OpIndex == nil ||
		raw.Type != xdr.ContractEventTypeContract || raw.ContractId == nil ||
		raw.Body.V != 0 || raw.Body.V0 == nil || (metaVersion != 3 && metaVersion != 4) {
		return movement, false
	}

	topics := raw.Body.V0.Topics
	if len(topics) < 2 {
		return movement, false
	}

	name, ok := topics[0].GetSym()
	if !ok {
		return movement, false
	}

	quantity, muxed, ok := tokenAmount(raw.Body.V0.Data)
	if !ok {
		return movement, false
	}

	asset := tokenAsset(raw, event.LedgerSequence, metaVersion, string(name))
	movement = domain.TokenTransfer{
		TransactionHash: event.TransactionHash,
		LedgerSequence:  event.LedgerSequence,
		EventIndex:      event.Index,
		OpIndex:         *event.OpIndex,
		Kind:            string(name),
		Asset:           asset,
		Amount:          quantity,
	}

	switch string(name) {
	case "transfer":
		if len(topics) < 3 {
			return movement, false
		}

		movement.From, ok = tokenAddress(topics[1])
		if !ok {
			return movement, false
		}

		movement.To, ok = tokenAddress(topics[2])
		movement.ToMuxed = muxed

	case "burn":
		movement.From, ok = tokenAddress(topics[1])

	case "mint", "clawback":
		// Custom mint/clawback provenance is not implemented; verify SAC events only.
		if asset.Kind != "sac" {
			return movement, false
		}

		addressIndex := 1
		if metaVersion == 3 {
			addressIndex = 2
			if len(topics) < 3 {
				return movement, false
			}

			if _, valid := tokenAddress(topics[1]); !valid {
				return movement, false
			}
		}

		address, valid := tokenAddress(topics[addressIndex])
		ok = valid
		if string(name) == "mint" {
			movement.To = address
		} else {
			movement.From = address
		}

	default:
		return movement, false
	}

	if !ok {
		return movement, false
	}

	// Before protocol 23, SAC transfers involving the issuer represented mint/burn.
	if metaVersion == 3 && movement.Kind == "transfer" && asset.Issuer != nil {
		if movement.From == *asset.Issuer {
			movement.Kind = "mint"
			movement.From = ""
		} else if movement.To == *asset.Issuer {
			movement.Kind = "burn"
			movement.To = ""
		}
	}

	return movement, true
}

func tokenAddress(value xdr.ScVal) (string, bool) {
	address, ok := value.GetAddress()
	if !ok {
		return "", false
	}

	encoded, e := address.String()

	return encoded, e == nil
}

func tokenAmount(value xdr.ScVal) (string, any, bool) {
	var muxed any
	if entries, isMap := value.GetMap(); isMap {
		if entries == nil {
			return "", nil, false
		}

		found := false
		seen := map[xdr.ScSymbol]bool{}
		for _, entry := range *entries {
			key, ok := entry.Key.GetSym()
			if !ok || seen[key] {
				return "", nil, false
			}

			seen[key] = true
			switch key {
			case "amount":
				value = entry.Val
				found = true

			case "to_muxed_id":
				switch entry.Val.Type {
				case xdr.ScValTypeScvU64, xdr.ScValTypeScvString, xdr.ScValTypeScvBytes:
					muxed = Value(entry.Val)

				case xdr.ScValTypeScvVoid:

				default:
					return "", nil, false
				}
			}
		}

		if !found {
			return "", nil, false
		}
	}

	parts, ok := value.GetI128()
	if !ok {
		return "", nil, false
	}

	quantity := amount.String128Raw(parts)
	n, valid := new(big.Int).SetString(quantity, 10)
	if !valid || n.Sign() < 0 {
		return "", nil, false
	}

	return quantity, muxed, true
}

func tokenAsset(raw xdr.ContractEvent, ledger uint32, metaVersion int32, name string) domain.Asset {
	address, _ := strkey.Encode(strkey.VersionByteContract, raw.ContractId[:])
	asset := domain.Asset{
		Network:             domain.Network,
		ContractID:          address,
		Kind:                "contract",
		Verification:        "event_shape",
		FirstObservedLedger: ledger,
	}

	candidates := []xdr.Asset{xdr.MustNewNativeAsset()}
	topics := raw.Body.V0.Topics
	expected := 3
	if name == "transfer" || (metaVersion == 3 && (name == "mint" || name == "clawback")) {
		expected = 4
	}

	if len(topics) == expected {
		if text, ok := topics[len(topics)-1].GetStr(); ok {
			parsed, e := xdr.BuildAssets(string(text))
			if e == nil && len(parsed) == 1 {
				candidates = append(candidates, parsed[0])
			}
		}
	}

	for _, candidate := range candidates {
		id, e := candidate.ContractID(domain.Passphrase)
		if e != nil || id != [32]byte(*raw.ContractId) {
			continue
		}

		canonical := candidate.StringCanonical()
		code := strings.TrimRight(candidate.GetCode(), "\x00")
		decimals := uint32(7)
		asset.Kind = "sac"
		asset.Verification = "contract_id"
		asset.ClassicAsset = &canonical
		asset.Decimals = &decimals
		if candidate.Type == xdr.AssetTypeAssetTypeNative {
			code = "XLM"
		} else {
			issuer := candidate.GetIssuer()
			asset.Issuer = &issuer
		}

		asset.Code = &code
		break
	}

	return asset
}
