package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/httpapi"
	"github.com/crouton/stellar-explorer/internal/storage"
	"github.com/crouton/stellar-explorer/internal/testutil"
)

const maximumNFTID = "115792089237316195423570985008687907853269984665640564039457584007913129639935"

type firstNFTCommitStore struct {
	*storage.Store
	bundle domain.Bundle
}

func (s *firstNFTCommitStore) NFTSnapshot(ctx context.Context) (storage.Cursor, error) {
	snapshot, err := s.Store.NFTSnapshot(ctx)
	if err != nil {
		return snapshot, err
	}

	if err = s.Store.Save(ctx, s.bundle); err != nil {
		return snapshot, err
	}

	return snapshot, nil
}

func TestNFTDetailEmptySnapshotDoesNotIncludeTheFirstConcurrentCommit(t *testing.T) {
	for _, suffix := range []string{"", "/tokens/7"} {
		t.Run(suffix, func(t *testing.T) {
			store := &firstNFTCommitStore{
				Store:  testutil.Database(t),
				bundle: testutil.NFTBundle(t, 10),
			}
			handler := (&httpapi.API{Store: store, MaxLimit: 100}).Handler()
			path := "/api/v1/nfts/collections/" + testutil.Contract(30) + suffix
			response := request(handler, "GET", path)
			if response.Code != 404 {
				t.Fatalf(
					"empty snapshot leaked a newer NFT commit: %d %s",
					response.Code,
					response.Body,
				)
			}

			checkpoint, err := store.Checkpoint(context.Background())
			if err != nil || checkpoint != 10 {
				t.Fatalf("concurrent commit did not run: %d %v", checkpoint, err)
			}
		})
	}
}

func TestInvalidNFTQueriesNeverReachStorage(t *testing.T) {
	handler := (&httpapi.API{MaxLimit: 100}).Handler()
	collection := "/api/v1/nfts/collections/" + testutil.Contract(30)
	for _, path := range []string{
		"/api/v1/nfts/collections/invalid",
		"/api/v1/nfts/collections/" + testutil.Address(1),
		collection + "/tokens/-1",
		collection + "/tokens/01",
		collection + "/tokens/1.5",
		collection + "/tokens/1e3",
		collection + "/tokens/" + strings.Repeat("9", 78),
		collection + "?limit=0",
		collection + "?limit=101",
		collection + "?limit=1&limit=2",
		collection + "?cursor=one&cursor=two",
		collection + "/tokens/0?limit=0",
		collection + "/tokens/0?cursor=one&cursor=two",
		"/api/v1/nfts/transfers?collection=invalid",
		"/api/v1/nfts/transfers?token_id=-1",
		"/api/v1/nfts/transfers?token_id=00",
		"/api/v1/nfts/transfers?token_id=" + strings.Repeat("9", 78),
		"/api/v1/nfts/transfers?kind=approve",
		"/api/v1/nfts/transfers?from=invalid",
		"/api/v1/nfts/transfers?min_ledger=11&max_ledger=10",
		"/api/v1/nfts/transfers?cursor=invalid",
		"/api/v1/nfts/transfers?amount=1",
		"/api/v1/nfts/collections?token_id=0",
		"/api/v1/accounts/invalid/nft-transfers",
		"/api/v1/contracts/" + testutil.Address(1) + "/nft-transfers",
	} {
		t.Run(path, func(t *testing.T) {
			response := request(handler, "GET", path)
			if response.Code != 400 {
				t.Fatalf("invalid NFT query reached storage: %d %s", response.Code, response.Body)
			}
		})
	}
}

func TestNFTEndpointsPreserveExactIDsAndBoundedHistory(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	collection, otherCollection := testutil.Contract(30), testutil.Contract(31)
	contractRecipient := testutil.Contract(40)
	bundle := testutil.NFTBundle(
		t,
		10,
		testutil.NFTEvent(collection, "mint", []string{testutil.Address(2)}, testutil.U32(0), true),
		testutil.NFTEvent(collection, "transfer",
			[]string{testutil.Address(2), contractRecipient},
			testutil.U256(math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64), false),
		testutil.NFTEvent(
			otherCollection,
			"mint",
			[]string{testutil.Address(3)},
			testutil.U32(7),
			false,
		),
	)
	if err := s.Save(ctx, bundle); err != nil {
		t.Fatal(err)
	}

	handler := (&httpapi.API{Store: s, MaxLimit: 100}).Handler()
	for _, scenario := range []struct {
		path  string
		count int
	}{
		{"/api/v1/nfts/collections", 2},
		{"/api/v1/nfts/transfers", 3},
		{"/api/v1/nfts/transfers?collection=" + collection, 2},
		{"/api/v1/nfts/transfers?token_id=0", 1},
		{"/api/v1/nfts/transfers?token_id=" + maximumNFTID, 1},
		{"/api/v1/nfts/transfers?kind=mint", 2},
		{"/api/v1/nfts/transfers?from=" + testutil.Address(2), 1},
		{"/api/v1/nfts/transfers?to=" + contractRecipient, 1},
		{"/api/v1/nfts/transfers?min_ledger=11", 0},
		{"/api/v1/nfts/collections/" + collection + "/transfers", 2},
		{"/api/v1/accounts/" + testutil.Address(2) + "/nft-transfers", 2},
		{"/api/v1/contracts/" + contractRecipient + "/nft-transfers", 1},
		{"/api/v1/contracts/" + testutil.Contract(99) + "/nft-transfers", 0},
		{"/api/v1/transfers", 0},
	} {
		t.Run(scenario.path, func(t *testing.T) {
			response := request(handler, "GET", scenario.path)
			var page domain.Page
			if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil ||
				response.Code != 200 || len(page.Items) != scenario.count {
				t.Fatalf("incorrect NFT history: %d %s %v", response.Code, response.Body, err)
			}
		})
	}

	response := request(
		handler,
		"GET",
		"/api/v1/nfts/collections/"+collection+"/tokens/"+maximumNFTID,
	)
	var detail struct {
		Item           domain.NFTItem `json:"item"`
		Transfers      domain.Page    `json:"transfers"`
		SnapshotLedger uint32         `json:"snapshot_ledger"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil || response.Code != 200 ||
		detail.Item.TokenID != maximumNFTID || detail.Item.TransferCount != 1 ||
		detail.SnapshotLedger != 10 || len(detail.Transfers.Items) != 1 {
		t.Fatalf("large NFT ID lost precision: %d %s %v", response.Code, response.Body, err)
	}

	var movement domain.NFTTransfer
	if err := json.Unmarshal(detail.Transfers.Items[0], &movement); err != nil ||
		movement.TokenID != maximumNFTID || movement.TokenIDType != "u256" {
		t.Fatalf("movement ID lost precision: %+v %v", movement, err)
	}

	response = request(handler, "GET", "/api/v1/transactions/"+bundle.Transactions[0].Hash)
	var transaction domain.Transaction
	if err := json.Unmarshal(response.Body.Bytes(), &transaction); err != nil ||
		response.Code != 200 ||
		len(transaction.NFTTransfers) != 3 ||
		len(transaction.TokenTransfers) != 0 {
		t.Fatalf("NFT evidence missing from transaction: %d %v", response.Code, err)
	}

	response = request(handler, "GET", "/api/v1/nfts/collections/"+collection+"?limit=1")
	var first struct {
		Collection domain.NFTCollectionView `json:"collection"`
		Transfers  domain.Page              `json:"transfers"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &first); err != nil || response.Code != 200 ||
		first.Collection.ObservedTokenCount != 2 || first.Collection.TransferCount != 2 ||
		first.Transfers.NextCursor == nil {
		t.Fatalf(
			"collection summary or cursor is incorrect: %d %s %v",
			response.Code,
			response.Body,
			err,
		)
	}

	newBundle := testutil.NFTBundle(t, 11,
		testutil.NFTEvent(collection, "transfer",
			[]string{testutil.Address(2), testutil.Address(4)}, testutil.U32(0), false),
	)
	if err := s.Save(ctx, newBundle); err != nil {
		t.Fatal(err)
	}

	response = request(handler, "GET", "/api/v1/nfts/collections/"+collection+
		"?limit=1&cursor="+*first.Transfers.NextCursor)
	var older struct {
		Collection     domain.NFTCollectionView `json:"collection"`
		Transfers      domain.Page              `json:"transfers"`
		SnapshotLedger uint32                   `json:"snapshot_ledger"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &older); err != nil || response.Code != 200 ||
		older.Collection.TransferCount != 2 || older.SnapshotLedger != 10 || len(older.Transfers.Items) != 1 {
		t.Fatalf(
			"NFT summary leaked beyond history snapshot: %d %s %v",
			response.Code,
			response.Body,
			err,
		)
	}

	for _, path := range []string{
		"/api/v1/nfts/collections/" + otherCollection + "?cursor=" + *first.Transfers.NextCursor,
		"/api/v1/nfts/collections/" + collection + "?kind=mint&cursor=" + *first.Transfers.NextCursor,
	} {
		if response = request(handler, "GET", path); response.Code != 400 {
			t.Fatalf("NFT cursor crossed query boundaries: %d %s", response.Code, response.Body)
		}
	}

	for _, path := range []string{
		fmt.Sprintf("/api/v1/nfts/collections/%s/tokens/8", collection),
		"/api/v1/nfts/collections/" + testutil.Contract(99),
	} {
		if response = request(handler, "GET", path); response.Code != 404 {
			t.Fatalf("unobserved NFT detail should return 404: %d %s", response.Code, response.Body)
		}
	}
}
