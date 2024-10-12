package sync

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
	"sync"
)

type orderDataProvider interface {
	GetActiveBuyOrders(marketId string) ([]types.AggregatedOrder, error)
	GetActiveSellOrders(marketId string) ([]types.AggregatedOrder, error)
}

type orderStorage interface {
	Upsert(list []*entity.MarketOrder) error
}

type Order struct {
	logger logrus.FieldLogger

	dataProvider orderDataProvider
	storage      orderStorage
}

func NewOrderSync(logger logrus.FieldLogger, dataProvider orderDataProvider, storage orderStorage) (*Order, error) {
	if logger == nil || dataProvider == nil || storage == nil {
		return nil, internal.NewInvalidDependenciesErr("NewOrderSync")
	}

	return &Order{logger: logger, dataProvider: dataProvider, storage: storage}, nil
}

func (o *Order) SyncMarket(market *types.Market) error {
	mId := converter.GetMarketId(market.GetBase(), market.GetQuote())
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()

		list, err := o.dataProvider.GetActiveBuyOrders(mId)
		if err != nil {
			o.logger.WithError(err).Error("error getting syncing buy orders")
			return
		}

		err = o.syncList(list)
		if err != nil {
			o.logger.WithError(err).Error("error syncing buy orders")
		}
	}()

	go func() {
		defer wg.Done()

		list, err := o.dataProvider.GetActiveSellOrders(mId)
		if err != nil {
			o.logger.WithError(err).Error("error getting syncing sell orders")
			return
		}

		err = o.syncList(list)
		if err != nil {
			o.logger.WithError(err).Error("error syncing sell orders")
		}
	}()

	wg.Wait()

	return nil
}

func (o *Order) syncList(source []types.AggregatedOrder) error {
	if len(source) == 0 {
		o.logger.Info("no active orders found")

		return nil
	}

	entities := o.convertAggregatedOrder(source)
	if len(entities) == 0 {
		o.logger.Info("no converter orders found")

		return nil
	}

	return o.storage.Upsert(entities)
}

func (o *Order) convertAggregatedOrder(source []types.AggregatedOrder) []*entity.MarketOrder {
	var entities []*entity.MarketOrder
	for _, order := range source {

		//TODO: calculate quote amount

		e, err := converter.NewMarketOrderEntity(&order)
		if err != nil {
			o.logger.WithError(err).Error("error converting order proto to entity")
			continue
		}

		entities = append(entities, e)
	}

	return entities
}
