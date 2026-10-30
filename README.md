# bze-aggregator-api

A small HTTP service that keeps the aggregator API at its public address.
Every `/api` request except the health checks is answered by
[BZE Scan](https://github.com/bze-alphateam/bze-scan), which serves the same
paths with identical responses under its `/aggregator` base. The service has
no database of its own.

### Environment
All configuration is read from a `.env` file in the working directory (see
`.env.dist`):

```
HTTP_PORT=8888                 # default 8000
LOG_LEVEL=info                 # panic, fatal, error, warning, info, debug, trace
SCAN_API_HOST=https://explorer.getbze.com/aggregator
BLOCKCHAIN_REST_HOST=https://rest.getbze.com
HEALTH_NODES=node1=https://rpc1.example:443,node2=https://rpc2.example:443
PREFIXED_REST_HOSTS=bze=https://rest.getbze.com,osmo=https://lcd.osmosis.zone
```

`SCAN_API_HOST` and `BLOCKCHAIN_REST_HOST` are required.

### Proxied endpoints
`GET /api/<path>?<query>` is forwarded to `GET $SCAN_API_HOST/api/<path>?<query>`:
`/api/prices`, `/api/supply/total`, `/api/supply/circulating`,
`/api/articles/medium`, `/api/dex/tickers`, `/api/dex/orders`,
`/api/dex/history`, `/api/dex/intervals`, `/api/dex/swaps`, and any other path
Scan serves under that base. See Scan's documentation for parameters and
response shapes.

- Scan's status code, `Content-Type` and body are returned unchanged, errors
  included. The CORS headers are this service's own (`*`).
- When Scan does not answer (connection error or 10 s timeout) the response is
  `502 {"message":"upstream unavailable"}` and a warning is logged.

### Health endpoints
Answered by this service, never proxied.

1. `GET /api/health/market?market_id={market_id}&minutes={minutes}` — whether
   the market traded on chain in the last `minutes` (default 10), read from
   `BLOCKCHAIN_REST_HOST`.
   ```json
   {"is_healthy": true, "last_trade": "2026-10-30T10:00:00Z"}
   ```

2. `GET /api/health/nodes` — queries every `HEALTH_NODES` RPC; unhealthy when
   one fails or is more than 2 blocks behind another.
   ```json
   {"is_healthy": true, "errors": ""}
   ```

3. `POST /api/health/balances` — checks that each address holds at least
   `min_amount` of `denom`, using the REST host whose `PREFIXED_REST_HOSTS`
   prefix matches the address.
   ```json
   {"addresses": [{"address": "bze1...", "denom": "ubze", "min_amount": 1000000}]}
   ```
   Response:
   ```json
   {"is_healthy": false, "errors": "[bze1...]: [balance [10] is less than min amount [1000000]]"}
   ```

### Build
```
go build ./...
go vet ./...
go test ./... -race
```

The production image is `docker/prod/Dockerfile`; mount the runtime `.env` at
`/app/.env`.
