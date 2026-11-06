package decoder_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func decodeTokenEvents(
	t *testing.T,
	version int32,
	success bool,
	events ...xdr.ContractEvent,
) domain.Transaction {
	t.Helper()
	meta := xdr.TransactionMeta{V: version}
	if version == 3 {
		meta.V3 = &xdr.TransactionMetaV3{
			SorobanMeta: &xdr.SorobanTransactionMeta{
				Events:      events,
				ReturnValue: xdr.ScVal{Type: xdr.ScValTypeScvVoid},
			},
		}
	} else {
		meta.V4 = &xdr.TransactionMetaV4{
			Operations: []xdr.OperationMetaV2{
				{Events: events},
			},
		}
	}

	env := testutil.PaymentEnvelope()
	tx, e := decoder.Transaction(env, testutil.Result(env, success), meta, testutil.Ledger(10), 1)
	if e != nil {
		t.Fatal(e)
	}

	return tx
}

func TestTokenEventsPreserveInt128AndIdentifySAC(t *testing.T) {
	native := xdr.MustNewNativeAsset()
	credit := xdr.MustNewCreditAsset("USD", testutil.Address(9))
	for _, asset := range []xdr.Asset{native, credit} {
		event := testutil.TokenEvent(testutil.SACContract(asset), "transfer",
			[]string{testutil.Address(3), testutil.Address(4)},
			testutil.I128(9223372036854775807, 18446744073709551615),
			testutil.String(asset.StringCanonical()))

		for _, version := range []int32{3, 4} {
			tx := decodeTokenEvents(t, version, true, event)
			if len(tx.TokenTransfers) != 1 {
				t.Fatalf("missing movement: %+v", tx)
			}

			movement := tx.TokenTransfers[0]
			if movement.Amount != "170141183460469231731687303715884105727" ||
				movement.Asset.Kind != "sac" || movement.Asset.ClassicAsset == nil ||
				*movement.Asset.ClassicAsset != asset.StringCanonical() || movement.Asset.Decimals == nil ||
				*movement.Asset.Decimals != 7 || movement.EventIndex != 0 || movement.TxIndex != 1 {
				t.Fatalf("incorrect movement: %+v", movement)
			}

			if !slices.Contains(tx.Participants, testutil.Address(4)) {
				t.Fatal("token recipient missing from account history")
			}

			before, _ := json.Marshal(tx.TokenTransfers)
			if e := decoder.RebuildTokenActivity(&tx); e != nil {
				t.Fatal(e)
			}

			after, _ := json.Marshal(tx.TokenTransfers)
			if string(before) != string(after) {
				t.Fatal("migration and live decoder disagree")
			}
		}
	}
}

func TestTokenMapAmountsAndMuxedIDs(t *testing.T) {
	id := xdr.Uint64(18446744073709551615)
	values := []xdr.ScVal{
		{Type: xdr.ScValTypeScvU64, U64: &id},
		testutil.String("invoice-42"),
		{Type: xdr.ScValTypeScvVoid},
	}
	for _, muxed := range values {
		entries := xdr.ScMap{
			{Key: testutil.Symbol("amount"), Val: testutil.I128(0, 0)},
			{Key: testutil.Symbol("future_field"), Val: testutil.String("ignored")},
			{Key: testutil.Symbol("to_muxed_id"), Val: muxed},
		}
		value := testutil.Map(entries)
		event := testutil.TokenEvent(testutil.Contract(9), "transfer",
			[]string{testutil.Address(3), testutil.Contract(4)}, value)
		tx := decodeTokenEvents(t, 4, true, event)
		if len(tx.TokenTransfers) != 1 || tx.TokenTransfers[0].Amount != "0" {
			t.Fatal("map amount or zero movement missing")
		}

		asset := tx.TokenTransfers[0].Asset
		if asset.Kind != "contract" || asset.Verification != "event_shape" ||
			asset.Decimals != nil ||
			asset.Code != nil {
			t.Fatal("custom token metadata was invented")
		}

		if muxed.Type != xdr.ScValTypeScvVoid &&
			!reflect.DeepEqual(tx.TokenTransfers[0].ToMuxed, decoder.Value(muxed)) {
			t.Fatal("muxed information changed or lost precision")
		}
	}
}

func TestTokenSACLegacyMintBurnAndClawback(t *testing.T) {
	asset := xdr.MustNewCreditAsset("USD", testutil.Address(9))
	contract := testutil.SACContract(asset)
	for _, tc := range []struct {
		version   int32
		name      string
		addresses []string
		kind      string
	}{
		{3, "transfer", []string{testutil.Address(9), testutil.Address(3)}, "mint"},
		{3, "transfer", []string{testutil.Address(3), testutil.Address(9)}, "burn"},
		{3, "mint", []string{testutil.Address(9), testutil.Address(3)}, "mint"},
		{3, "clawback", []string{testutil.Address(9), testutil.Address(3)}, "clawback"},
		{4, "mint", []string{testutil.Address(3)}, "mint"},
		{4, "clawback", []string{testutil.Address(3)}, "clawback"},
		{4, "burn", []string{testutil.Address(3)}, "burn"},
	} {
		event := testutil.TokenEvent(
			contract,
			tc.name,
			tc.addresses,
			testutil.I128(0, 5),
			testutil.String(asset.StringCanonical()),
		)
		tx := decodeTokenEvents(t, tc.version, true, event)
		if len(tx.TokenTransfers) != 1 || tx.TokenTransfers[0].Kind != tc.kind {
			t.Fatalf("incorrect SAC interpretation: %+v", tc)
		}
	}
}

func TestTokenLookalikesNeverVerifyAnAsset(t *testing.T) {
	asset := xdr.MustNewCreditAsset("USD", testutil.Address(9))
	mainnetID, _ := asset.ContractID(network.PublicNetworkPassphrase)
	wrongNetwork := strkey.MustEncode(strkey.VersionByteContract, mainnetID[:])
	for _, contract := range []string{testutil.Contract(9), wrongNetwork} {
		event := testutil.TokenEvent(
			contract,
			"transfer",
			[]string{testutil.Address(3), testutil.Address(4)},
			testutil.I128(0, 42),
			testutil.String(asset.StringCanonical()),
		)
		tx := decodeTokenEvents(t, 4, true, event)
		if len(tx.TokenTransfers) != 1 || tx.TokenTransfers[0].Asset.Kind != "contract" ||
			tx.TokenTransfers[0].Asset.Decimals != nil {
			t.Fatal("forged asset identity trusted")
		}
	}
}

func TestTokenMalformedEventsRemainRaw(t *testing.T) {
	base := func() xdr.ContractEvent {
		return testutil.TokenEvent(testutil.Contract(9), "transfer",
			[]string{testutil.Address(3), testutil.Address(4)}, testutil.I128(0, 42))
	}
	for _, tc := range []struct {
		name   string
		change func(*xdr.ContractEvent)
	}{
		{"negative amount", func(event *xdr.ContractEvent) {
			event.Body.V0.Data = testutil.I128(-1, 0)
		}},
		{"wrong amount type", func(event *xdr.ContractEvent) {
			event.Body.V0.Data = testutil.String("42")
		}},
		{"missing destination", func(event *xdr.ContractEvent) {
			event.Body.V0.Topics = event.Body.V0.Topics[:2]
		}},
		{"wrong sender type", func(event *xdr.ContractEvent) {
			event.Body.V0.Topics[1] = testutil.String("GFAKE")
		}},
		{"diagnostic type", func(event *xdr.ContractEvent) {
			event.Type = xdr.ContractEventTypeDiagnostic
		}},
		{"unknown symbol", func(event *xdr.ContractEvent) {
			event.Body.V0.Topics[0] = testutil.Symbol("swap")
		}},
		{"custom mint", func(event *xdr.ContractEvent) {
			event.Body.V0.Topics[0] = testutil.Symbol("mint")
		}},
		{"no contract", func(event *xdr.ContractEvent) {
			event.ContractId = nil
		}},
		{"missing map amount", func(event *xdr.ContractEvent) {
			entries := xdr.ScMap{}
			event.Body.V0.Data = testutil.Map(entries)
		}},
		{"duplicate map amount", func(event *xdr.ContractEvent) {
			entries := xdr.ScMap{
				{Key: testutil.Symbol("amount"), Val: testutil.I128(0, 1)},
				{Key: testutil.Symbol("amount"), Val: testutil.I128(0, 2)},
			}
			event.Body.V0.Data = testutil.Map(entries)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event := base()
			tc.change(&event)
			tx := decodeTokenEvents(t, 4, true, event)
			if len(tx.TokenTransfers) != 0 || len(tx.Events) != 1 || tx.Events[0].RawXDR == "" {
				t.Fatal("malformed event interpreted or raw event lost")
			}
		})
	}

	tx := decodeTokenEvents(t, 4, false, base())
	if len(tx.TokenTransfers) != 0 {
		t.Fatal("failed transaction created a movement")
	}
}

func TestRealFixtureContainsVerifiedNativeTokenMovements(t *testing.T) {
	b := testutil.RealBundle(t)
	found := false
	for _, tx := range b.Transactions {
		for _, movement := range tx.TokenTransfers {
			if movement.Asset.Kind == "sac" && movement.Asset.Code != nil &&
				*movement.Asset.Code == "XLM" {
				found = true
			}
		}
	}

	if !found {
		t.Fatal("real fixture did not expose native SAC token movements")
	}
}
