# UI guide

This guide explains where to find each explorer screen, what its panels mean and how to use the controls. It describes the current Crouton Stellar Explorer UI, including the compact search palette, activity charts and NFT history.

For local startup, see [run](run.md). The default UI address is **http://localhost:3000**. For implementation details, see [architecture](architecture.md) and the [database schema](database.md).

All screens display **indexed Stellar Testnet history**. The explorer is read-only: there is no wallet connection, transaction submission or token trading action. A missing record does not prove that it is absent from the network.

Quick links: [screen map](#screen-map), [search](#global-search), [transactions](#transactions), [NFTs](#nfts), [filters](#filters-and-pagination) and [charts](#charts-and-statistics).

## Global layout

| Area            | Where it is                                                           | What it contains                                                                       |
| --------------- | --------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| Brand           | Left side of the top header row                                       | Crouton Digital logo and Stellar Explorer name; click to return to Overview            |
| Network context | In the top header row, next to search                                 | Stellar and the Testnet badge; this identifies the network, not synchronization health |
| Global search   | Right side of the desktop header; a separate full-width row on mobile | Opens the centered search palette                                                      |
| Main navigation | Below the top header row                                              | Overview, Ledgers, Transactions, Assets, NFTs, Token movements and Events              |
| Page heading    | Top of the central content area                                       | Screen title, context/breadcrumb and, where applicable, a status or page action        |
| Content panels  | Below the page heading                                                | Filters, metadata, charts, lists and expandable evidence, depending on the screen      |
| Pagination      | Below the relevant list                                               | Older results and a way back to the newest results, when available                     |
| Footer          | After the page content                                                | Crouton home link and the Testnet / indexed-history / exact-amounts reminder           |

The desktop header remains visible while scrolling. On narrower screens it becomes part of the normal page flow. A keyboard-only **Skip to content** link appears when focused and moves directly to the page content. The desktop **Stellar RPC docs ↗** link opens external documentation in a new tab.

## Screen map

Curly-brace values below are route placeholders. Replace them with an indexed identifier; the braces are not part of the address.

| Screen                  | Route                       | How to reach it                                                              |
| ----------------------- | --------------------------- | ---------------------------------------------------------------------------- |
| Network overview        | `/`                         | Overview tab or the Crouton brand                                            |
| Ledgers                 | `/ledgers`                  | Ledgers tab or All ledgers on Overview                                       |
| Ledger detail           | `/ledgers/{sequence}`       | Ledger number in a table, a detail panel or search                           |
| Transactions            | `/transactions`             | Transactions tab or All transactions on Overview                             |
| Transaction detail      | `/tx/{hash}`                | Transaction hash in a list/event or search                                   |
| Account activity        | `/accounts/{address}`       | A supported G-address link or search                                         |
| Contract activity       | `/contracts/{address}`      | A supported C-address link or search                                         |
| Assets                  | `/assets`                   | Assets tab                                                                   |
| Asset activity          | `/assets/{address}`         | Asset name/code or token contract in a movement/list                         |
| NFT collections         | `/nfts`                     | NFTs tab                                                                     |
| NFT collection activity | `/nfts/{address}`           | Collection contract in an NFT list                                           |
| NFT item activity       | `/nfts/{address}/{tokenId}` | Exact token ID in an NFT movement row                                        |
| NFT movements           | `/nfts/transfers`           | NFT movements on the collections page, a filtered link or an NFT participant |
| Token movements         | `/transfers`                | Token movements tab or Filter movements on an asset page                     |
| Events                  | `/events`                   | Events tab or Filter events on a contract page                               |

Account, contract, asset and NFT detail views are reached through their identifiers rather than separate top-level navigation tabs. The **NFTs** tab remains active on collection, item and NFT movement screens.

## Global search

Click the header search button, press `/`, or use **Ctrl+K / Cmd+K** outside an editable field. A compact input row opens in the center of the screen and the background dims and blurs. The input receives focus; background interaction and scrolling are blocked while the palette is open.

The placeholder is **Tx hash / account / contract / ledger…**. Search accepts:

| Input                                                    | What it searches             | Destination        |
| -------------------------------------------------------- | ---------------------------- | ------------------ |
| Ledger number or its positive decimal prefix             | Indexed ledger sequences     | Ledger detail      |
| Transaction hash or at least four hexadecimal characters | Indexed transaction hashes   | Transaction detail |
| G-address or at least four address characters            | Indexed account participants | Account activity   |
| C-address or at least four address characters            | Observed contracts           | Contract activity  |

Matches appear as you type, with an entity type and full identifier. A complete identifier is prioritized. Use Up/Down to select, Enter or **Search ↵** to open, or click a result. The Search button appears after entering a nonempty query; previews do not navigate by themselves.

The palette grows to fit feedback/results and scrolls internally when needed. It shows short-prefix guidance, loading, no indexed matches or a retryable error. **Clear search** empties the query, returns focus to the input and collapses the palette to one row. Escape, the close button or the backdrop dismisses it and returns focus to the header button. Tab and Shift+Tab remain inside the palette.

Search resolves identifiers, not asset names, symbols, collection artwork or NFT item metadata. To locate an NFT ID, use the collection/item links or the **Token ID** filter on NFT movements. See [search behavior](search.md) for matching rules.

## Overview

**Location:** Overview tab, `/`.

From top to bottom, the page contains:

1. **Network overview** heading and **↻ Refresh** button. The overview also refreshes every ten seconds while mounted.
2. Four synchronization cards.
3. An indexer error alert when reported, followed by the partial-history notice and last RPC observation time when known.
4. **Recent transaction activity** chart and its bounded-window statistics.
5. **Latest ledgers**, with an **All ledgers ↗** link.
6. **Recent transactions**, with an **All transactions ↗** link.

| Synchronization card | Meaning                                                                         |
| -------------------- | ------------------------------------------------------------------------------- |
| NETWORK LEDGER       | Most recently observed RPC tip; it is not a continuously live network reading   |
| INDEXED LEDGER       | Last ledger fully committed to the explorer database                            |
| INDEXER LAG          | Difference between the observed tip and indexed checkpoint, when both are known |
| INDEXED FROM         | First ledger in the retained indexed range                                      |

An em dash means that the value is unknown. **Caught up at last observation** describes that observation, while **Catching up**, **Waiting for indexer** and **Indexer needs attention** explain other states. Refreshing the page reads the index; it does not manually import a missing transaction.

## Ledgers

### Ledger list

**Location:** Ledgers tab, `/ledgers`.

The table lists indexed ledgers newest first. Columns show the ledger number, shortened hash, transaction count, protocol version and close time. Click a ledger number to open its details. Pagination appears below the table when there is another page.

### Ledger detail

**Location:** `/ledgers/{sequence}`.

The top detail card contains the full hash with Copy, close time, protocol version, transaction count and previous hash with Copy. The **Transactions** panel below contains transactions from that ledger. Click a transaction hash to inspect it. The LEDGERS breadcrumb returns to the list.

## Transactions

### Transaction list

**Location:** Transactions tab, `/transactions`.

The page contains the filter form, **Transaction activity** statistics/chart, then the transaction table and pagination. Columns show transaction hash, Success/Failed status, ledger, source account, result fee in XLM and close time. Hashes, ledger numbers and account addresses are links.

Use **Participant address**, **Contract**, **Function**, **Status** and ledger bounds to narrow the view. The participant filter follows indexed relations; it does not imply a complete effects history. The function filter matches a top-level invocation. See [filters](#filters-and-pagination) below.

### Transaction detail

**Location:** `/tx/{hash}`.

The heading shows the transaction's **Success** or **Failed** status. The following blocks appear in this order:

| Block                   | What it shows / how to use it                                                                                               |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| What happened           | Plain-language summary and supported action cards; start here to understand the transaction                                 |
| Transaction detail card | Full hash with Copy, ledger, source account, fee account, result fee in XLM/stroops, result code and close time             |
| Fees and refunds        | Initial charge, refund and net fee when fee-event evidence reconciles; otherwise an explicit unknown/unverified explanation |
| State changes           | Committed before/after projections, change type, subject, metadata/operation provenance and expandable ledger keys          |
| Operations              | Operation index/type, decode status, effective source, decoded details and expandable Operation XDR                         |
| Soroban calls           | Top-level function names, contract links, expandable Typed arguments and Return value (when provided)                       |
| Events                  | Committed event cards with scope/provenance, emitter, typed topics/value and Raw event XDR                                  |
| Token movements         | Recognized fungible movement rows from this transaction                                                                     |
| NFT movements           | Separate NFT movement rows with exact IDs and event-format evidence                                                         |
| Raw transaction         | Expand Envelope XDR, Result XDR, Transaction metadata XDR and Diagnostics (provider dependent)                              |

**What happened** action badges distinguish **Applied**, **Failed** and **Not applied**. Requested amounts on unsuccessful actions are attempts, not committed transfers. **Why it failed** explains result-based failures when available. Operation links jump to the corresponding operation card. Expand **Diagnostic evidence** to inspect diagnostic errors; handled diagnostics can also occur in successful transactions.

The result fee and fee-event breakdown are separate displays. Missing charge/refund evidence stays unknown rather than becoming zero. State-change baselines can be **Present**, **Absent**, **Archived** or **Unknown**. These are transaction-local metadata projections, not current balances. State changes use local **Previous / Next** controls with ten writes per page.

#### Recognized swaps

Swap details appear inside the relevant **What happened** action card when supported evidence is available.

| Swap panel           | Fields and expandable evidence                                                                                                                                                          |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Classic path payment | Actual Amount sent / Amount received when reconciled, request limits, Executed trades and raw path-payment evidence                                                                     |
| Soroswap execution   | Actual amounts, requested input/output limits, Trader, Deadline, Swap route, Router and evidence                                                                                        |
| Aquarius execution   | Actual sent/spent and received amounts, request limits, routing mode, Trader, optional Funded input / Returned input, Router settlement evidence, Split routes and pool/router evidence |

**Trader** can differ from **Operation source**. Maximum/minimum limits are distinct from actual execution. Aquarius returned input is a token settlement return, separate from the transaction fee refund. Split-route percentages describe branch weights; intermediate hop amounts are not extra total volume. Supporting movement rows refer to the same transfers, so they should not be added again to a swap total. Unsupported shapes remain generic calls/movements. See [interpretation scope](interpretation.md).

## Account activity

**Location:** `/accounts/{address}` through a G-address link or search.

From top to bottom:

1. Copyable address, retained ledger range and included-participant explanation.
2. **Account on Horizon Testnet ↗**, an external account-data link.
3. **Account transaction activity** line chart, when the current transaction page has rows.
4. **Indexed transactions** and their pagination.
5. **Token movements** involving the account, with independent movement pagination.
6. **NFT movements** involving the account, with independent NFT pagination.

The chart is limited to this account and ends at the newest transaction on the current transaction page. The retained range describes the local index; no live balance or complete account history is displayed.

## Contract activity

**Location:** `/contracts/{address}` through a C-address link or search.

From top to bottom:

1. Copyable contract address and indexed-range notice.
2. **Contract metadata**: executable kind, WASM hash/evidence, first/last observation and creation ledger/transaction/creator when known.
3. **Contract transaction activity** line chart, when the current related-transaction page has rows.
4. **Recent calls**: top-level function cards, transaction links, execution status and expandable Typed arguments.
5. **Contract events**, with its own **Newest / Older events** controls and a **Filter events →** link to Events with the contract prefilled.
6. **Related transactions**, with transaction-list pagination.
7. **Token movements** and **NFT movements**, each with independent history controls.

Executable values distinguish WASM, Stellar Asset Contract and Unknown. A SAC does not have an applicable WASM hash in this view. **First observed** is not a creation date. Metadata reflects retained evidence and does not establish current contract state or a complete internal call tree. **Recent calls** is tied to the currently displayed related transactions.

## Assets and fungible movements

### Assets

**Location:** Assets tab, `/assets`.

The table lists token contracts observed in recognized movement events. It shows Asset, Contract, Identification, Decimals and First observed ledger. Click the asset/code or contract to open **Asset activity**.

**Verified SAC** means the network-specific Stellar Asset Contract identity was checked. **Event format match** means a compatible event shape was recognized; it does not certify arbitrary contract code or metadata. Unknown decimals remain **Unknown**.

### Asset activity

**Location:** `/assets/{address}`.

The top card shows the copyable Token contract, Identification, Asset code, Decimals, Issuer and a link to generic **Contract events** evidence. A partial-range notice follows. The **Token movements** table is below, with pagination and **Filter movements →** to open the global movement screen with this asset prefilled.

Amounts use known decimals. When decimals are unknown, the UI displays exact **base units**. The view does not calculate balances or total supply.

### Token movements

**Location:** Token movements tab, `/transfers`.

The filter form is followed by **Movement activity** statistics/chart, then the movement table. Rows show Action, Asset/Identification, From, To, Amount, Transaction and Ledger. Supported action labels include transfer, mint, burn and clawback. **Mint** / **Removed** placeholders represent missing sender/recipient in the decoded evidence, not literal addresses.

Click an asset to open Asset activity, a G/C participant to open its account/contract page, or a transaction/ledger to inspect its source context. Movement counts are event counts, not summed token quantities.

## NFTs

### NFT collections

**Location:** NFTs tab, `/nfts`.

The header's **NFT movements →** link opens the global NFT history. The collections table shows Collection contract, Identification, Observed token IDs, Recorded movements and First observed ledger. Click a collection contract to open **NFT collection activity**.

**Observed token IDs** counts distinct IDs in retained history at the list snapshot. **Recorded movements** counts recognized records. Neither is total supply, current ownership or a certification of the collection.

### NFT collection activity

**Location:** `/nfts/{address}`.

The detail card contains the copyable collection contract, identification, observed ID/movement counts, **Count snapshot**, first observation and a generic Contract evidence link. The notice explains count scope. The **NFT movements** table is below, with **Filter NFT movements →** and pagination.

Counts cover the whole collection through the displayed snapshot ledger. Movement filters only narrow the history list below. Names, artwork and metadata are not fetched. Click an exact token ID to open its item history.

### NFT item activity

**Location:** `/nfts/{address}/{tokenId}`.

The top card shows the full copyable Token ID, collection link, identification, recorded movement count, count snapshot, first observation and **Current owner: Unknown**. The NFT movement history is below. Use the NFT COLLECTION breadcrumb to return to the collection.

IDs are exact unsigned decimal identifiers, including zero; they are not fungible token amounts. An observed recipient does not establish the current owner or the complete lifecycle of an item.

### NFT movements

**Location:** `/nfts/transfers`, from the NFT collections page or a filtered link.

Below the heading/notice, the NFT filter form precedes the movement table. Rows show Action and source event, exact Token ID with original integer type/event format, Collection, From, To, Transaction and Ledger. Burn-extension records are labeled separately from SEP-50 format matches.

Click an ID for item history, a collection for collection history, or a transaction/ledger for source context. NFT **From / To** links open this NFT movement screen filtered by that participant, including contract recipients with no generic contract metadata page. **Mint**, **Burned** and **Unknown** are placeholders where the corresponding address is absent.

## Events

**Location:** Events tab, `/events`.

The event filter form is above the event cards; pagination is below the panel. Each card shows the event index, operation or transaction scope, event type, optional contract emitter and transaction link. Transaction events also show their application stage. Expand **Topics (typed SCVal)**, **Value (typed SCVal)** or **Raw event XDR** for details.

**Event source** separates operation events from transaction events such as fees/refunds. **Event type** is an independent classification. The Diagnostic type option filters committed indexed event records of that type; provider diagnostic traces remain on transaction pages. This page currently has no activity chart.

## Filters and pagination

Filter forms appear on Transactions, Events, Token movements and NFT movements. Editing fields changes a draft; click **Apply filters** to load the new view. **Reset filters** clears the form. All conditions apply together, and changing filters starts from the newest page. The resulting URL can be bookmarked/shared; browser Back/Forward restores URL-based filter state.

| Screen          | Available fields                                                                            |
| --------------- | ------------------------------------------------------------------------------------------- |
| Transactions    | Participant address, Contract, Function, Status, From ledger, To ledger                     |
| Events          | Contract, Event name, Event type, Event source, From ledger, To ledger                      |
| Token movements | Asset contract, From address, To address, From ledger, To ledger                            |
| NFT movements   | Collection contract, Token ID, NFT action, From address, To address, From ledger, To ledger |

Ledger bounds are inclusive. **From ledger** must not exceed **To ledger**. Functions and event names are exact, case-sensitive symbols; event names refer to the first symbol topic. Token ID filters use the full unsigned decimal ID, not a prefix or scientific notation. For precise endpoint semantics, see [activity filters](filters.md).

Custom dropdowns support pointer/touch selection, Up/Down, Home/End, type-ahead and Enter/Space. Escape cancels the open choice; Tab accepts the highlighted choice and moves to the next control. Choosing a dropdown value does not apply the entire filter form until **Apply filters** is used.

**Older results →** continues a cursor-based list. **↺ Newest** removes its cursor while keeping filters. These are not numbered pages or a total-history count. Additional records may appear when returning to the newest view.

Embedded account/contract event and movement panels have their own local pagination. Changing one does not advance the others, and their local cursors are not represented as the main page's URL cursor. State changes use separate Previous/Next controls within a single transaction.

## Charts and statistics

| Screen            | Visual                             | What it counts                                            |
| ----------------- | ---------------------------------- | --------------------------------------------------------- |
| Overview          | Recent transaction activity bars   | Indexed transactions in the displayed recent window       |
| Transactions      | Transaction activity bars          | Transactions matching the current filters/window          |
| Token movements   | Movement activity bars             | Recognized movements matching the current filters/window  |
| Account activity  | Account transaction activity line  | That account's indexed transactions in the bounded window |
| Contract activity | Contract transaction activity line | Related indexed transactions in the bounded window        |

Every chart shows its actual ledger range and considers at most **120 indexed ledgers**, grouped into at most twelve buckets of up to ten ledgers each. This is a ledger-based window, not a fixed hourly/daily timeline. **matching** counts cover that window, not the full result set or only the visible table rows. Outcome/movement-kind counts and the transaction success percentage refer to the same window.

Hover or tap a bucket, use the arrows below the chart, or focus it and press Left/Right/Home/End. The selected count, ledger range and ending time appear below. Expand **Activity data** to read the same bucket counts as text. Zero-height bars/the line baseline represent zero matches. A chart error has its own **Retry activity** button.

Account/contract line charts end at the newest transaction on the current transaction page. Other list charts use their applicable filters and compatible list snapshot. Charts do not display price, balances, supply, summed token volume or complete-network throughput. Ledger, asset, event and NFT screens currently have no chart. See [activity statistics](activity.md).

## Shared controls and states

| UI element                       | Meaning / behavior                                                                                    |
| -------------------------------- | ----------------------------------------------------------------------------------------------------- |
| Shortened identifier link        | Opens the relevant entity; the full identifier is available in its hover title                        |
| Copy                             | Copies the full displayed value; the button changes to Copied or Copy unavailable                     |
| UTC timestamp                    | Main text uses UTC; hover shows a browser-local time tooltip                                          |
| Expandable evidence row          | Click its label, or use Enter/Space when focused, to open raw/typed data                              |
| XLM / stroops                    | Two representations of a fee amount; raw integer fee values remain visible on transaction details     |
| Base units                       | Exact token quantity without an assumed decimal scale; hovering a token amount exposes raw base units |
| Success / Failed                 | Transaction execution result                                                                          |
| Applied / Failed / Not applied   | Status of an interpreted action; not an independent transfer confirmation                             |
| Unknown / em dash                | Unavailable information, not an implied zero or absent network entity                                 |
| Loading indexed data…            | Request in progress                                                                                   |
| No indexed activity / no matches | Empty retained or filtered result; read the accompanying scope explanation                            |
| Unable to load this view         | API/load failure with a message and Try again button                                                  |

An empty token/NFT panel means there are no recognized movement rows for that result, not that the address has a zero balance or owns no assets. Invalid identifiers, invalid filters and records outside the indexed range are reported explicitly. A diagnostic error inside raw evidence is different from a page-loading error.

## Mobile and keyboard use

On smaller screens, the header/search/navigation reflow, filters stack and responsive tables become labeled cards. Full IDs and decoded data wrap or scroll inside their own containers. Important fields remain labeled rather than disappearing with table headers. Search remains centered and constrains/scrolls results within the visible viewport.

Use Tab/Shift+Tab to move between controls, Enter/Space to activate buttons or evidence disclosures, and the dedicated arrow-key behavior for search, dropdowns and charts. Search traps focus only while its modal is open. On touch screens, use the visible close, clear, chart-arrow and pagination controls; hover tooltips are supplementary.

## Suggested walkthroughs

1. **Inspect a transaction:** Overview → a recent transaction → What happened → Fees and refunds → Events → Raw transaction for supporting XDR.
2. **Investigate failed calls:** Transactions → set Contract and/or Function, choose Failed → Apply filters → open a transaction → Why it failed and Diagnostic evidence.
3. **Follow a token:** Assets → asset → Token movements → Filter movements → narrow From/To and ledger bounds → open the source transaction.
4. **Follow an NFT:** NFTs → collection → token ID → item movement history. Use a participant link to inspect that participant's filtered NFT history.
5. **Inspect fees/refunds:** Events → Event source = Transaction events, Event name = `fee` → Apply filters → follow the transaction link → Fees and refunds.

These walkthroughs use the retained indexed range. For supported evidence and limitations, see [NFT history](nfts.md), [token movements](tokens.md), [contract metadata](contracts.md), [interpretation](interpretation.md) and [standards/fees](standards.md).
