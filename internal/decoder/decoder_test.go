package decoder_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/crouton/stellar-explorer/internal/decoder"
	"github.com/crouton/stellar-explorer/internal/stellar"
	"github.com/crouton/stellar-explorer/internal/testutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestPaymentExactAmountAndParticipants(t *testing.T) {
	env := testutil.PaymentEnvelope()
	tx, e := decoder.Transaction(env, testutil.Result(env, true), xdr.TransactionMeta{
		V:  2,
		V2: &xdr.TransactionMetaV2{},
	}, testutil.Ledger(10), 1)
	if e != nil {
		t.Fatal(e)
	}

	if !tx.Successful || tx.FeeCharged != "100" || len(tx.Operations) != 1 {
		t.Fatalf("unexpected transaction: %+v", tx)
	}

	details := tx.Operations[0].Details.(map[string]any)
	if details["amount_stroops"] != "9223372036854775807" || details["asset"] != "native" {
		t.Fatalf("lost payment precision: %v", details)
	}

	if len(tx.Participants) != 2 || tx.Participants[1] != testutil.Address(2) {
		t.Fatalf("missing payment destination: %v", tx.Participants)
	}

	raw, _ := json.Marshal(tx)
	if !strings.Contains(string(raw), `"amount_stroops":"9223372036854775807"`) {
		t.Fatal("amount must be a JSON string")
	}
}

func TestFailedTransactionAndUnknownOperation(t *testing.T) {
	env := testutil.PaymentEnvelope()
	env.V1.Tx.Operations = []xdr.Operation{
		{
			Body: xdr.OperationBody{
				Type: xdr.OperationTypeManageData,
				ManageDataOp: &xdr.ManageDataOp{
					DataName: "example",
				},
			},
		},
	}
	tx, e := decoder.Transaction(env, testutil.Result(env, false), xdr.TransactionMeta{
		V:  2,
		V2: &xdr.TransactionMetaV2{},
	}, testutil.Ledger(10), 1)
	if e != nil {
		t.Fatal(e)
	}

	if tx.Successful || len(tx.Events) != 0 || tx.Operations[0].DecodeStatus != "unknown" ||
		tx.Operations[0].RawXDR == "" {
		t.Fatal("failed tx or raw fallback incorrect")
	}
}

func TestSCValSigned128AndUnknownFallback(t *testing.T) {
	v := xdr.ScVal{
		Type: xdr.ScValTypeScvI128,
		I128: &xdr.Int128Parts{
			Hi: -1,
			Lo: 18446744073709551615,
		},
	}
	if decoder.Value(v).(map[string]any)["value"] != "-1" {
		t.Fatal("incorrect signed i128")
	}

	u := xdr.ScVal{
		Type:      xdr.ScValTypeScvTimepoint,
		Timepoint: new(xdr.TimePoint),
	}
	r := decoder.Value(u).(map[string]any)
	if r["decode_status"] != "unknown" || r["xdr"] == "" {
		t.Fatal("missing raw fallback")
	}
}

func TestLedgerMetadataOrderAndValidation(t *testing.T) {
	r := testutil.Remote(10, []xdr.TransactionEnvelope{testutil.PaymentEnvelope()})
	b, e := decoder.Ledger(r)
	if e != nil {
		t.Fatal(e)
	}

	if len(b.Transactions) != 1 || b.Transactions[0].TxIndex != 1 {
		t.Fatal("transaction missing")
	}

	r.Sequence++
	if _, e = decoder.Ledger(r); e == nil {
		t.Fatal("accepted mismatching sequence")
	}

	r.MetadataXDR = "broken"
	if _, e = decoder.Ledger(r); e == nil {
		t.Fatal("accepted malformed XDR")
	}
}

func TestRealTestnetFixtures(t *testing.T) {
	raw, e := os.ReadFile("../../tests/fixtures/testnet.json")
	if e != nil {
		t.Fatal(e)
	}

	var f struct {
		Network string                 `json:"network"`
		Ledgers []stellar.RemoteLedger `json:"ledgers"`
	}
	if e = json.Unmarshal(raw, &f); e != nil {
		t.Fatal(e)
	}

	if f.Network != "testnet" || len(f.Ledgers) == 0 {
		t.Fatal("missing Testnet provenance")
	}

	seen := map[string]bool{}

	for _, r := range f.Ledgers {
		b, e := decoder.Ledger(r)
		if e != nil {
			t.Fatal(e)
		}

		for _, tx := range b.Transactions {
			if !tx.Successful {
				seen["failed"] = true
			}

			if len(tx.Calls) > 0 && tx.Successful {
				seen["soroban"] = true
			}

			if len(tx.Events) > 0 {
				seen["events"] = true
			}

			for _, op := range tx.Operations {
				if op.Type == "OperationTypePayment" {
					seen["payment"] = true
				}
			}

			if tx.Hash == "" || tx.SourceAccount == "" || tx.MetaXDR == "" {
				t.Fatal("incomplete transaction")
			}
		}
	}

	for _, tag := range []string{
		"payment",
		"soroban",
		"events",
		"failed",
	} {
		if !seen[tag] {
			t.Errorf("missing real %s fixture", tag)
		}
	}
}
