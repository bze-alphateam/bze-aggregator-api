# bze-aggregator-api

### Environment
`.env` file can contain:  
```
HTTP_PORT=8888 (default: 8888)
LOG_LEVEL=info (optons: panic, fatal, error, warning, info, debug, trace.  default: info)

BLOCKCHAIN_RPC_HOST=https://testnet-rpc.getbze.com
BLOCKCHAIN_REST_HOST=https://testnet.getbze.com
```

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
