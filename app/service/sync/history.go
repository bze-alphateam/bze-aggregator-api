package sync

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	requestedHistoryLength = 5000
)

type historyProvider interface {
	GetMarketHistory(marketId string, limit uint64, key string) (list []types.HistoryOrder, paginationKey string, err error)
}

type historyStorage interface {
	GetLastHistoryOrder(marketId string) (*entity.MarketHistory, error)
	SaveMarketHistoryOrders(marketId string, orders []*entity.MarketHistory, clearExecutedAt []time.Time) error
}

type History struct {
	logger logrus.FieldLogger

	dataProvider historyProvider
	storage      historyStorage
}

func NewHistorySync(logger logrus.FieldLogger, dataProvider historyProvider, storage historyStorage) (*History, error) {
	if logger == nil || dataProvider == nil || storage == nil {
		return nil, internal.NewInvalidDependenciesErr("NewHistorySync")
	}

	return &History{logger: logger, dataProvider: dataProvider, storage: storage}, nil
}

func (h *History) SyncHistory(market *types.Market) error {
	marketId := converter.GetMarketId(market.GetBase(), market.GetQuote())
	l := h.logger.WithField("market", marketId)
	l.Info("preparing to sync history")

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
		hist, next, err := h.dataProvider.GetMarketHistory(marketId, requestedHistoryLength, key)
		if err != nil {
			return err
		}

		if len(hist) == 0 {
			l.WithField("key", key).WithField("limit", requestedHistoryLength).Info("no history found on the blockchain")
			break
		}

		done, err := h.syncHistoryList(marketId, hist, last)
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

func (h *History) syncHistoryList(marketId string, list []types.HistoryOrder, lastSyncedOrder *entity.MarketHistory) (finished bool, err error) {
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

		hist, err := converter.NewMarketHistoryEntity(&order)
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
