# bze-aggregator-api



### Environment
`.env` file can contain:  
```
HTTP_PORT=8888 (default: 8888)
LOG_LEVEL=info (optons: panic, fatal, error, warning, info, debug, trace.  default: info)

BLOCKCHAIN_RPC_HOST=https://testnet-rpc.getbze.com
BLOCKCHAIN_REST_HOST=https://testnet.getbze.com
```

Release build  
`GOOS=linux GOARCH=amd64 go build -o bze-agg-linux_amd64`
