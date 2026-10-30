# bze-aggregator-api

### Database schema
The MySQL/MariaDB schema lives in `docker/mysql/init/01-schema.sql` and is
applied automatically on the first start of the `docker/docker-compose.yml`
database (empty data dir). To apply it to an existing server, pipe it in
manually. Index choices are documented inline in the schema file — keep them
in sync with `app/repository` queries.

The PostgreSQL side (`POSTGRES_DSN`) is **not** ours: it is the CometBFT
event index written by a BZE node running with `indexer = "psql"`; its schema
ships with the node.

### Environment
`.env` file can contain:  
```
HTTP_PORT=8888 (default: 8888)
LOG_LEVEL=info (optons: panic, fatal, error, warning, info, debug, trace.  default: info)

BLOCKCHAIN_RPC_HOST=https://testnet-rpc.getbze.com
BLOCKCHAIN_REST_HOST=https://testnet.getbze.com
```

`MYSQL_DSN` must carry `parseTime=true` and `timeTruncate=1us`, e.g.
`user:password@tcp(127.0.0.1:3306)/dbname?parseTime=true&timeTruncate=1us`.
Without `timeTruncate` the driver sends time parameters with nanosecond
precision and MariaDB cannot use an index range on them, which makes
`/api/dex/tickers` and `/api/dex/intervals` scan far more rows than needed.

### Endpoints
1. `Health` - endpoint to check if a market is healthy (has active trades) in the last X minutes  
`/api/health/market?market_id={market_id}&minutes={minutes}`

2. `Total Supply` - endpoint to get the total supply of BZE  
`/api/supply/total`

3. `Circulating Supply` - endpoint to get the circulating supply of BZE  
`/api/supply/circulating`

4. `DEX Tickers` - endpoint to get the tickers of all markets  
`/api/dex/tickers`  
Query Params:  
   - `format` - optional query param to format the response.  Options: `coingecko`

Response:  
```json
[
    {
        "base": "ibc/ED07A3391A112B175915CD8FAF43A2DA8E4790EDE12566649D0C2F97716B8518",
        "quote": "ubze",
        "market_id": "ibc/ED07A3391A112B175915CD8FAF43A2DA8E4790EDE12566649D0C2F97716B8518/ubze",
        "last_price": 371.467177,
        "base_volume": 35418.105262,
        "quote_volume": 13376177.651923,
        "bid": 379.380065,
        "ask": 385,
        "high": 384.99585,
        "low": 370,
        "open_price": 371.402764,
        "change": 0.02
    },
    {
        "base": "ubze",
        "quote": "ibc/6490A7EAB61059BFC1CDDEB05917DD70BDF3A611654162A1A47DB930D40D8AF4",
        "market_id": "ubze/ibc/6490A7EAB61059BFC1CDDEB05917DD70BDF3A611654162A1A47DB930D40D8AF4",
        "last_price": 0.001402,
        "base_volume": 39429308.253886,
        "quote_volume": 53266.054683,
        "bid": 0.0011,
        "ask": 0.0016,
        "high": 0.001597,
        "low": 0.00111,
        "open_price": 0.001508,
        "change": -7.03
    }
]
```  
Using `format=coingecko` query param would return the following response:  
```json
[
    {
        "ticker_id": "ubze_ibc/6490A7EAB61059BFC1CDDEB05917DD70BDF3A611654162A1A47DB930D40D8AF4",
        "base_currency": "ubze",
        "target_currency": "ibc/6490A7EAB61059BFC1CDDEB05917DD70BDF3A611654162A1A47DB930D40D8AF4",
        "pool_id": "ubze/ibc/6490A7EAB61059BFC1CDDEB05917DD70BDF3A611654162A1A47DB930D40D8AF4",
        "last_price": 0.00133,
        "base_volume": 33531556.231123,
        "target_volume": 45416.389246,
        "bid": 0.0011,
        "ask": 0.0016,
        "high": 0.0016,
        "low": 0.00111
    },
    {
        "ticker_id": "ibc/ED07A3391A112B175915CD8FAF43A2DA8E4790EDE12566649D0C2F97716B8518_ubze",
        "base_currency": "ibc/ED07A3391A112B175915CD8FAF43A2DA8E4790EDE12566649D0C2F97716B8518",
        "target_currency": "ubze",
        "pool_id": "ibc/ED07A3391A112B175915CD8FAF43A2DA8E4790EDE12566649D0C2F97716B8518/ubze",
        "last_price": 448.675272,
        "base_volume": 32312.759469,
        "target_volume": 13352520.463762,
        "bid": 425.112641,
        "ask": 455,
        "high": 455,
        "low": 370
    }
]
```  

#### Merged markets (CoinGecko format only)

`COINGECKO_MERGED_MARKETS` folds a market into one CoinGecko already tracks, e.g. when a market's
quote asset is replaced (Noble USDC by Injective USDC). With `format=coingecko`:

- `tickers`: the tracked ticker sums both markets' 24h volumes, spans their high/low and takes
  `last_price` and `bid`/`ask` from the new market when it has them; the new market's ticker is
  not listed, so the volume is counted once.
- `orders` (`ticker_id` of the tracked market): the new market's book when it has orders, else the
  tracked market's. Books are never mixed.
- `history` (`ticker_id` of the tracked market): the trades of both markets, newest first, cut to
  `limit`.

The default format, and the data collection, keep every market separate.

5. `DEX Orders` - endpoint to get the orders of a market  
`/api/dex/orders?market_id={market_id}&limit={limit}`  

Query Params:  
   - `market_id` - required (if ticker_id not present)  
   - `ticker_id` - optional query param to get orders for a specific ticker_id.  If present, market_id is not required.
   - `format` - optional query param to format the response.  Options: `coingecko`
   - `depth` - optional query param to get the order book depth.  Default: 10

Response:
```json
{
    "market_id": "factory/bze13gzq40che93tgfm9kzmkpjamah5nj0j73pyhqk/uvdl/ubze",
    "timestamp": "1732031048",
    "bids": [
        {
            "price": "0.000001",
            "volume": "10000000"
        },
        {
            "price": "0.0001",
            "volume": "100000"
        }
    ],
    "asks": [
        {
            "price": "4.2",
            "volume": "696966"
        },
        {
            "price": "4.3",
            "volume": "42000"
        }
    ]
}
```  

6. `DEX History` - endpoint to get the history of a market    
`/api/dex/history?market_id={market_id}&limit={limit}&address={address}`  
Query Params:  
   - `market_id` - required  
   - `limit` - optional query param to limit the number of results.  Default: 10
   - `address` - optional query param to get the history of a specific address
   - `format` - optional query param to format the response.  Options: `coingecko`
   - `ticker_id` - optional query param to get history for a specific ticker_id.  If present, market_id is not required.
   - `start_time` - optional query param to get history after a specific time.  Format: `timestamp in milliseconds`
   - `end_time` - optional query param to get history before a specific time.  Format: `timestamp in milliseconds`
   - `type` - optional query param to get history of a specific order type. Options: `buy`, `sell`

Response:
```json
[
    {
        "order_id": 436157,
        "price": "0.00156",
        "base_volume": "12",
        "quote_volume": "0.01872",
        "executed_at": "1731016441000",
        "order_type": "sell"
    },
    {
        "order_id": 436316,
        "price": "0.00148",
        "base_volume": "10",
        "quote_volume": "0.0148",
        "executed_at": "1730998672000",
        "order_type": "buy"
    }
]
```  
7. `DEX Intervals` - endpoint to get the intervals of a market  
`/api/dex/intervals?market_id={market_id}&limit={limit}&minutes={minutes}`    

Query Params:  
   - `market_id` - required  
   - `ticker_id` - optional query param to get intervals for a specific ticker_id.  If present, market_id is not required.
   - `limit` - optional query param to limit the number of results.  Default: 10
   - `minutes` - optional query param to get intervals for a specific time frame.

Response: 
```json
    {
        "market_id": "ubze/ibc/6490A7EAB61059BFC1CDDEB05917DD70BDF3A611654162A1A47DB930D40D8AF4",
        "minutes": 60,
        "start_at": "2024-11-19T10:00:00+02:00",
        "end_at": "2024-11-19T11:00:00+02:00",
        "lowest_price": "0",
        "open_price": "0",
        "average_price": "0",
        "highest_price": "0",
        "close_price": "0",
        "base_volume": "0",
        "quote_volume": "0"
    },
    {
        "market_id": "ubze/ibc/6490A7EAB61059BFC1CDDEB05917DD70BDF3A611654162A1A47DB930D40D8AF4",
        "minutes": 60,
        "start_at": "2024-11-19T09:00:00+02:00",
        "end_at": "2024-11-19T10:00:00+02:00",
        "lowest_price": "0",
        "open_price": "0",
        "average_price": "0",
        "highest_price": "0",
        "close_price": "0",
        "base_volume": "0",
        "quote_volume": "0"
    }
]
```

Release build  
`GOOS=linux GOARCH=amd64 go build -o bze-agg-linux_amd64`

### Comparing with BZE Scan

BZE Scan serves copies of the aggregator's endpoints under `/aggregator` (e.g.
`https://explorer.getbze.com/aggregator/api/dex/tickers`). `bze-agg compare` checks that its DEX
endpoints answer what the aggregator answers before traffic moves to them. It needs no `.env`, no
database and no node: build the binary and run it anywhere with access to both hosts.

```
./bze-agg compare <orders|swaps|history|intervals|tickers|all> [--agg https://getbze.com] \
    [--scan https://explorer.getbze.com/aggregator] [--interval 20m] [--duration 24h]
```

- `--agg`, `--scan`: the base URLs; the same `/api/dex/...` path and query are appended to both.
- `--interval`: time between the starts of two rounds; the first round starts right away.
- `--duration`: how long to run (the defaults give 72 rounds).

Each round reads the aggregator's tickers (and, for `swaps` and `history`, recent swaps and trades)
to build the requests it compares: every order book market's orders, the takers of the newest
swaps, every market's and pool's history, candles and the tickers themselves. Each request is
fetched from the aggregator, from Scan, then from the aggregator again, one request at a time and
with a `bze-agg-compare` user agent; it diverges only when Scan's answer differs from both
aggregator answers, so a trade landing in between is not reported. Before comparing, both answers
are normalised for the differences Scan is allowed: stable trade ids, block times in milliseconds,
the order of trades within one second, trades the aggregator lost within a second it has, the
v8.0.x doubled events (inside their block range only), swaps with a `_` in a denom, UTC times,
leading zero candles (and a 400 for a pool without a swap), the order of tickers, `null` against
`[]`, and a `liquidity_in_usd` Scan lists where the aggregator has none (the newer `ulp/<hash>`
pools). On a full page only the seconds newer than the oldest second of each full side are compared.
Numbers may differ by a relative 1e-12 (1e-3 for `liquidity_in_usd`). Prices of markets whose quote
has three or more decimals more than the base are right on Scan only and show up as differences.

The output has one line per round and endpoint (`round N <endpoint>: X requests, Y divergent`),
then at most 20 lines per round naming each difference's request, JSON path, aggregator value and
Scan value, and a summary per endpoint at the end. The exit code is 1 when any round diverged.

To run it for 24 hours detached:

```
nohup ./bze-agg compare all > compare.log 2>&1 &
```

Stopping it (`kill <pid>`) prints the summary of the rounds so far.

### Proxying to BZE Scan

The API can answer endpoints from BZE Scan, which serves the same paths and responses under its
`/aggregator` base, while the aggregator keeps syncing so its own handlers stay ready as the
failover. Two `.env` keys control it:

- `SCAN_API_HOST`: Scan's base URL including `/aggregator`, e.g.
  `https://explorer.getbze.com/aggregator`. Scan runs blue/green on two ports, so use a stable
  address such as its public host rather than one backend port.
- `SCAN_PROXIED_ENDPOINTS`: comma-separated names of the endpoints to proxy:

| Name | Routes |
| --- | --- |
| `prices` | `/api/prices` |
| `supply` | `/api/supply/total`, `/api/supply/circulating` |
| `articles` | `/api/articles/medium` |
| `orders` | `/api/dex/orders` |
| `swaps` | `/api/dex/swaps` |
| `history` | `/api/dex/history` |
| `intervals` | `/api/dex/intervals` |
| `tickers` | `/api/dex/tickers` |

`/api/health/*` is never proxied: it reports on the aggregator itself. The API refuses to start on
an unknown name or on a non-empty list without `SCAN_API_HOST`; an empty list proxies nothing.

A proxied request goes to `SCAN_API_HOST` plus the original path and query (10 s timeout), and
Scan's status, `Content-Type` and body are returned as they are. CORS headers stay the
aggregator's own. **Failover:** when Scan cannot be reached, times out or answers 5xx, the API
logs one warning naming the endpoint and answers from its own handler. A 4xx from Scan is a valid
answer (e.g. a bad parameter) and is passed through.

Roll back by emptying `SCAN_PROXIED_ENDPOINTS` and restarting the API.
