package decoder

import (
	"encoding/hex"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func stateKey(key xdr.LedgerKey) (string, any) {
	switch key.Type {
	case xdr.LedgerEntryTypeAccount:
		return key.MustAccount().AccountId.Address(), nil
	case xdr.LedgerEntryTypeTrustline:
		line := key.MustTrustLine()

		return line.AccountId.Address(), trustlineAsset(line.Asset)
	case xdr.LedgerEntryTypeContractData:
		data := key.MustContractData()
		address, _ := data.Contract.String()

		return address, map[string]any{
			"key":        Value(data.Key),
			"durability": strings.TrimPrefix(data.Durability.String(), "ContractDataDurability"),
		}
	case xdr.LedgerEntryTypeContractCode:
		hash := key.MustContractCode().Hash

		return hex.EncodeToString(hash[:]), nil
	case xdr.LedgerEntryTypeTtl:
		hash := key.MustTtl().KeyHash

		return hex.EncodeToString(hash[:]), nil
	case xdr.LedgerEntryTypeOffer:
		offer := key.MustOffer()

		return offer.SellerId.Address(), strconv.FormatInt(int64(offer.OfferId), 10)
	case xdr.LedgerEntryTypeData:
		data := key.MustData()

		return data.AccountId.Address(), string(data.DataName)
	default:
		return strings.TrimPrefix(key.Type.String(), "LedgerEntryType"), nil
	}
}

func stateSnapshot(entry xdr.LedgerEntry) *domain.StateSnapshot {
	fields := map[string]any{}
	projection := "supported_fields"
	switch entry.Data.Type {
	case xdr.LedgerEntryTypeAccount:
		account := entry.Data.MustAccount()
		fields["balance_stroops"] = strconv.FormatInt(int64(account.Balance), 10)
		fields["sequence"] = strconv.FormatInt(int64(account.SeqNum), 10)
		fields["flags"] = strconv.FormatUint(uint64(account.Flags), 10)
		fields["subentries"] = strconv.FormatUint(uint64(account.NumSubEntries), 10)
		fields["home_domain"] = string(account.HomeDomain)
	case xdr.LedgerEntryTypeTrustline:
		line := entry.Data.MustTrustLine()
		fields["balance_base_units"] = strconv.FormatInt(int64(line.Balance), 10)
		fields["limit_base_units"] = strconv.FormatInt(int64(line.Limit), 10)
		fields["flags"] = strconv.FormatUint(uint64(line.Flags), 10)
	case xdr.LedgerEntryTypeContractData:
		data := entry.Data.MustContractData()
		fields["value"] = stateValue(data.Val)
	case xdr.LedgerEntryTypeContractCode:
		code := entry.Data.MustContractCode()
		fields["wasm_hash"] = hex.EncodeToString(code.Hash[:])
		fields["wasm_bytes"] = strconv.Itoa(len(code.Code))
	case xdr.LedgerEntryTypeTtl:
		fields["live_until_ledger"] = strconv.FormatUint(
			uint64(entry.Data.MustTtl().LiveUntilLedgerSeq), 10)
	case xdr.LedgerEntryTypeOffer:
		offer := entry.Data.MustOffer()
		fields["selling"] = offer.Selling.StringCanonical()
		fields["buying"] = offer.Buying.StringCanonical()
		fields["amount_base_units"] = strconv.FormatInt(int64(offer.Amount), 10)
		fields["price_numerator"] = strconv.FormatInt(int64(offer.Price.N), 10)
		fields["price_denominator"] = strconv.FormatInt(int64(offer.Price.D), 10)
		fields["flags"] = strconv.FormatUint(uint64(offer.Flags), 10)
	case xdr.LedgerEntryTypeData:
		fields["value_hex"] = hex.EncodeToString(entry.Data.MustData().DataValue)
	default:
		projection = "xdr_only"
	}

	return &domain.StateSnapshot{
		LastModifiedLedger: uint32(entry.LastModifiedLedgerSeq),
		Projection:         projection,
		Fields:             readableStateValue(fields).(map[string]any),
	}
}

// Ledger strings can contain arbitrary bytes that JSONB cannot represent exactly.
// Preserve those bytes as hex; source metadata XDR remains the canonical evidence.
func readableStateValue(value any) any {
	switch content := value.(type) {
	case string:
		if strings.ContainsRune(content, '\x00') || !utf8.ValidString(content) {
			return map[string]any{
				"encoding":  "hex",
				"value_hex": hex.EncodeToString([]byte(content)),
			}
		}

		return content
	case map[string]any:
		for key, item := range content {
			content[key] = readableStateValue(item)
		}

		return content
	case []any:
		for index, item := range content {
			content[index] = readableStateValue(item)
		}

		return content
	default:
		return value
	}
}

func stateValue(value xdr.ScVal) any {
	if value.Type != xdr.ScValTypeScvContractInstance {
		return Value(value)
	}

	instance := value.MustInstance()
	fields := map[string]any{
		"executable": strings.TrimPrefix(
			instance.Executable.Type.String(),
			"ContractExecutableType",
		),
		"storage": []any{},
	}
	if instance.Executable.Type == xdr.ContractExecutableTypeContractExecutableWasm {
		hash := instance.Executable.MustWasmHash()
		fields["wasm_hash"] = hex.EncodeToString(hash[:])
	}

	if instance.Storage != nil {
		storage := []any{}
		for _, item := range *instance.Storage {
			storage = append(storage, map[string]any{
				"key":   Value(item.Key),
				"value": Value(item.Val),
			})
		}

		fields["storage"] = storage
	}

	return fields
}

func trustlineAsset(asset xdr.TrustLineAsset) string {
	if asset.Type == xdr.AssetTypeAssetTypePoolShare {
		id := asset.MustLiquidityPoolId()

		return "pool:" + hex.EncodeToString(id[:])
	}

	return asset.ToAsset().StringCanonical()
}
