# MVP verification

Go unit tests and XDR fixture tests run without network access. PostgreSQL integration tests are skipped only when `TEST_DATABASE_URL` is missing; this variable is required in CI.

```bash
go test ./...
TEST_DATABASE_URL='postgres://crouton:crouton-local@localhost:5432/crouton_test?sslmode=disable' go test -race ./... -count=1
go vet ./...
cd web
npm ci
npm test
npm run typecheck
npm run build
```

Each integration test creates a separate randomly named schema, applies migrations, and removes only its own schema. Use a test database and a role with permission to create schemas. Tests cover rollback after a child-record write fails, duplicate-free replay, checkpoint persistence, chain continuity, snapshot pagination, payment recipient history, all API endpoints, search, and errors.

## Browser E2E tests

Import the fixtures into a separate test database before running Playwright. The test runner starts and stops the backend and production Nuxt server on ports `18080` and `13000`.

```bash
DATABASE_URL="$TEST_DATABASE_URL" go run ./cmd/replay tests/fixtures/testnet.json
cd web
npm run build
npx playwright install --with-deps chromium
TEST_DATABASE_URL="$TEST_DATABASE_URL" npm run test:e2e
```

If Chrome is already installed, set `PLAYWRIGHT_CHROME_PATH=/usr/bin/google-chrome`. Tests cover desktop and mobile layouts, Crouton logo loading and header navigation, searching for a real transaction, XDR, account/contract pages, invalid/not-found requests, and loading/empty/error states. Error-state tests intercept browser requests; successful flows use the real API and PostgreSQL.

Custom controls are checked with pointer, touch, keyboard navigation, typeahead, Escape cancellation, Tab selection, outside clicks, and viewport resizing. Keyboard behavior follows the [select-only combobox pattern](https://www.w3.org/WAI/ARIA/apg/patterns/combobox/examples/combobox-select-only/). Responsive tests visit the thirteen overview/list/entity views at 320, 600, and 768 pixels; focused NFT tests additionally cover collection/item views through 320–1440 pixels, verify navigation touch targets and overflow, and check that mobile table cards retain every labeled field. Screenshots cover narrow layouts and open custom dropdowns.

`web/tests/e2e/regressions.spec.ts` adds ten scenarios, each run in desktop and mobile Chromium: exact clipboard contents and accessible feedback, unavailable/denied clipboard recovery, transaction navigation and browser history, filtered-list retry, stale-response rejection after filter changes, invalid-cursor recovery, IME search composition, escaped contract strings, and the keyboard skip link. Clipboard permissions, delayed/error responses, and the contract-string payload are controlled browser mocks; navigation, search, and recovered list requests use the fixture-backed API. The mobile project emulates an iPhone viewport and touch input; it does not test Safari or a physical device.

After preparing the fixture database and production frontend build as above, run only these scenarios from `web/`:

```bash
TEST_DATABASE_URL="$TEST_DATABASE_URL" npm run test:e2e -- tests/e2e/regressions.spec.ts
```

| Acceptance criterion | Verification                                                                                                    |
| -------------------- | --------------------------------------------------------------------------------------------------------------- |
| AC-01                | Compose configuration, production Go/Nuxt builds, startup guide; full Compose smoke check performed separately  |
| AC-02 / AC-03        | `internal/decoder/decoder_test.go`, real XDR fixtures, API integration, transaction/contract E2E tests          |
| AC-04                | `internal/httpapi/api_test.go`, browser search, not-found                                                       |
| AC-05                | `internal/storage/store_test.go`: failure after ledger/transaction inserts, rollback, replay                    |
| AC-06                | `internal/ingestion/indexer_test.go`: a new indexer resumes from an existing checkpoint; PostgreSQL persistence |
| AC-07                | OpenAPI, API integration, and a consistent error envelope                                                       |
| AC-08                | Playwright desktop/mobile tests and controlled loading/empty/error states                                       |
| AC-09                | README, architecture, and startup documentation                                                                 |

## Fixture provenance

Token tests cover scalar/map event bodies, exact i128 amounts, SAC contract-ID verification, legacy issuer mint/burn semantics, muxed destination data, malformed event fallback, and failed transaction exclusion. PostgreSQL tests verify atomic rollback, replay, movement and asset cursor pagination, metadata promotion, and migration from retained events without changing raw XDR or checkpoint. Browser tests cover asset navigation, token recipient history, and unknown-decimal base-unit displays.

Contract tests cover v3/v4 instance metadata, code upgrades, SAC identity, non-instance storage exclusion, fixed creation-preimage vectors, factory creation with unknown deployer, failed calls, and restoration. PostgreSQL tests check replay, rollback, application order, metadata-only history and search, and atomic backfill preserving source data and ingestion state. Browser tests verify real metadata, unknown fields, creation links, and responsive layouts.

Filter tests check AND semantics, contract/function correlation within one call, direction, event symbol/type, inclusive bounds, empty results, strict validation, scope preservation, and cursor binding/snapshots. Browser tests cover applying/resetting filters, pagination, reload/back navigation, invalid ranges, and readable empty-text insets on desktop and mobile.

CAP-67 tests cover committed transaction fees/refunds on successful and failed transactions, fee-bump payer attribution, source/stage provenance, exact signed i128 arithmetic, malformed/missing/duplicate fees and reconciliation with every transaction in the real fixture. PostgreSQL tests verify fee-write rollback, atomic migration from retained metadata, conflicting-source rejection, unchanged raw XDR/checkpoint/dataset/movement identities, replay and event-source cursor binding. Browser tests cover exact fee totals, unknown/inconsistent breakdowns and custom source filtering on desktop and mobile. The current support boundaries are documented in the [standards matrix](../docs/standards.md).

Interpretation tests verify exact payment/token amounts, failed multi-operation atomicity, fee-bump inner failures, precondition failures, classic action deduplication, no inferred swaps, handled diagnostics, bounded diagnostic output and deterministic reconstruction from source XDR. PostgreSQL tests check migration replay, version recording, source/data/state preservation and rollback on corrupt XDR. API tests cover readable failures and lightweight lists. Browser tests cover real successful/failed transactions, diagnostic evidence and exact unknown-decimal amounts at narrow widths. See [interpretation scope](../docs/interpretation.md).

State tests cover metadata v0–v4, complete-key baseline matching, exact account/i128/TTL values, creation/removal/restoration, unknown baselines, contract instance/code/storage projections, lossless NUL/invalid-UTF-8 hex fallback and failed-operation exclusion. Swap tests distinguish actual amounts from request limits, reconcile classic order-book/pool claims across multiple hops, reject inconsistent results and suppress rolled-back conversions. Migration 8 tests check live/rebuilt equality, unchanged source and ingestion state, replay and atomic rollback after a later corrupt row. API and browser tests verify detailed evidence, real metadata, unknown/archived states, pagination, limits versus execution, and mobile wrapping.

Soroswap adapter tests check the registered Testnet router and typed exact-input/output requests against committed router events, return amounts, the successful result preimage hash, deterministic pool addresses and ordered transfers. The commitment includes original operation contract/system events and excludes diagnostics and transaction-level fees. Tampered or rewritten evidence that cannot reproduce it retains ordinary contract-call and movement evidence, as do unsupported/ambiguous calls. Migration 9 tests cover enrichment of retained XDR, source and ingestion-state preservation, replay and atomic rollback. Browser tests use intercepted interpretation payloads over real transaction pages to verify actual amounts versus limits, a contract trader distinct from the operation source, shared hop/event provenance, unknown-decimal i128 amounts, exact u64 deadlines and explicit missing WASM identity. Desktop and mobile cases check wrapping at 320–1440 pixels; backend tests establish adapter acceptance, while these controlled payloads check presentation.

Aquarius adapter tests cover direct/chained exact input, chained strict receive, bounded single-group exact-input splits, mode-specific returns and successful result commitments. Matching router/pool-trade events and ordered token transfers establish funding, net spending, output payout and any positive unused-input return after payout. Zero return requires a null event index; extra zero-valued transfers remain unsupported. Split tests reconcile positive weights totaling 10000, exactly divisible allocations, branch paths, endpoint totals and references into flattened pool evidence. Unsupported group/rounding forms, nested calls and inconsistent settlement remain generic contract calls. Migrations 10 and 11 verify retained-history enrichment and preservation of source data and ingestion state.

Browser tests use intercepted payloads over real transaction pages to check actual amounts versus requested input/minimum output or maximum input/exact output, a contract trader distinct from the operation source, net versus funded/returned input, nullable return-event provenance, weighted branch grouping and pool references. They retain legacy adapter-version-1 payloads without optional settlement/routes, exact unknown-decimal amounts, missing/observed executable evidence and unreconciled-call fallback. Direct, chained, strict-receive and split cases check 320–1440-pixel layouts. These presentation cases do not independently establish deployed code identity, pool-registry membership or an undocumented allocation-rounding policy.

[`fixtures/testnet.json`](fixtures/testnet.json) contains public Testnet RPC data captured on November 5, 2026. It includes metadata for ledger 5569284: 12 transactions covering a payment, a successful Soroban invocation, committed events, and a failed transaction. The original XDR payloads are unmodified. These are historical test fixtures and do not represent the current network tip.

Update the fixtures explicitly with `go run ./cmd/capture -output tests/fixtures/testnet.json -span 200`. This command only reads public RPC data, verifies the Testnet passphrase, and searches for the required example categories. If a category is missing, fixture coverage is incomplete and the fixture test fails. Validate a new fixture before replacing the reference data.

[`fixtures/soroswap-testnet.json`](fixtures/soroswap-testnet.json) contains six additional public Testnet transaction source records exported from retained RPC history on November 5, 2026. It preserves the original envelope, result and metadata XDR for ledgers 5570457, 5570493, 5574008, 5574017, 5574027 and 5574039. These are transaction-level decoder fixtures, not a sequence of complete ledger replays. Tests verify network-specific identity, exact router events/returns/transfers, deterministic factory pool addresses and execution-result commitments; examples include a contract-wallet trader and custom tokens with unknown decimals. Do not import this file with the complete-ledger replay command.

[`fixtures/aquarius-testnet.json`](fixtures/aquarius-testnet.json) contains nine public Testnet transaction source records exported from retained RPC history on November 5, 2026. It includes four exact-input exchanges (two direct and two chained), four strict-receive exchanges (two with one hop and two with three hops), and one single-group split exchange. Original envelope, result and metadata XDR, transaction identities, transaction indexes and ledger evidence remain unmodified. Decoder tests reconcile all nine against the registered-router ABI, committed result hashes, router/pool events and token settlement, including unused input returned after output payout and separate weighted branch paths. These records are transaction-level decoder fixtures, not complete ledgers or a continuous ledger sequence. Do not import this file with the complete-ledger replay command; it does not establish complete network coverage or independently verified deployed code identity.

NFT tests check exact unsigned zero/max IDs and original widths, scalar/token-ID-map bodies, burn-extension labels, separate fungible payments, successful invocation commitments, failed/diagnostic/fee exclusion and constructor-emitted synthetic cases. PostgreSQL/API tests verify atomic persistence/enrichment, replay, source conflicts, unchanged prior evidence/checkpoints, collection/token/participant filters, cursor binding and snapshot counts. Browser presentation cases cover collection/item navigation, full u256 ID filtering and pagination, unknown ownership, distinct NFT/FT transaction panels, normalized contract-party links, empty/error/loading states and narrow layouts. No live metadata or artwork is requested.

Activity tests check the fixed 120-ledger window, grouping of ten actual rows, inclusive bounds, zero buckets, category totals, filter/cursor compatibility and source snapshots. Frontend unit tests verify query scoping, zero-height geometry and success-rate semantics. Browser tests compare the real activity API, inspect zero buckets by keyboard/touch controls, retain cursor/filter bindings without list limits, and distinguish absent indexed history from zero matching records.

[`fixtures/nfts-testnet.json`](fixtures/nfts-testnet.json) contains six unmodified public Testnet transaction source records exported from retained RPC history on November 5, 2026: two scalar NFT transfers, one scalar mint and three token-ID-map mints. Examples include ID zero, a contract recipient and an NFT marketplace transfer with a separate fungible payment. Recognition remains an event-shape observation, not deployed-code or collection-compliance verification. Constructor mint coverage is synthetic, not claimed as a captured real example. These transaction-level decoder fixtures are not complete ledger replays; do not import them with the complete-ledger replay command.
