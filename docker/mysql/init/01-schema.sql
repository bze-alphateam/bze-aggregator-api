-- MySQL/MariaDB schema for the aggregator's own store (MYSQL_DSN).
-- Reconstructed from app/entity structs and app/repository queries; the
-- indexes below exist to serve specific queries — keep them in sync when
-- repository queries change.
--
-- Amount/price columns are strings on purpose: the chain uses arbitrary
-- precision decimals (LegacyDec) and the Go code scans them as strings.
-- market_order.price_dec is the only numeric price, used for order-book
-- sorting.

-- market: one row per DEX market.
-- UNIQUE market_id backs MarketRepository.SaveIfNotExists (ON DUPLICATE KEY)
-- and all market_id lookups/joins.
CREATE TABLE IF NOT EXISTS market (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    market_id VARCHAR(256) NOT NULL,
    base VARCHAR(128) NOT NULL,
    quote VARCHAR(128) NOT NULL,
    created_by VARCHAR(64) NOT NULL,
    i_created_at DATETIME NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_market_market_id (market_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

-- market_history: executed trades (order book + swaps). Insert-mostly, no
-- unique key: dedupe is done by delete-then-insert on (market_id, executed_at).
-- executed_at is DATETIME(3) because the chain provides millisecond
-- timestamps and SaveMarketHistoryOrders deletes by exact executed_at values.
--
-- Index coverage:
--   idx_mh_market_executed          GetLastHistoryOrder, GetByExecutedAt,
--                                   GetFirstMarketOrderTime, the delete in
--                                   SaveMarketHistoryOrders, GetHistoryBy
--   idx_mh_market_type_executed     GetHistoryBy with the type filter
--   idx_mh_market_pending_interval  GetOldestNotAddedToInterval
--   idx_mh_executed                 GetMarketsWithLastExecuted (range on
--                                   executed_at alone, across all markets)
--   idx_mh_maker / idx_mh_taker     address filters (OR of the two columns —
--                                   served via index_merge)
CREATE TABLE IF NOT EXISTS market_history (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    market_id VARCHAR(256) NOT NULL,
    order_type VARCHAR(16) NOT NULL,
    amount VARCHAR(80) NOT NULL,
    price VARCHAR(80) NOT NULL,
    executed_at DATETIME(3) NOT NULL,
    maker VARCHAR(64) NOT NULL,
    taker VARCHAR(64) NOT NULL,
    i_quote_amount VARCHAR(80) NOT NULL,
    i_created_at DATETIME NOT NULL,
    i_added_to_interval TINYINT(1) NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    KEY idx_mh_market_executed (market_id, executed_at),
    KEY idx_mh_market_type_executed (market_id, order_type, executed_at),
    KEY idx_mh_market_pending_interval (market_id, i_added_to_interval, executed_at),
    KEY idx_mh_executed (executed_at),
    KEY idx_mh_maker (maker),
    KEY idx_mh_taker (taker)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

-- market_order: current order book snapshot per market. Rewritten wholesale
-- by MarketOrderRepository.Upsert (DELETE by market_id + INSERT), so no
-- unique key. The composite index serves GetMarketOrdersWithDepth,
-- GetHighestBuy and GetLowestSell (filter on market_id + order_type, sort on
-- price_dec) and, by prefix, the DELETE by market_id.
CREATE TABLE IF NOT EXISTS market_order (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    market_id VARCHAR(256) NOT NULL,
    order_type VARCHAR(16) NOT NULL,
    amount VARCHAR(80) NOT NULL,
    price VARCHAR(80) NOT NULL,
    price_dec DOUBLE NOT NULL,
    i_quote_amount VARCHAR(80) NOT NULL,
    i_created_at DATETIME NOT NULL,
    PRIMARY KEY (id),
    KEY idx_mo_market_type_price (market_id, order_type, price_dec)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

-- market_history_interval: OHLCV candles. The UNIQUE key backs the upsert in
-- MarketIntervalRepository.Save (ON DUPLICATE KEY UPDATE) and also serves
-- every read (filter on market_id + length, range/sort on start_at).
CREATE TABLE IF NOT EXISTS market_history_interval (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    market_id VARCHAR(256) NOT NULL,
    length INT NOT NULL,
    start_at DATETIME NOT NULL,
    end_at DATETIME NOT NULL,
    lowest_price VARCHAR(80) NOT NULL,
    open_price VARCHAR(80) NOT NULL,
    average_price VARCHAR(80) NOT NULL,
    highest_price VARCHAR(80) NOT NULL,
    close_price VARCHAR(80) NOT NULL,
    base_volume VARCHAR(80) NOT NULL,
    quote_volume VARCHAR(80) NOT NULL,
    i_created_at DATETIME NOT NULL,
    i_updated_at DATETIME NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_mhi_market_length_start (market_id, length, start_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

-- market_liquidity_data: one row per AMM pool. The UNIQUE key backs the
-- upsert in MarketLiquidityDataRepository.SaveOrUpdate and the join from
-- market_history in GetAddressSwapHistory.
CREATE TABLE IF NOT EXISTS market_liquidity_data (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    market_id VARCHAR(256) NOT NULL,
    lp_denom VARCHAR(128) NOT NULL,
    fee VARCHAR(80) NOT NULL,
    reserve_base VARCHAR(80) NOT NULL,
    reserve_quote VARCHAR(80) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_mld_market (market_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
