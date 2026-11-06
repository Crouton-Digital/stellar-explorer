export interface Page<T> {
  items: T[]
  next_cursor: string | null
}
export interface Ledger {
  network: string
  sequence: number
  hash: string
  previous_hash: string
  closed_at: string
  protocol_version: number
  transaction_count: number
}
export interface Call {
  op_index: number
  contract_id: string
  function_name: string
  arguments: unknown[]
  decode_status: string
}
export interface Event {
  transaction_hash: string
  event_index: number
  op_index: number | null
  event_scope: 'operation' | 'transaction'
  stage: string | null
  ledger_sequence: number
  contract_id: string
  event_type: string
  topics: unknown[]
  value: unknown
  raw_xdr: string
}
export interface Operation {
  op_index: number
  type: string
  source_account: string
  details: unknown
  decode_status: string
  raw_xdr: string
}
export interface Transaction {
  network: string
  hash: string
  ledger_sequence: number
  tx_index: number
  source_account: string
  fee_account: string
  successful: boolean
  fee_charged: string
  fees: FeeSummary
  result_code: string
  interpretation?: Interpretation
  created_at: string
  envelope_xdr: string
  result_xdr: string
  meta_xdr: string
  operations: Operation[]
  calls: Call[]
  events: Event[]
  token_transfers: TokenTransfer[]
  nft_transfers?: NFTTransfer[]
  contracts: Contract[]
  return_value: unknown
  diagnostics: unknown
}

export interface Interpretation {
  version: number
  summary: string
  actions: Action[]
  failure: {
    code: string
    inner_code: string | null
    message: string
    operations: { op_index: number; code: string; message: string }[]
  } | null
  diagnostic_errors: {
    event_index: number
    contract_id?: string
    type: string
    code: string
    message: string
  }[]
  diagnostics_truncated: boolean
  state_changes?: StateChange[]
}

export interface StateSnapshot {
  last_modified_ledger: number
  projection: 'supported_fields' | 'xdr_only'
  fields: Record<string, unknown>
}

export interface StateChange {
  id: string
  kind: 'created' | 'updated' | 'removed' | 'restored'
  entry_type: string
  scope: 'transaction_before' | 'operation' | 'transaction_after'
  op_index: number | null
  change_index: number
  before_index: number | null
  subject: string
  key: unknown
  key_xdr: string
  before_status: 'present' | 'absent' | 'archived' | 'unknown'
  after_status: 'present' | 'absent'
  before: StateSnapshot | null
  after: StateSnapshot | null
}

export interface ExchangeLeg {
  asset: Asset
  amount: string
}

export interface ContractSwap {
  protocol: 'soroswap'
  adapter_version: number
  verification: 'registered_router_and_transfers'
  router_contract: string
  router_wasm_hash: string | null
  mode: 'exact_input' | 'exact_output'
  trader: string
  input_limit: string
  output_limit: string
  deadline: string
  path: string[]
  amounts: string[]
  input: ExchangeLeg
  output: ExchangeLeg
  hops: {
    index: number
    pool_contract: string
    input: ExchangeLeg
    output: ExchangeLeg
    input_event_index: number
    output_event_index: number
  }[]
  router_event_index: number
  transfer_event_indexes: number[]
}

export interface AquariusSwapHop {
  index: number
  pool_tokens: string[]
  pool_index: string
  pool_contract: string
  input: ExchangeLeg
  output: ExchangeLeg
  input_event_index: number
  output_event_index: number
  trade_event_index: number
  router_event_index: number
}

export interface AquariusSettlement {
  funded_amount: string
  refunded_amount: string
  funding_event_index: number
  refund_event_index: number | null
  payout_event_index: number
}

export interface AquariusSwapRoute {
  index: number
  weight_bps: number
  path: string[]
  input: ExchangeLeg
  output: ExchangeLeg
  hop_indexes: number[]
}

export interface AquariusSwap {
  protocol: 'aquarius'
  adapter_version: number
  verification: 'registered_router_events_and_transfers'
  router_contract: string
  router_wasm_hash: string | null
  mode: 'exact_input' | 'exact_output'
  routing: 'direct' | 'chained' | 'split'
  trader: string
  input_limit: string
  output_limit: string
  path: string[]
  input: ExchangeLeg
  output: ExchangeLeg
  hops: AquariusSwapHop[]
  transfer_event_indexes: number[]
  settlement?: AquariusSettlement
  routes?: AquariusSwapRoute[]
}

export interface PathPayment {
  mode: 'strict_send' | 'strict_receive'
  send_asset: Asset
  destination_asset: Asset
  send_limit: string
  destination_limit: string
  path: Asset[]
  execution: {
    protocol: 'stellar_dex'
    input: ExchangeLeg
    output: ExchangeLeg
    trades: {
      claim_index: number
      venue: 'order_book' | 'liquidity_pool'
      input: ExchangeLeg
      output: ExchangeLeg
    }[]
  } | null
}

export interface Action {
  id: string
  kind: string
  title: string
  description: string
  status: 'applied' | 'failed' | 'not_applied'
  source: 'operation' | 'token_event'
  op_index: number
  event_indexes: number[]
  source_account?: string
  from?: string
  to?: string
  contract_id?: string
  function?: string
  amount: string | null
  asset: Asset | null
  to_muxed?: unknown
  result_code: string | null
  path_payment?: PathPayment
  contract_swap?: ContractSwap
  aquarius_swap?: AquariusSwap
}

export interface FeeSummary {
  status: 'complete' | 'unavailable' | 'invalid'
  source: 'transaction_events' | 'unavailable'
  charged: string | null
  refunded: string | null
  net: string | null
  reason: string | null
  events: {
    event_index: number
    account: string
    amount: string
    stage: string
  }[]
}
export interface SyncState {
  network: string
  first_indexed_ledger: number | null
  last_indexed_ledger: number | null
  latest_network_ledger: number | null
  lag: number | null
  observed_at: string | null
  last_error: string | null
  history_complete: boolean
  processed_transactions: number
  error_count: number
}
export interface SearchMatch {
  type: 'ledger' | 'transaction' | 'account' | 'contract'
  key: string
  path: string
}

export interface SearchSuggestions {
  network: string
  query: string
  limit: number
  items: SearchMatch[]
}

export interface Overview {
  sync: SyncState
  ledgers: Page<Ledger>
  transactions: Page<Transaction>
}

export interface ActivityBucket {
  first_ledger: number
  last_ledger: number
  closed_at: string
  count: number
}

export interface Activity {
  network: string
  kind: 'transactions' | 'transfers' | 'events'
  window_limit: number
  first_ledger: number | null
  last_ledger: number | null
  ledger_count: number
  from: string | null
  to: string | null
  total: number
  buckets: ActivityBucket[]
  breakdown: { key: string; count: number }[]
}

export interface AddressDetail {
  address: string
  source: string
  history_semantics: string
  range: SyncState
  transactions: Page<Transaction>
  events?: Page<Event>
}

export interface Contract {
  network: string
  contract_id: string
  kind: 'unknown' | 'sac' | 'wasm'
  wasm_hash: string | null
  executable_source: 'instance_entry' | 'asset_contract_id' | null
  executable_ledger: number | null
  executable_tx_index: number | null
  first_observed_ledger: number
  last_observed_ledger: number
  creation_ledger: number | null
  creation_transaction: string | null
  creator: string | null
}

export interface ContractDetail extends AddressDetail {
  contract: Contract
}

export interface Asset {
  network: string
  contract_id: string
  kind: 'sac' | 'contract'
  verification: 'contract_id' | 'event_shape'
  classic_asset: string | null
  code: string | null
  issuer: string | null
  decimals: number | null
  first_observed_ledger: number
}

export interface TokenTransfer {
  transaction_hash: string
  ledger_sequence: number
  tx_index: number
  event_index: number
  op_index: number
  kind: 'transfer' | 'mint' | 'burn' | 'clawback'
  asset: Asset
  from?: string
  to?: string
  amount: string
  to_muxed?: unknown
  created_at: string
}

export interface AssetDetail {
  asset: Asset
  transfers: Page<TokenTransfer>
  range: SyncState
}

export interface NFTCollection {
  network: string
  contract_id: string
  verification: 'event_shape'
  first_observed_ledger: number
  observed_token_count: number
  transfer_count: number
}

export interface NFTTransfer {
  transaction_hash: string
  ledger_sequence: number
  tx_index: number
  event_index: number
  op_index: number
  kind: 'transfer' | 'mint' | 'burn'
  contract_id: string
  token_id: string
  token_id_type: 'u32' | 'u64' | 'u128' | 'u256'
  from?: string
  to?: string
  created_at: string
  verification: 'event_shape'
  event_standard: 'sep50' | 'openzeppelin_burnable'
  format: 'scalar' | 'token_id_map'
}

export interface NFTCollectionDetail {
  collection: NFTCollection
  transfers: Page<NFTTransfer>
  range: SyncState
  snapshot_ledger: number
}

export interface NFTItem {
  network: string
  contract_id: string
  token_id: string
  first_observed_ledger: number
  transfer_count: number
  verification: 'event_shape'
}

export interface NFTItemDetail {
  item: NFTItem
  transfers: Page<NFTTransfer>
  range: SyncState
  snapshot_ledger: number
}
