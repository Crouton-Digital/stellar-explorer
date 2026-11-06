package decoder_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestStateChangesAcrossMetadataVersions(t *testing.T) {
	before := testutil.AccountEntry(testutil.Address(1), 9223372036854775807)
	after := testutil.AccountEntry(testutil.Address(1), 9223372036854775707)
	group := xdr.LedgerEntryChanges{
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryState, before),
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryUpdated, after),
	}
	operations := []xdr.OperationMeta{{Changes: group}}
	metas := []xdr.TransactionMeta{
		{V: 0, Operations: &operations},
		{V: 1, V1: &xdr.TransactionMetaV1{Operations: operations}},
		{V: 2, V2: &xdr.TransactionMetaV2{Operations: operations}},
		{V: 3, V3: &xdr.TransactionMetaV3{Operations: operations}},
		{V: 4, V4: &xdr.TransactionMetaV4{
			Operations: []xdr.OperationMetaV2{{Changes: group}},
		}},
	}
	for _, meta := range metas {
		t.Run(fmt.Sprintf("v%d", meta.V), func(t *testing.T) {
			envelope := testutil.PaymentEnvelope()
			transaction, err := decoder.Transaction(envelope, testutil.Result(envelope, true),
				meta, testutil.Ledger(10), 1)
			if err != nil {
				t.Fatal(err)
			}

			changes := transaction.Interpretation.StateChanges
			if len(changes) != 1 {
				t.Fatal("baseline was counted as a write", changes)
			}

			change := changes[0]
			if change.BeforeStatus != "present" || change.AfterStatus != "present" ||
				change.Kind != "updated" || change.EntryType != "Account" ||
				change.Scope != "operation" || *change.OpIndex != 0 ||
				change.ChangeIndex != 1 || *change.BeforeIndex != 0 ||
				change.Before.Fields["balance_stroops"] != "9223372036854775807" ||
				change.After.Fields["balance_stroops"] != "9223372036854775707" ||
				change.After.Fields["sequence"] != "9223372036854775807" {
				t.Fatalf("incorrect state transition: %+v", change)
			}

			var key xdr.LedgerKey
			if err = xdr.SafeUnmarshalBase64(change.KeyXDR, &key); err != nil ||
				key.MustAccount().AccountId.Address() != testutil.Address(1) {
				t.Fatal("state key evidence lost", err)
			}
		})
	}
}

func TestStateRestorationAndMissingBaselines(t *testing.T) {
	entry := testutil.AccountEntry(testutil.Address(1), 20)
	other := testutil.AccountEntry(testutil.Address(2), 30)
	group := xdr.LedgerEntryChanges{
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryState, other),
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryUpdated, entry),
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryRemoved, entry),
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryCreated, entry),
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryRestored, other),
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryUpdated, other),
	}
	envelope := testutil.PaymentEnvelope()
	transaction, err := decoder.Transaction(envelope, testutil.Result(envelope, true),
		xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{
			Operations: []xdr.OperationMetaV2{{Changes: group}},
		}}, testutil.Ledger(10), 1)
	if err != nil {
		t.Fatal(err)
	}

	changes := transaction.Interpretation.StateChanges
	if len(changes) != 5 || changes[0].BeforeStatus != "unknown" || changes[0].Before != nil ||
		changes[1].AfterStatus != "absent" || changes[1].After != nil ||
		changes[1].Before.Fields["balance_stroops"] != "20" ||
		changes[2].BeforeStatus != "absent" || changes[2].Before != nil ||
		changes[3].BeforeStatus != "archived" || changes[3].Before != nil ||
		changes[4].BeforeStatus != "present" || *changes[4].BeforeIndex != 4 {
		t.Fatal("missing, removed, created or restored state was misrepresented", changes)
	}
}

func TestFailedTransactionKeepsTransactionStateAndOmitsOperationWrites(t *testing.T) {
	for _, successful := range []bool{true, false} {
		transaction := testutil.StateBundle(t, 10, successful).Transactions[0]
		changes := transaction.Interpretation.StateChanges
		expectedCount := 1
		if successful {
			expectedCount = 2
		}

		if len(changes) != expectedCount || changes[0].Scope != "transaction_before" ||
			changes[0].OpIndex != nil {
			t.Fatal("rolled-back writes or transaction-level state was incorrect", changes)
		}

		expected := transaction.Interpretation
		transaction.Interpretation.StateChanges = nil
		if err := decoder.RebuildInterpretation(&transaction); err != nil ||
			!reflect.DeepEqual(expected, transaction.Interpretation) {
			t.Fatal("source replay differs from live state interpretation", err)
		}
	}
}

func TestSorobanStateProjectsStorageCodeAndTTL(t *testing.T) {
	change := testutil.InstanceChange(testutil.Contract(9), 10,
		xdr.LedgerEntryChangeTypeLedgerEntryCreated, testutil.WasmExecutable(3))
	entry := change.MustCreated()
	data := *entry.Data.ContractData
	entry.Data.ContractData = &data
	entry.Data.ContractData.Key = testutil.Symbol("amount")
	entry.Data.ContractData.Val = testutil.I128(9223372036854775807, 18446744073709551615)
	code := xdr.LedgerEntry{
		Data: xdr.LedgerEntryData{
			Type:         xdr.LedgerEntryTypeContractCode,
			ContractCode: &xdr.ContractCodeEntry{Hash: xdr.Hash{3}, Code: []byte{1, 2, 3}},
		},
	}
	ttl := xdr.LedgerEntry{
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeTtl,
			Ttl:  &xdr.TtlEntry{KeyHash: xdr.Hash{4}, LiveUntilLedgerSeq: 4294967295},
		},
	}
	bundle := testutil.ContractBundle(t, 10, change,
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryCreated, entry),
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryCreated, code),
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryCreated, ttl))
	changes := bundle.Transactions[0].Interpretation.StateChanges
	value := changes[1].After.Fields["value"].(map[string]any)
	if len(changes) != 4 || changes[0].Subject != testutil.Contract(9) ||
		changes[0].After.Fields["value"].(map[string]any)["wasm_hash"] == nil ||
		value["value"] != "170141183460469231731687303715884105727" ||
		changes[2].After.Fields["wasm_bytes"] != "3" ||
		changes[3].After.Fields["live_until_ledger"] != "4294967295" {
		t.Fatal("Soroban storage, code or TTL projection lost evidence", changes)
	}
}

func TestStateBaselinesDoNotCrossGroupsOrContractDataKeys(t *testing.T) {
	instance := testutil.InstanceChange(testutil.Contract(9), 10,
		xdr.LedgerEntryChangeTypeLedgerEntryState, testutil.WasmExecutable(1))
	baseline := instance.MustState()
	baseline.Data.ContractData.Key = testutil.Symbol("balance")
	baseline.Data.ContractData.Val = testutil.I128(0, 20)
	differentKey := baseline
	keyData := *baseline.Data.ContractData
	differentKey.Data.ContractData = &keyData
	differentKey.Data.ContractData.Key = testutil.Symbol("allowance")
	differentDurability := baseline
	durabilityData := *baseline.Data.ContractData
	differentDurability.Data.ContractData = &durabilityData
	differentDurability.Data.ContractData.Durability = xdr.ContractDataDurabilityTemporary
	account := testutil.AccountEntry(testutil.Address(1), 20)
	group := xdr.LedgerEntryChanges{
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryState, baseline),
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryUpdated, differentKey),
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryUpdated, differentDurability),
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryUpdated, account),
	}
	envelope := testutil.ContractEnvelope()
	transaction, err := decoder.Transaction(envelope, testutil.Result(envelope, true),
		xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{
			TxChangesBefore: xdr.LedgerEntryChanges{
				testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryState, account),
			},
			Operations: []xdr.OperationMetaV2{{Changes: group}},
			TxChangesAfter: xdr.LedgerEntryChanges{
				testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryRemoved, account),
			},
		}}, testutil.Ledger(10), 1)
	if err != nil {
		t.Fatal(err)
	}

	changes := transaction.Interpretation.StateChanges
	if len(changes) != 4 {
		t.Fatal("baseline-only group was counted as a write")
	}

	for _, change := range changes {
		if change.BeforeStatus != "unknown" || change.Before != nil || change.BeforeIndex != nil {
			t.Fatal("baseline crossed a key, durability or metadata group", change)
		}
	}
}

func TestUnsupportedStateEntryKeepsIdentityAndSourceProvenance(t *testing.T) {
	value := xdr.Uint32(65536)
	entry := xdr.LedgerEntry{
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeConfigSetting,
			ConfigSetting: &xdr.ConfigSettingEntry{
				ConfigSettingId:      xdr.ConfigSettingIdConfigSettingContractMaxSizeBytes,
				ContractMaxSizeBytes: &value,
			},
		},
	}
	bundle := testutil.ContractBundle(t, 10,
		testutil.EntryChange(t, xdr.LedgerEntryChangeTypeLedgerEntryCreated, entry))
	transaction := bundle.Transactions[0]
	change := transaction.Interpretation.StateChanges[0]
	if change.EntryType != "ConfigSetting" || change.After.Projection != "xdr_only" ||
		len(change.After.Fields) != 0 || change.KeyXDR == "" || change.ChangeIndex != 0 ||
		transaction.MetaXDR == "" {
		t.Fatal("unsupported entry disappeared or gained invented fields", change)
	}
}

func TestStateStringsPreserveNULAndInvalidUTF8Bytes(t *testing.T) {
	change := testutil.InstanceChange(testutil.Contract(9), 10,
		xdr.LedgerEntryChangeTypeLedgerEntryCreated, testutil.WasmExecutable(1))
	change.Created.Data.ContractData.Key = testutil.String("key\x00")
	change.Created.Data.ContractData.Val = testutil.String("a\x00\xff")
	bundle := testutil.ContractBundle(t, 10, change)
	state := bundle.Transactions[0].Interpretation.StateChanges[0]
	value := state.After.Fields["value"].(map[string]any)["value"].(map[string]any)
	key := state.Key.(map[string]any)["key"].(map[string]any)["value"].(map[string]any)
	if value["encoding"] != "hex" || value["value_hex"] != "6100ff" ||
		key["value_hex"] != "6b657900" {
		t.Fatal("binary strings were dropped or changed", state)
	}

	raw, err := json.Marshal(state)
	if err != nil || strings.Contains(string(raw), `\u0000`) ||
		strings.Contains(string(raw), `\ufffd`) {
		t.Fatal("state projection cannot be stored exactly as JSONB", string(raw), err)
	}
}
