package handlers

import (
	"fmt"
	"slices"

	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
)

type intervalStorage interface {
	SyncIntervals(marketId string) error
}

type MarketIntervalSync struct {
	mProvider  marketProvider
	lpProvider lpProvider
	storage    intervalStorage
	logger     logrus.FieldLogger
}

func NewMarketIntervalSync(logger logrus.FieldLogger, provider marketProvider, storage intervalStorage, lpProvider lpProvider) (*MarketIntervalSync, error) {
	if logger == nil || provider == nil || storage == nil || lpProvider == nil {
		return nil, internal.NewInvalidDependenciesErr("NewMarketIntervalSync")
	}

	return &MarketIntervalSync{
		mProvider:  provider,
		lpProvider: lpProvider,
		storage:    storage,
		logger:     logger,
	}, nil
}

// SyncIntervals rebuilds the candles of a single order book market or liquidity
// pool. The two live in separate id namespaces - markets are "base/quote" and
// pools are "base_quote" - and swap history is stored under the pool id, so an
// id is looked for among the pools as well as among the chain's markets.
func (m *MarketIntervalSync) SyncIntervals(id string) error {
	isPool, err := m.isLiquidityPool(id)
	if err != nil {
		return err
	}

	if isPool {
		// pool candles are built straight out of market_history, which the swap
		// sync keys by pool id, so there is no chain market to resolve first.
		// This is the same call the listener makes when it sees a SwapEvent.
		return m.storage.SyncIntervals(id)
	}

	for _, market := range getMarkets(m.mProvider, m.logger) {
		if converter.GetMarketId(market.GetBase(), market.GetQuote()) != id {
			continue
		}

		return m.syncInterval(&market)
	}

	return fmt.Errorf("%s is neither a known market nor a known liquidity pool", id)
}

// SyncAll rebuilds the candles of every market and every liquidity pool: both
// of them write into market_history, so covering only the markets would leave
// the pools with no way of being rebuilt at all.
func (m *MarketIntervalSync) SyncAll() {
	syncAll(m.mProvider, m.logger, m.syncInterval)

	pools, err := m.lpProvider.GetAllLiquidityPoolsIds()
	if err != nil {
		m.logger.WithError(err).Error("could not get the liquidity pools to use in SyncAll")

		return
	}

	for _, poolId := range pools {
		l := m.logger.WithField("pool_id", poolId)
		if err = m.storage.SyncIntervals(poolId); err != nil {
			l.WithError(err).Error("could not sync the intervals of this liquidity pool")

			continue
		}
	}
}

func (m *MarketIntervalSync) isLiquidityPool(id string) (bool, error) {
	pools, err := m.lpProvider.GetAllLiquidityPoolsIds()
	if err != nil {
		return false, fmt.Errorf("could not get the liquidity pools: %w", err)
	}

	return slices.Contains(pools, id), nil
}

func (m *MarketIntervalSync) syncInterval(market *types.Market) error {
	return m.storage.SyncIntervals(types.CreateMarketId(market.GetBase(), market.GetQuote()))
}
