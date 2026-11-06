package decoder

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/crouton/stellar-explorer/internal/domain"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const maxDiagnosticErrors = 64

const maxDiagnosticDepth = 32

var camelWordBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// Diagnostics provide evidence; handled contract errors do not determine transaction success.
func interpretDiagnostics(interpretation *domain.Interpretation, meta xdr.TransactionMeta) error {
	if meta.V < 1 {
		return nil
	}

	events, err := meta.GetDiagnosticEvents()
	if err != nil {
		return err
	}

	seen := map[string]bool{}
	for index, diagnostic := range events {
		event := diagnostic.Event
		if event.Body.V != 0 || event.Body.V0 == nil {
			continue
		}

		contract := ""
		if event.ContractId != nil {
			contract = strkey.MustEncode(strkey.VersionByteContract, event.ContractId[:])
		}

		collect := func(value xdr.ScError) {
			decoded := diagnosticError(value)
			decoded.EventIndex = index
			decoded.ContractID = contract
			key := fmt.Sprintf("%d:%s:%s", index, decoded.Type, decoded.Code)
			if seen[key] {
				return
			}

			if len(interpretation.DiagnosticErrors) >= maxDiagnosticErrors {
				interpretation.DiagnosticsTruncated = true

				return
			}

			seen[key] = true
			interpretation.DiagnosticErrors = append(interpretation.DiagnosticErrors, decoded)
		}

		for _, topic := range event.Body.V0.Topics {
			walkDiagnosticErrors(topic, 0, collect, &interpretation.DiagnosticsTruncated)
		}

		walkDiagnosticErrors(event.Body.V0.Data, 0, collect, &interpretation.DiagnosticsTruncated)
	}

	return nil
}

func walkDiagnosticErrors(
	value xdr.ScVal,
	depth int,
	collect func(xdr.ScError),
	truncated *bool,
) {
	if depth > maxDiagnosticDepth {
		*truncated = true

		return
	}

	switch value.Type {
	case xdr.ScValTypeScvError:
		collect(value.MustError())
	case xdr.ScValTypeScvVec:
		if items := value.MustVec(); items != nil {
			for _, item := range *items {
				walkDiagnosticErrors(item, depth+1, collect, truncated)
			}
		}
	case xdr.ScValTypeScvMap:
		if entries := value.MustMap(); entries != nil {
			for _, entry := range *entries {
				walkDiagnosticErrors(entry.Key, depth+1, collect, truncated)
				walkDiagnosticErrors(entry.Val, depth+1, collect, truncated)
			}
		}
	}
}

func diagnosticError(value xdr.ScError) domain.DiagnosticError {
	decoded := domain.DiagnosticError{
		Type: value.Type.String(),
	}
	if value.Type == xdr.ScErrorTypeSceContract {
		decoded.Code = strconv.FormatUint(uint64(value.MustContractCode()), 10)
		decoded.Message = "Contract-defined error " + decoded.Code + ". Its meaning depends on the contract."

		return decoded
	}

	decoded.Code = value.MustCode().String()
	category := strings.TrimPrefix(decoded.Type, "ScErrorTypeSce")
	code := strings.TrimPrefix(decoded.Code, "ScErrorCodeScec")
	label := strings.ToLower(camelWordBoundary.ReplaceAllString(code, "$1 $2"))
	decoded.Message = category + " diagnostic: " + label + "."

	return decoded
}
