package decoder

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// RebuildContractMetadata shares the live decoder with migration of saved source XDR.
func RebuildContractMetadata(t *domain.Transaction) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("invalid contract metadata XDR: %v", recovered)
		}
	}()

	var env xdr.TransactionEnvelope
	if err = xdr.SafeUnmarshalBase64(t.EnvelopeXDR, &env); err != nil {
		return fmt.Errorf("contract envelope: %w", err)
	}

	var meta xdr.TransactionMeta
	if err = xdr.SafeUnmarshalBase64(t.MetaXDR, &meta); err != nil {
		return fmt.Errorf("contract metadata: %w", err)
	}

	return contractMetadata(t, env, meta)
}

func contractMetadata(
	t *domain.Transaction,
	env xdr.TransactionEnvelope,
	meta xdr.TransactionMeta,
) error {
	observed := map[string]*domain.Contract{}
	observe := func(address string) *domain.Contract {
		if !strkey.IsValidContractAddress(address) {
			return nil
		}

		if existing := observed[address]; existing != nil {
			return existing
		}

		contract := &domain.Contract{
			Network:             t.Network,
			ContractID:          address,
			Kind:                "unknown",
			FirstObservedLedger: t.LedgerSequence,
			LastObservedLedger:  t.LedgerSequence,
		}
		observed[address] = contract

		return contract
	}

	for _, call := range t.Calls {
		observe(call.ContractID)
	}

	for _, event := range t.Events {
		if event.Scope != "transaction" {
			observe(event.ContractID)
		}
	}

	for _, movement := range t.TokenTransfers {
		contract := observe(movement.Asset.ContractID)
		if contract != nil && movement.Asset.Kind == "sac" {
			contract.Kind = "sac"
			contract.ExecutableSource = pointer("asset_contract_id")
			contract.ExecutableLedger = pointer(t.LedgerSequence)
			contract.ExecutableTxIndex = pointer(t.TxIndex)
		}

		observe(movement.From)
		observe(movement.To)
	}

	if t.Successful {
		before, operations, after, err := contractChanges(meta)
		if err != nil {
			return err
		}

		apply := func(changes xdr.LedgerEntryChanges, canCreate bool) error {
			for _, change := range changes {
				var entry *xdr.LedgerEntry
				switch change.Type {
				case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
					entry = change.Created
				case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
					entry = change.Updated
				case xdr.LedgerEntryChangeTypeLedgerEntryState:
					entry = change.State
				case xdr.LedgerEntryChangeTypeLedgerEntryRestored:
					entry = change.Restored
				}

				if entry == nil || entry.Data.Type != xdr.LedgerEntryTypeContractData {
					continue
				}

				data := entry.Data.MustContractData()
				if data.Key.Type != xdr.ScValTypeScvLedgerKeyContractInstance ||
					data.Val.Type != xdr.ScValTypeScvContractInstance ||
					data.Durability != xdr.ContractDataDurabilityPersistent {
					continue
				}

				address, err := data.Contract.String()
				if err != nil {
					return err
				}

				contract := observe(address)
				if contract == nil {
					continue
				}

				executable := data.Val.MustInstance().Executable
				contract.Kind = "unknown"
				contract.WasmHash = nil
				switch executable.Type {
				case xdr.ContractExecutableTypeContractExecutableWasm:
					hash := executable.MustWasmHash()
					contract.Kind = "wasm"
					contract.WasmHash = pointer(hex.EncodeToString(hash[:]))
				case xdr.ContractExecutableTypeContractExecutableStellarAsset:
					contract.Kind = "sac"
				}

				contract.ExecutableSource = pointer("instance_entry")
				contract.ExecutableLedger = pointer(t.LedgerSequence)
				contract.ExecutableTxIndex = pointer(t.TxIndex)
				if canCreate && change.Type == xdr.LedgerEntryChangeTypeLedgerEntryCreated &&
					uint32(entry.LastModifiedLedgerSeq) == t.LedgerSequence {
					contract.CreationLedger = pointer(t.LedgerSequence)
					contract.CreationTransaction = pointer(t.Hash)
				}
			}

			return nil
		}

		if err = apply(before, false); err != nil {
			return err
		}

		for index, changes := range operations {
			ops := env.Operations()
			canCreate := index < len(ops) &&
				ops[index].Body.Type == xdr.OperationTypeInvokeHostFunction
			if err = apply(changes, canCreate); err != nil {
				return err
			}
		}

		if err = apply(after, false); err != nil {
			return err
		}

		// Only a matching top-level creation preimage identifies the deployer here.
		// Factory-created contracts keep an unknown creator until stronger evidence is decoded.
		for _, op := range env.Operations() {
			if op.Body.Type != xdr.OperationTypeInvokeHostFunction {
				continue
			}

			fn := op.Body.MustInvokeHostFunctionOp().HostFunction
			var preimage xdr.ContractIdPreimage
			switch fn.Type {
			case xdr.HostFunctionTypeHostFunctionTypeCreateContract:
				preimage = fn.MustCreateContract().ContractIdPreimage
			case xdr.HostFunctionTypeHostFunctionTypeCreateContractV2:
				preimage = fn.MustCreateContractV2().ContractIdPreimage
			default:
				continue
			}

			id, err := contractID(preimage)
			if err != nil {
				return err
			}

			contract := observed[id]
			if contract == nil || contract.CreationLedger == nil ||
				preimage.Type != xdr.ContractIdPreimageTypeContractIdPreimageFromAddress {
				continue
			}

			creator, err := preimage.MustFromAddress().Address.String()
			if err != nil {
				return err
			}

			contract.Creator = &creator
		}
	}

	t.Contracts = []domain.Contract{}
	for _, contract := range observed {
		t.Contracts = append(t.Contracts, *contract)
	}

	sort.Slice(t.Contracts, func(i, j int) bool {
		return t.Contracts[i].ContractID < t.Contracts[j].ContractID
	})

	return nil
}

func contractChanges(
	meta xdr.TransactionMeta,
) (xdr.LedgerEntryChanges, []xdr.LedgerEntryChanges, xdr.LedgerEntryChanges, error) {
	var before, after xdr.LedgerEntryChanges
	var operations []xdr.OperationMeta
	switch meta.V {
	case 0:
		operations = meta.MustOperations()
	case 1:
		v := meta.MustV1()
		before, operations = v.TxChanges, v.Operations
	case 2:
		v := meta.MustV2()
		before, operations, after = v.TxChangesBefore, v.Operations, v.TxChangesAfter
	case 3:
		v := meta.MustV3()
		before, operations, after = v.TxChangesBefore, v.Operations, v.TxChangesAfter
	case 4:
		v := meta.MustV4()
		changes := []xdr.LedgerEntryChanges{}
		for _, op := range v.Operations {
			changes = append(changes, op.Changes)
		}

		return v.TxChangesBefore, changes, v.TxChangesAfter, nil
	default:
		return nil, nil, nil, fmt.Errorf("unsupported contract metadata version %d", meta.V)
	}

	changes := []xdr.LedgerEntryChanges{}
	for _, op := range operations {
		changes = append(changes, op.Changes)
	}

	return before, changes, after, nil
}

func contractID(preimage xdr.ContractIdPreimage) (string, error) {
	value := xdr.HashIdPreimage{
		Type: xdr.EnvelopeTypeEnvelopeTypeContractId,
		ContractId: &xdr.HashIdPreimageContractId{
			NetworkId:          network.ID(domain.Passphrase),
			ContractIdPreimage: preimage,
		},
	}
	raw, err := value.MarshalBinary()
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(raw)

	return strkey.Encode(strkey.VersionByteContract, hash[:])
}

func pointer[T any](value T) *T {
	return &value
}
