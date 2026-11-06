package storage_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/storage"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

const maximumNFTID = "115792089237316195423570985008687907853269984665640564039457584007913129639935"

func nftHistoryBundle(t *testing.T, sequence uint32) domain.Bundle {
	t.Helper()
	collection := testutil.Contract(30)

	return testutil.NFTBundle(
		t,
		sequence,
		testutil.NFTEvent(collection, "mint", []string{testutil.Address(2)}, testutil.U32(7), true),
		testutil.NFTEvent(
			collection,
			"transfer",
			[]string{testutil.Address(2), testutil.Address(3)},
			testutil.U64(7),
			false,
		),
		testutil.NFTEvent(
			collection,
			"burn",
			[]string{testutil.Address(3)},
			testutil.U32(7),
			false,
		),
		testutil.NFTEvent(testutil.Contract(31), "transfer",
			[]string{testutil.Address(4), testutil.Contract(8)},
			testutil.U256(^uint64(0), ^uint64(0), ^uint64(0), ^uint64(0)), true),
	)
}

func TestNFTPersistenceSourceProofReplayAndExactIDs(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	for _, corrupt := range []struct {
		name   string
		change func(*domain.Bundle)
	}{
		{
			name: "token ID",
			change: func(bundle *domain.Bundle) {
				bundle.Transactions[0].NFTTransfers[0].TokenID = "8"
			},
		},
		{
			name: "foreign transaction",
			change: func(bundle *domain.Bundle) {
				bundle.Transactions[0].NFTTransfers[0].TransactionHash = "foreign"
			},
		},
		{
			name: "forged parent source",
			change: func(bundle *domain.Bundle) {
				bundle.Transactions[0].Events[0].RawXDR = "different source"
			},
		},
	} {
		t.Run(corrupt.name, func(t *testing.T) {
			bundle := nftHistoryBundle(t, 10)
			corrupt.change(&bundle)
			if err := s.Save(ctx, bundle); err == nil {
				t.Fatal("NFT activity without matching committed source was accepted")
			}

			var ledgers, collections, transfers int
			if err := s.Pool.QueryRow(ctx, `
				SELECT
				    (SELECT count(*) FROM ledgers),
				    (SELECT count(*) FROM nft_collections),
				    (SELECT count(*) FROM nft_transfers)
			`).Scan(&ledgers, &collections, &transfers); err != nil ||
				ledgers != 0 || collections != 0 || transfers != 0 {
				t.Fatal("failed NFT write left partial ledger or activity data", err)
			}
		})
	}

	bundle := nftHistoryBundle(t, 10)
	for replay := 0; replay < 2; replay++ {
		if err := s.Save(ctx, bundle); err != nil {
			t.Fatal(err)
		}
	}

	var nft, fungible int
	var identifier string
	if err := s.Pool.QueryRow(ctx, `
		SELECT
		    (SELECT count(*) FROM nft_transfers),
		    (SELECT count(*) FROM token_transfers),
		    (SELECT max(token_id)::text FROM nft_transfers)
	`).Scan(&nft, &fungible, &identifier); err != nil || nft != 4 || fungible != 0 ||
		identifier != maximumNFTID {
		t.Fatal("NFT IDs lost precision, duplicated or became fungible amounts", err)
	}

	collection, err := s.NFTCollection(ctx, testutil.Contract(30), 10)
	if err != nil || collection.ObservedTokenCount != 1 || collection.TransferCount != 3 {
		t.Fatal("numeric token identity was split by its event encoding type", collection, err)
	}

	item, err := s.NFTItem(ctx, testutil.Contract(31), maximumNFTID, 10)
	if err != nil || item.TokenID != maximumNFTID || item.TransferCount != 1 {
		t.Fatal("maximum u256 item history unavailable", item, err)
	}
}

func TestNFTCursorsBindDatasetScopeFiltersAndHistoricalCounts(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	if err := s.Save(ctx, nftHistoryBundle(t, 10)); err != nil {
		t.Fatal(err)
	}

	page, err := s.List(ctx, "nft_transfers", "", 1, nil)
	if err != nil || page.NextCursor == nil || len(page.Items) != 1 {
		t.Fatal("NFT history cursor unavailable", err)
	}

	cursor, err := storage.ParseCursor(*page.NextCursor, "nft_transfers", "")
	if err != nil {
		t.Fatal(err)
	}

	for _, mutate := range []func(*storage.Cursor){
		func(cursor *storage.Cursor) {
			cursor.Dataset = "another dataset"
		},
		func(cursor *storage.Cursor) {
			cursor.Snapshot = 11
		},
	} {
		invalid := *cursor
		mutate(&invalid)
		if _, err := s.List(ctx, "nft_transfers", "", 1, &invalid); !errors.Is(
			err,
			storage.ErrInvalidCursor,
		) {
			t.Fatal("NFT cursor accepted a foreign dataset or future snapshot", err)
		}
	}

	if _, err := storage.ParseCursor(*page.NextCursor, "collection_nft_transfers", testutil.Contract(30)); err == nil {
		t.Fatal("global NFT cursor accepted for a scoped collection")
	}

	filter := domain.Filters{NFTKind: "mint"}
	if _, err := storage.ParseCursor(*page.NextCursor, "nft_transfers", "", filter); err == nil {
		t.Fatal("NFT cursor accepted different filters")
	}

	next := testutil.NFTBundle(
		t,
		11,
		testutil.NFTEvent(
			testutil.Contract(30),
			"mint",
			[]string{testutil.Address(4)},
			testutil.U32(8),
			false,
		),
		testutil.NFTEvent(
			testutil.Contract(32),
			"mint",
			[]string{testutil.Address(4)},
			testutil.U32(1),
			false,
		),
	)
	if err := s.Save(ctx, next); err != nil {
		t.Fatal(err)
	}

	remainder, err := s.List(ctx, "nft_transfers", "", 10, cursor)
	if err != nil || len(remainder.Items) != 3 || remainder.NextCursor != nil {
		t.Fatal("NFT cursor snapshot changed after ingestion", err)
	}

	for _, raw := range remainder.Items {
		var movement domain.NFTTransfer
		if err := json.Unmarshal(raw, &movement); err != nil || movement.LedgerSequence != 10 {
			t.Fatal("a future NFT movement entered an existing snapshot", err)
		}
	}

	historical, err := s.NFTCollection(ctx, testutil.Contract(30), 10)
	current, currentErr := s.NFTCollection(ctx, testutil.Contract(30), 11)
	if err != nil || currentErr != nil || historical.ObservedTokenCount != 1 ||
		historical.TransferCount != 3 || current.ObservedTokenCount != 2 || current.TransferCount != 4 {
		t.Fatal(
			"NFT collection counts were computed from a page or leaked future activity",
			err,
			currentErr,
		)
	}

	for _, query := range []struct {
		kind  string
		scope string
		count int
	}{
		{"collection_nft_transfers", testutil.Contract(30), 4},
		{"nft_token_transfers", testutil.Contract(30) + ":7", 3},
		{"account_nft_transfers", testutil.Address(3), 2},
		{"contract_nft_transfers", testutil.Contract(8), 1},
	} {
		result, err := s.List(ctx, query.kind, query.scope, 20, nil)
		if err != nil || len(result.Items) != query.count {
			t.Fatal("scoped NFT history incomplete", query, err)
		}
	}

	first, err := s.List(ctx, "nft_collections", "", 1, nil)
	if err != nil || len(first.Items) != 1 || first.NextCursor == nil {
		t.Fatal("collection list cursor missing", err)
	}

	collectionCursor, err := storage.ParseCursor(*first.NextCursor, "nft_collections", "")
	if err != nil {
		t.Fatal(err)
	}

	remaining, err := s.List(ctx, "nft_collections", "", 10, collectionCursor)
	if err != nil || len(remaining.Items) != 2 ||
		reflect.DeepEqual(first.Items[0], remaining.Items[0]) {
		t.Fatal("NFT collection pagination omitted or duplicated an emitter", err)
	}
}
