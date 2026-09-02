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

## Listener Flow (Live Sync)
`bze-agg sync listener` wires everything together:
- Subscribes to Tendermint WebSocket events for `tradebin` types.
- Runs an initial full sync (markets → liquidity pools → swap events → history → orders → intervals).
- On each event type, it triggers only the necessary syncs (e.g., `OrderExecuted` → history + intervals + orders; `SwapEvent` → Postgres swap processing + liquidity refresh).
- In-memory locks ensure each market/pool is processed once at a time.

## Where the Data Ends Up
- **MySQL** stores the aggregator view (`market`, `market_history`, `market_orders`, `market_liquidity_data`, `market_history_interval`).
- **PostgreSQL** remains the node’s event index; only read for swap ingestion and cleaned with `cleanup` when needed.
