## Sync Flows Overview
The aggregator keeps its own MySQL tables in sync by pulling data from two places:
- The blockchain itself via gRPC/RPC (market state, orders, history, liquidity).
- A PostgreSQL database that the blockchain node indexes with every block/Tx event (used to hydrate swap history quickly).

Below is a plain-English rundown of what comes from where and how it is processed.

## Data Sources
- **Blockchain gRPC** – `tradebin` query endpoints provide current markets, active order books, trade history, and liquidity pool state. Hosts are set with `BLOCKCHAIN_GRPC_HOST`/`BLOCKCHAIN_RPC_HOST`.
- **Node-indexed PostgreSQL** – a BZE node configured with the Postgres indexer writes block data into `blocks`, `events`, and `attributes`. Each event row has `status` (0 = new) and is keyed to its block (`blocks.created_at` carries the block time). The aggregator reads this database via `POSTGRES_DSN`.

## What Gets Synced From gRPC
- **Markets** (`sync markets`): `AllMarkets` is fetched and stored if missing. If history already exists, the created timestamp is set just before the first on-chain order so charts align.
- **Active orders** (`sync orders`): Buy/sell aggregated books are queried per market and upserted.
- **Trade history** (`sync history`): Paginates over `MarketHistory`, resuming from the last stored trade. Converts amounts/prices using chain-registry metadata for correct decimals, then writes to `market_history`.
- **Intervals/candles** (`sync intervals`): Rebuilds candles from stored history by grouping trades into time buckets and marking processed trades (`i_added_to_interval`).
- **Liquidity pools** (`sync liquidity`): Pulls all pools, back-fills missing markets, and updates pool balances/liquidity stats.

## What Gets Synced From PostgreSQL
- **Swap events** (`sync events` or via listener):
  1) Read unprocessed `bze.tradebin.SwapEvent` rows from `events` (joined with `blocks` for height/time).
  2) Load key/value pairs from `attributes` to reconstruct the swap payload.
  3) Convert using chain-registry metadata to human units, fetch exact block time from RPC (cached), and create a `market_history` row.
  4) Mark the Postgres event as processed (`status = 1`) so it is never re-ingested.
  5) Return the touched pool IDs so liquidity can be refreshed.

This flow lets us ingest swaps as soon as the node indexes them, without waiting for gRPC history pagination.

## LP Swap Back-Fill (Disaster Recovery)
`bze-agg swap-backfill` recovers pool swaps that no longer exist anywhere else. Order-book history can always be re-paginated from chain state, but LP swaps only ever existed as emitted `bze.tradebin.SwapEvent`s: if the node's Postgres index is lost, walking historical blocks is the only way back. Asking the node to search by event type (`tx_search`) is not an option - it overloads archive nodes.

Everything is staged first and committed as a separate, confirmed step, so the command can run for days next to a working listener without the two sharing a single row, candle or flag:
- `init [height]` - reports the oldest swap still held per pool (that timestamp is where the data stops; nothing on chain or in either database records it) and creates `temp_checkpoint` + `temp_market_history` for the range `height` down to the liquidity pools upgrade at 20237800.
- `run` - walks the range descending, skips failed transactions, picks up both transaction and block-hook swaps (the hourly fee conversion swaps through the pools too), parses the swap events through the same converters as the live sync, and stages what it finds. Rows and checkpoint advance in one transaction per chunk, so an interrupted run resumes exactly where it stopped.
- `commit` - drops staged rows the listener has already ingested (anything at or after a pool's oldest live trade), then inserts the rest one day at a time. Each day is a single transaction: rows, candles and the staged rows' `committed` flag land together, which is what makes a killed commit safe to re-run. The day the live data starts in is committed with `i_added_to_interval = 0` and no candles - the listener owns that window and rebuilds it from the complete set. Finally each pool's `market.i_created_at` is moved back to cover the recovered candles (the intervals API uses it as a hard floor) and the staging tables are renamed to `swap_backfill_*` as a record of what was inserted.
- `cleanup` - discards a back-fill. Refuses once anything was committed: the staging tables are then the only record of it.

## Listener Flow (Live Sync)
`bze-agg sync listener` wires everything together:
- Subscribes to Tendermint WebSocket events for `tradebin` types.
- Runs an initial full sync (markets → liquidity pools → swap events → history → orders → intervals).
- On each event type, it triggers only the necessary syncs (e.g., `OrderExecuted` → history + intervals + orders; `SwapEvent` → Postgres swap processing + liquidity refresh).
- In-memory locks ensure each market/pool is processed once at a time.

## Where the Data Ends Up
- **MySQL** stores the aggregator view (`market`, `market_history`, `market_orders`, `market_liquidity_data`, `market_history_interval`).
- **PostgreSQL** remains the node’s event index; only read for swap ingestion and cleaned with `cleanup` when needed.
