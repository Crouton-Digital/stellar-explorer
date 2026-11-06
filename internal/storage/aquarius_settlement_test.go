package storage_test

import (
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

func TestAquariusSettlementMigrationPreservesHistoryAndReplays(t *testing.T) {
	bundles := []domain.Bundle{
		testutil.AquariusStrictReceiveBundle(t, 10),
		testutil.AquariusSplitBundle(t, 11),
		testutil.AquariusChainedBundle(t, 12),
		testutil.AquariusBundle(t, 13),
		testutil.SoroswapBundle(t, 14),
		testutil.FeeBundle(t, 15, false),
		testutil.StateBundle(t, 16, true),
	}
	checkAquariusMigrationPreservesHistory(t, 11, 4, bundles)
}

func TestAquariusSettlementMigrationRollsBackEveryBatchAndQueuedWrite(t *testing.T) {
	checkAquariusMigrationRollsBack(t, 11, 4, testutil.AquariusStrictReceiveBundle)
}
