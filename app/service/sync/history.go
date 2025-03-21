package sync

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	requestedHistoryLength uint64 = 500
)

type assetProvider interface {
	GetAssetDetails(denom string) (*chain_registry.ChainRegistryAsset, error)
}

type historyProvider interface {
	GetMarketHistory(marketId string, limit uint64, key string) (list []types.HistoryOrder, paginationKey string, err error)
}

type historyStorage interface {
	GetLastHistoryOrder(marketId string) (*entity.MarketHistory, error)
	SaveMarketHistoryOrders(marketId string, orders []*entity.MarketHistory, clearExecutedAt []time.Time) error
}

type History struct {
	logger logrus.FieldLogger

	dataProvider  historyProvider
	storage       historyStorage
	assetProvider assetProvider
	locker        locker
}

func NewHistorySync(logger logrus.FieldLogger, dataProvider historyProvider, storage historyStorage, assetProvider assetProvider, l locker) (*History, error) {
	if logger == nil || dataProvider == nil || storage == nil || assetProvider == nil || l == nil {
		return nil, internal.NewInvalidDependenciesErr("NewHistorySync")
	}

	return &History{
		logger:        logger.WithField("service", "HistorySync"),
		dataProvider:  dataProvider,
		storage:       storage,
		assetProvider: assetProvider,
		locker:        l,
	}, nil
}

// SyncHistory syncs the history orders for the given market.
// It resumes from the last order found in DB for this market
// if batchSize is 0, it will use the value of requestedHistoryLength constant as limit
func (h *History) SyncHistory(market *types.Market, batchSize uint64) error {
	marketId := converter.GetMarketId(market.GetBase(), market.GetQuote())

	h.locker.Lock(getHistoryLockKey(marketId))
	defer h.locker.Unlock(getHistoryLockKey(marketId))

	histLimit := requestedHistoryLength
	if batchSize > 0 {
		histLimit = batchSize
	}

	l := h.logger.WithField("market", marketId).WithField("process", "SyncHistory")
	l.Info("preparing to sync history")
	conv, err := converter.NewTypesConverter(h.assetProvider, market)
	if err != nil {
		return err
	}

	l.Info("fetching last order from market's history")
	last, err := h.storage.GetLastHistoryOrder(marketId)
	if err != nil {
		return err
	}

	if last == nil {
		l.Info("no last order found. Will sync the entire history")
	}

	l.Info("starting loop to fetch history")
	var key string
	for {
		l.Info("fetching market history from blockchain")
		hist, next, err := h.dataProvider.GetMarketHistory(marketId, histLimit, key)
		if err != nil {
			return err
		}

		if len(hist) == 0 {
			l.WithField("key", key).WithField("limit", histLimit).Info("no history found on the blockchain")
			break
		}

		done, err := h.syncHistoryList(market, hist, last, conv)
		if err != nil {
			return err
		}

		if done || next == "" {
			l.Info("finished syncing history")
			break
		}

		key = next
	}

	return nil
}

func (h *History) syncHistoryList(market *types.Market, list []types.HistoryOrder, lastSyncedOrder *entity.MarketHistory, conv *converter.TypesConverter) (finished bool, err error) {
	marketId := converter.GetMarketId(market.GetBase(), market.GetQuote())
	l := h.logger.WithField("market", marketId)
	l.Info("syncing history list")
	if len(list) == 0 {
		l.Info("no history found on the blockchain")
		return true, nil
	}

	var toUpdate []*entity.MarketHistory
	var toClear []time.Time
	for _, order := range list {
		if lastSyncedOrder != nil && lastSyncedOrder.ExecutedAt.Unix() > order.GetExecutedAt() {
			l.Info("syncing history finished")
			finished = true
			break
		}

		hist, err := conv.HistoryOrderToHistoryEntity(&order)
		if err != nil {
			return false, err
		}

		toUpdate = append(toUpdate, hist)
		toClear = append(toClear, hist.ExecutedAt)
	}

	if len(toUpdate) == 0 {
		l.Info("history orders were found but already processed")

		return
	}

	err = h.storage.SaveMarketHistoryOrders(marketId, toUpdate, toClear)
	l.Info("successfully synced history list")

	return
}
