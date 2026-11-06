package storage_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/storage"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func readContract(t *testing.T, s *storage.Store, address string) domain.Contract {
	t.Helper()
	raw, err := s.Get(context.Background(), "contracts", address)
	if err != nil {
		t.Fatal(err)
	}

	var contract domain.Contract
	if err = json.Unmarshal(raw, &contract); err != nil {
		t.Fatal(err)
	}

	return contract
}

func TestContractPersistenceUpdatesReplayAndRollback(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	address := testutil.Contract(9)
	created := testutil.InstanceChange(address, 10,
		xdr.LedgerEntryChangeTypeLedgerEntryCreated, testutil.WasmExecutable(1))
	bundle := testutil.ContractBundle(t, 10, created)
	broken := testutil.ContractBundle(t, 10, created)
	broken.Transactions[0].Contracts[0].LastObservedLedger = 11
	if err := s.Save(ctx, broken); err == nil {
		t.Fatal("foreign contract observation accepted")
	}

	if checkpoint, err := s.Checkpoint(ctx); err != nil || checkpoint != 0 {
		t.Fatal("contract failure advanced checkpoint", err)
	}

	for replay := 0; replay < 2; replay++ {
		if err := s.Save(ctx, bundle); err != nil {
			t.Fatal(err)
		}
	}

	var count int
	if err := s.Pool.QueryRow(ctx, `
		SELECT
		    count(*)
		FROM
		    transaction_contracts
	`).Scan(&count); err != nil || count != 1 {
		t.Fatal("replay duplicated contract associations", count, err)
	}

	updated := testutil.InstanceChange(address, 11,
		xdr.LedgerEntryChangeTypeLedgerEntryUpdated, testutil.WasmExecutable(2))
	if err := s.Save(ctx, testutil.ContractBundle(t, 11, updated)); err != nil {
		t.Fatal(err)
	}

	// A later call without instance changes must preserve the last confirmed executable.
	if err := s.Save(ctx, testutil.ContractBundle(t, 12)); err != nil {
		t.Fatal(err)
	}

	contract := readContract(t, s, address)
	if contract.WasmHash == nil || *contract.WasmHash != fmt.Sprintf("02%062x", 0) ||
		contract.ExecutableLedger == nil || *contract.ExecutableLedger != 11 ||
		contract.FirstObservedLedger != 10 || contract.LastObservedLedger != 12 ||
		contract.CreationLedger == nil || *contract.CreationLedger != 10 ||
		contract.CreationTransaction == nil || *contract.CreationTransaction != bundle.Transactions[0].Hash {
		raw, _ := json.Marshal(contract)
		t.Fatalf(
			"lost creation or latest executable: %s, creation transaction %s",
			raw,
			bundle.Transactions[0].Hash,
		)
	}

	page, err := s.List(ctx, "contract_transactions", address, 20, nil)
	if err != nil || len(page.Items) != 3 {
		t.Fatal("contract history omitted metadata-only transactions", err)
	}
}

func TestContractExecutableUsesApplicationOrderWithinLedger(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	address := testutil.Contract(9)
	first := testutil.InstanceChange(address, 10,
		xdr.LedgerEntryChangeTypeLedgerEntryUpdated, testutil.WasmExecutable(1))
	second := testutil.InstanceChange(address, 10,
		xdr.LedgerEntryChangeTypeLedgerEntryUpdated, testutil.WasmExecutable(2))
	bundle := testutil.ContractBundle(t, 10, first)
	later := testutil.ContractBundle(t, 10, second).Transactions[0]
	later.Hash = fmt.Sprintf("%064x", 999)
	later.TxIndex = 2
	later.Contracts[0].ExecutableTxIndex = &later.TxIndex
	bundle.Transactions = []domain.Transaction{later, bundle.Transactions[0]}
	bundle.Ledger.TransactionCount = 2
	if err := s.Save(ctx, bundle); err != nil {
		t.Fatal(err)
	}

	contract := readContract(t, s, address)
	if contract.WasmHash == nil || *contract.WasmHash != fmt.Sprintf("02%062x", 0) {
		t.Fatal("executable followed slice order instead of application order")
	}
}

func TestContractMigrationPreservesSourceAndIngestionState(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	for sequence := uint32(10); sequence <= 11; sequence++ {
		kind := xdr.LedgerEntryChangeTypeLedgerEntryCreated
		if sequence == 11 {
			kind = xdr.LedgerEntryChangeTypeLedgerEntryUpdated
		}

		change := testutil.InstanceChange(testutil.Contract(9), sequence, kind,
			testutil.WasmExecutable(byte(sequence)))
		if err := s.Save(ctx, testutil.ContractBundle(t, sequence, change)); err != nil {
			t.Fatal(err)
		}
	}

	beforeContract := readContract(t, s, testutil.Contract(9))
	beforeState, err := s.State(ctx)
	if err != nil {
		t.Fatal(err)
	}

	before := contractSourceSnapshot(t, s)
	removeContractMigration(t, s)
	for repetition := 0; repetition < 2; repetition++ {
		if err = s.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}

	afterState, err := s.State(ctx)
	if err != nil || !reflect.DeepEqual(beforeState, afterState) {
		t.Fatal("contract migration changed checkpoint or counters", err)
	}

	if before != contractSourceSnapshot(t, s) ||
		!reflect.DeepEqual(beforeContract, readContract(t, s, testutil.Contract(9))) {
		t.Fatal("backfill changed source data, dataset identity, or metadata")
	}

	page, err := s.List(ctx, "contract_transactions", testutil.Contract(9), 20, nil)
	if err != nil || len(page.Items) != 2 {
		t.Fatal("backfill lost contract transaction associations", err)
	}
}

func TestContractMigrationRollsBackUnreadableSource(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	if err := s.Save(ctx, testutil.ContractBundle(t, 10)); err != nil {
		t.Fatal(err)
	}

	removeContractMigration(t, s)
	if _, err := s.Pool.Exec(ctx, `
		UPDATE transactions
		SET
		    payload = jsonb_set(payload, '{envelope_xdr}', '"invalid"'::jsonb)
	`); err != nil {
		t.Fatal(err)
	}

	if err := s.Migrate(ctx); err == nil {
		t.Fatal("unreadable source silently accepted")
	}

	var applied bool
	var table *string
	if err := s.Pool.QueryRow(ctx, `
		SELECT
		    EXISTS (SELECT 1 FROM schema_migrations WHERE version = 4),
		    to_regclass(current_schema() || '.contracts')::text
	`).Scan(&applied, &table); err != nil || applied || table != nil {
		t.Fatal("failed migration committed its schema", err)
	}

	if checkpoint, err := s.Checkpoint(ctx); err != nil || checkpoint != 10 {
		t.Fatal("failed migration changed checkpoint", err)
	}
}

func removeContractMigration(t *testing.T, s *storage.Store) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), `
		DROP TABLE transaction_contracts;
		DROP TABLE contracts;
		DELETE FROM schema_migrations WHERE version = 4;
		UPDATE transactions SET payload = payload - 'contracts'
	`); err != nil {
		t.Fatal(err)
	}
}

func contractSourceSnapshot(t *testing.T, s *storage.Store) string {
	t.Helper()
	var snapshot string
	if err := s.Pool.QueryRow(context.Background(), `
		SELECT
		    md5(string_agg(
		        (payload - 'contracts')::text || envelope_xdr || result_xdr || meta_xdr,
		        '' ORDER BY hash
		    )) || (SELECT dataset_id FROM indexer_metadata WHERE network = $1)
		FROM
		    transactions
		WHERE
		    network = $1
	`, domain.Network).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}

	return snapshot
}
