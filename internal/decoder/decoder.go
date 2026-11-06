package decoder

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strconv"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/crouton/stellar-explorer/internal/stellar"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Ledger keeps application order from metadata, not the envelope order in txSet.
func Ledger(remote stellar.RemoteLedger) (bundle domain.Bundle, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("invalid or unsupported ledger XDR: %v", r)
		}
	}()

	var meta xdr.LedgerCloseMeta
	if err = xdr.SafeUnmarshalBase64(remote.MetadataXDR, &meta); err != nil {
		return bundle, fmt.Errorf("ledger metadata: %w", err)
	}

	if meta.V < 0 || meta.V > 2 {
		return bundle, fmt.Errorf("unsupported LedgerCloseMeta version %d", meta.V)
	}

	h := meta.LedgerHeaderHistoryEntry()
	if remote.Sequence != meta.LedgerSequence() || remote.Hash != hex.EncodeToString(h.Hash[:]) {
		return bundle, fmt.Errorf("RPC ledger header mismatch")
	}

	previous := meta.PreviousLedgerHash()
	bundle.Ledger = domain.Ledger{
		Network:          domain.Network,
		Sequence:         meta.LedgerSequence(),
		Hash:             remote.Hash,
		PreviousHash:     hex.EncodeToString(previous[:]),
		ClosedAt:         meta.ClosedAt(),
		ProtocolVersion:  meta.ProtocolVersion(),
		TransactionCount: meta.CountTransactions(),
		RawXDR:           remote.MetadataXDR,
	}
	bundle.Transactions = []domain.Transaction{}
	envelopes := map[xdr.Hash]xdr.TransactionEnvelope{}

	for _, env := range meta.TransactionEnvelopes() {
		hash, e := network.HashTransactionInEnvelope(env, domain.Passphrase)
		if e != nil {
			return bundle, e
		}

		if _, ok := envelopes[hash]; ok {
			return bundle, fmt.Errorf("duplicate envelope")
		}

		envelopes[hash] = env
	}

	if len(envelopes) != meta.CountTransactions() {
		return bundle, fmt.Errorf("incomplete ledger transaction set")
	}

	for i := 0; i < meta.CountTransactions(); i++ {
		pair := meta.TransactionResultPair(i)
		env, ok := envelopes[pair.TransactionHash]
		if !ok {
			return bundle, fmt.Errorf("missing transaction envelope")
		}

		t, e := Transaction(env, pair, meta.TxApplyProcessing(i), bundle.Ledger, uint32(i+1))
		if e != nil {
			return bundle, fmt.Errorf(
				"transaction %s: %w",
				hex.EncodeToString(pair.TransactionHash[:]),
				e,
			)
		}

		bundle.Transactions = append(bundle.Transactions, t)
	}

	return bundle, nil
}

func Transaction(
	env xdr.TransactionEnvelope,
	result xdr.TransactionResultPair,
	meta xdr.TransactionMeta,
	l domain.Ledger,
	index uint32,
) (t domain.Transaction, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("invalid transaction XDR: %v", r)
		}
	}()

	t = domain.Transaction{
		Network:        domain.Network,
		Hash:           hex.EncodeToString(result.TransactionHash[:]),
		LedgerSequence: l.Sequence,
		TxIndex:        index,
		SourceAccount:  env.SourceAccount().ToAccountId().Address(),
		FeeAccount:     env.FeeAccount().ToAccountId().Address(),
		Successful:     result.Successful(),
		FeeCharged:     strconv.FormatInt(int64(result.Result.FeeCharged), 10),
		ResultCode:     result.Result.Result.Code.String(),
		CreatedAt:      l.ClosedAt,
		Operations:     []domain.Operation{},
		Calls:          []domain.Call{},
		Events:         []domain.Event{},
		TokenTransfers: []domain.TokenTransfer{},
		NFTTransfers:   []domain.NFTTransfer{},
		Participants:   []string{},
	}
	if t.EnvelopeXDR, err = xdr.MarshalBase64(env); err != nil {
		return t, err
	}

	if t.ResultXDR, err = xdr.MarshalBase64(result.Result); err != nil {
		return t, err
	}

	if t.MetaXDR, err = xdr.MarshalBase64(meta); err != nil {
		return t, err
	}

	participants := map[string]bool{
		t.SourceAccount: true,
		t.FeeAccount:    true,
	}

	for i, op := range env.Operations() {
		source := t.SourceAccount
		if op.SourceAccount != nil {
			source = op.SourceAccount.ToAccountId().Address()
		}

		participants[source] = true
		raw, e := xdr.MarshalBase64(op)
		if e != nil {
			return t, e
		}

		o := domain.Operation{
			Index:         i,
			Type:          op.Body.Type.String(),
			SourceAccount: source,
			DecodeStatus:  "unknown",
			RawXDR:        raw,
			Details: map[string]any{
				"raw_xdr": raw,
			},
		}

		switch op.Body.Type {
		case xdr.OperationTypePayment:
			p := op.Body.MustPaymentOp()
			dest := p.Destination.ToAccountId().Address()
			participants[dest] = true
			o.DecodeStatus = "decoded"
			o.Details = map[string]any{
				"destination":       dest,
				"destination_muxed": p.Destination.Address(),
				"asset":             p.Asset.StringCanonical(),
				"amount_stroops":    strconv.FormatInt(int64(p.Amount), 10),
			}

		case xdr.OperationTypeCreateAccount:
			p := op.Body.MustCreateAccountOp()
			dest := p.Destination.Address()
			participants[dest] = true
			o.DecodeStatus = "decoded"
			o.Details = map[string]any{
				"destination":              dest,
				"starting_balance_stroops": strconv.FormatInt(int64(p.StartingBalance), 10),
			}

		case xdr.OperationTypeAccountMerge:
			p := op.Body.MustDestination()
			dest := p.ToAccountId().Address()
			participants[dest] = true
			o.DecodeStatus = "decoded"
			o.Details = map[string]any{
				"destination": dest,
			}

		case xdr.OperationTypeInvokeHostFunction:
			fn := op.Body.MustInvokeHostFunctionOp().HostFunction
			if fn.Type == xdr.HostFunctionTypeHostFunctionTypeInvokeContract {
				v := fn.MustInvokeContract()
				address, e := v.ContractAddress.String()
				if e != nil {
					return t, e
				}

				args := []any{}

				for _, a := range v.Args {
					args = append(args, Value(a))
				}

				t.Calls = append(t.Calls, domain.Call{
					Index:        i,
					ContractID:   address,
					FunctionName: string(v.FunctionName),
					Arguments:    args,
					DecodeStatus: "decoded",
				})
				o.DecodeStatus = "decoded"
				o.Details = map[string]any{
					"contract_id":   address,
					"function_name": string(v.FunctionName),
					"arguments":     args,
				}
			}
		}

		t.Operations = append(t.Operations, o)
	}

	if err = decodeEvents(&t, meta, l.ProtocolVersion, participants); err != nil {
		return t, err
	}

	t.NFTTransfers = nftActivity(&t, result.Result, meta, participants)

	diagnostics := []string{}
	if meta.V >= 1 && meta.V <= 4 {
		events, e := meta.GetDiagnosticEvents()
		if e != nil {
			return t, e
		}

		for _, ev := range events {
			raw, e := xdr.MarshalBase64(ev)
			if e != nil {
				return t, e
			}

			diagnostics = append(diagnostics, raw)
		}
	}

	t.Diagnostics = map[string]any{
		"events_xdr":   diagnostics,
		"availability": "not_provided_or_empty",
	}
	if len(diagnostics) > 0 {
		t.Diagnostics = map[string]any{
			"events_xdr":   diagnostics,
			"availability": "provided",
		}
	}

	for address := range participants {
		t.Participants = append(t.Participants, address)
	}

	sort.Strings(t.Participants)
	if err = contractMetadata(&t, env, meta); err != nil {
		return t, err
	}

	if err = interpretTransaction(&t, env, result.Result, meta); err != nil {
		return t, err
	}

	return t, nil
}

// Value preserves the SCVal type and original XDR. Large numbers never cross JSON as floats.
func Value(v xdr.ScVal) any {
	raw, _ := xdr.MarshalBase64(v)
	r := map[string]any{
		"type":          v.Type.String(),
		"xdr":           raw,
		"decode_status": "decoded",
	}

	switch v.Type {
	case xdr.ScValTypeScvVoid:
		r["value"] = nil

	case xdr.ScValTypeScvBool:
		r["value"] = v.MustB()

	case xdr.ScValTypeScvError:
		value := v.MustError()
		var code string
		if value.Type == xdr.ScErrorTypeSceContract {
			code = strconv.FormatUint(uint64(value.MustContractCode()), 10)
		} else {
			code = value.MustCode().String()
		}

		r["value"] = map[string]any{
			"error_type": value.Type.String(),
			"code":       code,
		}

	case xdr.ScValTypeScvSymbol:
		r["value"] = string(v.MustSym())

	case xdr.ScValTypeScvString:
		r["value"] = string(v.MustStr())

	case xdr.ScValTypeScvBytes:
		r["value"] = hex.EncodeToString(v.MustBytes())

	case xdr.ScValTypeScvU32:
		r["value"] = strconv.FormatUint(uint64(v.MustU32()), 10)

	case xdr.ScValTypeScvI32:
		r["value"] = strconv.FormatInt(int64(v.MustI32()), 10)

	case xdr.ScValTypeScvU64:
		r["value"] = strconv.FormatUint(uint64(v.MustU64()), 10)

	case xdr.ScValTypeScvI64:
		r["value"] = strconv.FormatInt(int64(v.MustI64()), 10)

	case xdr.ScValTypeScvI128:
		p := v.MustI128()
		n := new(big.Int).Lsh(big.NewInt(int64(p.Hi)), 64)
		n.Add(n, new(big.Int).SetUint64(uint64(p.Lo)))
		r["value"] = n.String()

	case xdr.ScValTypeScvU128:
		p := v.MustU128()
		n := new(big.Int).Lsh(new(big.Int).SetUint64(uint64(p.Hi)), 64)
		n.Add(n, new(big.Int).SetUint64(uint64(p.Lo)))
		r["value"] = n.String()

	case xdr.ScValTypeScvAddress:
		a, e := v.MustAddress().String()
		if e != nil {
			r["decode_status"] = "unknown"
		} else {
			r["value"] = a
		}

	case xdr.ScValTypeScvVec:
		items := []any{}
		if p := v.MustVec(); p != nil {
			for _, item := range *p {
				items = append(items, Value(item))
			}
		}

		r["value"] = items

	case xdr.ScValTypeScvMap:
		items := []any{}
		if p := v.MustMap(); p != nil {
			for _, item := range *p {
				items = append(items, map[string]any{
					"key":   Value(item.Key),
					"value": Value(item.Val),
				})
			}
		}

		r["value"] = items

	default:
		r["decode_status"] = "unknown"
	}

	return r
}
