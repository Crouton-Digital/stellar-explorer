package decoder

import (
	"fmt"
	"strings"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type observedState struct {
	entry xdr.LedgerEntry
	index int
}

func interpretState(meta xdr.TransactionMeta, successful bool) ([]domain.StateChange, error) {
	before, operations, after, err := contractChanges(meta)
	if err != nil {
		return nil, err
	}

	changes := []domain.StateChange{}
	appendGroup := func(group xdr.LedgerEntryChanges, scope string, operation *int) error {
		decoded, err := stateGroup(group, scope, operation)
		if err != nil {
			return err
		}

		changes = append(changes, decoded...)

		return nil
	}
	if err = appendGroup(before, "transaction_before", nil); err != nil {
		return nil, err
	}

	// Operation effects are atomic and roll back when any operation fails.
	if successful {
		for index, group := range operations {
			if err = appendGroup(group, "operation", pointer(index)); err != nil {
				return nil, err
			}
		}
	}

	if err = appendGroup(after, "transaction_after", nil); err != nil {
		return nil, err
	}

	return changes, nil
}

func stateGroup(
	group xdr.LedgerEntryChanges,
	scope string,
	operation *int,
) ([]domain.StateChange, error) {
	changes := []domain.StateChange{}
	states := map[string]observedState{}
	for index, raw := range group {
		var entry *xdr.LedgerEntry
		var key xdr.LedgerKey
		switch raw.Type {
		case xdr.LedgerEntryChangeTypeLedgerEntryState:
			entry = raw.State
		case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
			entry = raw.Created
		case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
			entry = raw.Updated
		case xdr.LedgerEntryChangeTypeLedgerEntryRestored:
			entry = raw.Restored
		case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
			key = raw.MustRemoved()
		default:
			return nil, fmt.Errorf("unsupported state change type %d", raw.Type)
		}

		if entry != nil {
			var err error
			key, err = entry.LedgerKey()
			if err != nil {
				return nil, err
			}
		} else if raw.Type != xdr.LedgerEntryChangeTypeLedgerEntryRemoved {
			return nil, fmt.Errorf("missing ledger entry at %s change %d", scope, index)
		}

		keyXDR, err := xdr.MarshalBase64(key)
		if err != nil {
			return nil, err
		}

		if raw.Type == xdr.LedgerEntryChangeTypeLedgerEntryState {
			states[keyXDR] = observedState{entry: *entry, index: index}

			continue
		}

		identity := scope
		if operation != nil {
			identity = fmt.Sprintf("operation:%d", *operation)
		}

		change := domain.StateChange{
			ID: fmt.Sprintf("%s:change:%d", identity, index),
			Kind: strings.ToLower(
				strings.TrimPrefix(raw.Type.String(), "LedgerEntryChangeTypeLedgerEntry"),
			),
			EntryType:    strings.TrimPrefix(key.Type.String(), "LedgerEntryType"),
			Scope:        scope,
			OpIndex:      operation,
			ChangeIndex:  index,
			KeyXDR:       keyXDR,
			BeforeStatus: "unknown",
			AfterStatus:  "present",
		}
		change.Subject, change.Key = stateKey(key)
		change.Key = readableStateValue(change.Key)
		if previous, exists := states[keyXDR]; exists {
			change.BeforeStatus = "present"
			change.Before = stateSnapshot(previous.entry)
			change.BeforeIndex = pointer(previous.index)
		}

		switch raw.Type {
		case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
			change.BeforeStatus, change.Before, change.BeforeIndex = "absent", nil, nil
		case xdr.LedgerEntryChangeTypeLedgerEntryRestored:
			change.BeforeStatus, change.Before, change.BeforeIndex = "archived", nil, nil
		case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
			change.AfterStatus = "absent"
		}

		if entry != nil {
			change.After = stateSnapshot(*entry)
			states[keyXDR] = observedState{entry: *entry, index: index}
		} else {
			delete(states, keyXDR)
		}

		changes = append(changes, change)
	}

	return changes, nil
}
