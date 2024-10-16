package sync

import (
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
)

type historyProvider interface {
	GetMarketHistory(marketId string, limit uint64, key string) ([]types.HistoryOrder, error)
}

type History struct {
	logger logrus.FieldLogger

	dataProvider historyProvider
	storage      orderStorage
}

func NewHistorySync(logger logrus.FieldLogger, dataProvider historyProvider, storage orderStorage) (*History, error) {
	if logger == nil || dataProvider == nil || storage == nil {
		return nil, internal.NewInvalidDependenciesErr("NewOrderSync")
	}

	return &History{logger: logger, dataProvider: dataProvider, storage: storage}, nil
}

func (o *History) SyncHistory(market *types.Market) error {

	return nil
}
