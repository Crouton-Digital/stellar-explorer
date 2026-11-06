package decoder_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestContractMetadataVersionsAndCodeUpdates(t *testing.T) {
	for _, version := range []int32{3, 4} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			changes := xdr.LedgerEntryChanges{
				testutil.InstanceChange(testutil.Contract(9), 5,
					xdr.LedgerEntryChangeTypeLedgerEntryState, testutil.WasmExecutable(1)),
				testutil.InstanceChange(testutil.Contract(9), 10,
					xdr.LedgerEntryChangeTypeLedgerEntryUpdated, testutil.WasmExecutable(2)),
			}
			meta := contractMeta(version, changes)
			env := testutil.ContractEnvelope()
			transaction, err := decoder.Transaction(
				env,
				testutil.Result(env, true),
				meta,
				testutil.Ledger(10),
				2,
			)
			if err != nil {
				t.Fatal(err)
			}

			if len(transaction.Contracts) != 1 {
				t.Fatalf("expected one deduplicated instance: %+v", transaction.Contracts)
			}

			contract := transaction.Contracts[0]
			if contract.Kind != "wasm" || contract.WasmHash == nil ||
				*contract.WasmHash != fmt.Sprintf("02%062x", 0) ||
				contract.ExecutableLedger == nil || *contract.ExecutableLedger != 10 ||
				contract.ExecutableTxIndex == nil || *contract.ExecutableTxIndex != 2 ||
				contract.CreationLedger != nil || contract.Creator != nil {
				t.Fatalf("incorrect executable or invented creation: %+v", contract)
			}

			before := transaction.Contracts
			if err = decoder.RebuildContractMetadata(&transaction); err != nil ||
				!reflect.DeepEqual(transaction.Contracts, before) {
				t.Fatal("saved-XDR decoder differs from live decoder", err)
			}
		})
	}
}

func TestContractRestorationAndFailedCallsDoNotInventCreation(t *testing.T) {
	for _, scenario := range []string{"restored", "restore_footprint", "failed", "factory"} {
		t.Run(scenario, func(t *testing.T) {
			env := testutil.ContractEnvelope()
			kind := xdr.LedgerEntryChangeTypeLedgerEntryCreated
			if scenario == "restored" {
				kind = xdr.LedgerEntryChangeTypeLedgerEntryRestored
			}

			if scenario == "restore_footprint" {
				env.V1.Tx.Operations[0].Body = xdr.OperationBody{
					Type:               xdr.OperationTypeRestoreFootprint,
					RestoreFootprintOp: &xdr.RestoreFootprintOp{},
				}
			}

			change := testutil.InstanceChange(
				testutil.Contract(9),
				10,
				kind,
				testutil.WasmExecutable(1),
			)
			meta := contractMeta(4, xdr.LedgerEntryChanges{change})
			transaction, err := decoder.Transaction(
				env,
				testutil.Result(env, scenario != "failed"),
				meta,
				testutil.Ledger(10),
				1,
			)
			if err != nil || len(transaction.Contracts) != 1 {
				t.Fatal("missing observed contract", err)
			}

			contract := transaction.Contracts[0]
			if contract.Creator != nil {
				t.Fatal("transaction source incorrectly used as creator")
			}

			if scenario == "factory" {
				if contract.CreationLedger == nil || *contract.CreationLedger != 10 ||
					contract.CreationTransaction == nil || *contract.CreationTransaction != transaction.Hash {
					t.Fatal("committed factory creation not recorded")
				}
			} else if contract.CreationLedger != nil {
				t.Fatal("restoration or failed call labeled as creation")
			}

			if scenario == "failed" && (contract.Kind != "unknown" || contract.WasmHash != nil) {
				t.Fatal("failed transaction supplied executable evidence")
			}
		})
	}
}

func TestContractSACAndUnrelatedStorageEntries(t *testing.T) {
	sac := xdr.ContractExecutable{Type: xdr.ContractExecutableTypeContractExecutableStellarAsset}
	valid := testutil.InstanceChange(
		testutil.Contract(9),
		10,
		xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
		sac,
	)
	storage := testutil.InstanceChange(
		testutil.Contract(8),
		10,
		xdr.LedgerEntryChangeTypeLedgerEntryCreated,
		sac,
	)
	storage.Created.Data.ContractData.Key = testutil.Symbol("unrelated")
	bundle := testutil.ContractBundle(t, 10, valid, storage)
	contracts := bundle.Transactions[0].Contracts
	if len(contracts) != 1 || contracts[0].Kind != "sac" || contracts[0].WasmHash != nil ||
		contracts[0].ExecutableSource == nil || *contracts[0].ExecutableSource != "instance_entry" {
		t.Fatalf("incorrect instance selection: %+v", contracts)
	}
}

func TestContractCreationPreimageIdentifiesCreator(t *testing.T) {
	// These IDs are fixed vectors from the CAP-46 binary hash preimage, independent of this decoder.
	for _, scenario := range []struct {
		name    string
		creator string
		id      string
		v2      bool
	}{
		{"account_v1", testutil.Address(7), "CCHRSGODYCFJQZVGMLL3DA2OQ46PWFFJAXAT6FVAI7KBD4SLNPP7YROQ", false},
		{"account_v2", testutil.Address(7), "CCHRSGODYCFJQZVGMLL3DA2OQ46PWFFJAXAT6FVAI7KBD4SLNPP7YROQ", true},
		{"contract_v2", testutil.Contract(7), "CAUI4ACQX3DCLC2K3CBT6FFPWMUF5XO2BHJOSVHD544UPOFMVAH6SU54", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			address := testutil.ContractAddress(testutil.Contract(7))
			if scenario.creator == testutil.Address(7) {
				account := xdr.MustAddress(scenario.creator)
				address = xdr.ScAddress{
					Type:      xdr.ScAddressTypeScAddressTypeAccount,
					AccountId: &account,
				}
			}

			preimage := xdr.ContractIdPreimage{
				Type:        xdr.ContractIdPreimageTypeContractIdPreimageFromAddress,
				FromAddress: &xdr.ContractIdPreimageFromAddress{Address: address},
			}
			fn := xdr.HostFunction{
				Type: xdr.HostFunctionTypeHostFunctionTypeCreateContract,
				CreateContract: &xdr.CreateContractArgs{
					ContractIdPreimage: preimage,
					Executable:         testutil.WasmExecutable(1),
				},
			}
			if scenario.v2 {
				fn = xdr.HostFunction{
					Type: xdr.HostFunctionTypeHostFunctionTypeCreateContractV2,
					CreateContractV2: &xdr.CreateContractArgsV2{
						ContractIdPreimage: preimage,
						Executable:         testutil.WasmExecutable(1),
					},
				}
			}

			env := testutil.ContractEnvelope()
			env.V1.Tx.Operations[0].Body.InvokeHostFunctionOp.HostFunction = fn
			changes := xdr.LedgerEntryChanges{
				testutil.InstanceChange(
					scenario.id,
					10,
					xdr.LedgerEntryChangeTypeLedgerEntryCreated,
					testutil.WasmExecutable(1),
				),
				testutil.InstanceChange(
					testutil.Contract(8),
					10,
					xdr.LedgerEntryChangeTypeLedgerEntryCreated,
					testutil.WasmExecutable(1),
				),
			}
			transaction, err := decoder.Transaction(
				env,
				testutil.Result(env, true),
				contractMeta(4, changes),
				testutil.Ledger(10),
				1,
			)
			if err != nil {
				t.Fatal(err)
			}

			for _, contract := range transaction.Contracts {
				if contract.ContractID == scenario.id {
					if contract.Creator == nil || *contract.Creator != scenario.creator {
						t.Fatal("matching preimage did not identify creator")
					}
				} else if contract.Creator != nil {
					t.Fatal("creation preimage assigned to a different contract")
				}
			}
		})
	}
}

func TestRealContractMetadataAndUnreadableSource(t *testing.T) {
	bundle := testutil.RealBundle(t)
	foundWasm := false
	for _, transaction := range bundle.Transactions {
		for _, contract := range transaction.Contracts {
			if contract.Kind == "wasm" && contract.WasmHash != nil {
				foundWasm = true
			}
		}
	}

	if !foundWasm {
		t.Fatal("real Soroban fixture did not yield instance metadata")
	}

	for _, transaction := range []domain.Transaction{
		{EnvelopeXDR: "invalid"},
		{EnvelopeXDR: bundle.Transactions[0].EnvelopeXDR, MetaXDR: "invalid"},
	} {
		if err := decoder.RebuildContractMetadata(&transaction); err == nil {
			t.Fatal("unreadable raw source accepted")
		}
	}
}

func contractMeta(version int32, changes xdr.LedgerEntryChanges) xdr.TransactionMeta {
	if version == 3 {
		return xdr.TransactionMeta{
			V: 3,
			V3: &xdr.TransactionMetaV3{
				Operations: []xdr.OperationMeta{{Changes: changes}},
			},
		}
	}

	return xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			Operations: []xdr.OperationMetaV2{{Changes: changes}},
		},
	}
}
