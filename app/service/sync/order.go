package sync

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
)

type orderDataProvider interface {
	GetActiveBuyOrders(marketId string) ([]types.AggregatedOrder, error)
	GetActiveSellOrders(marketId string) ([]types.AggregatedOrder, error)
}

type orderStorage interface {
	Upsert(list []*entity.MarketOrder, marketIds []string) error
}

type Order struct {
	logger logrus.FieldLogger

	dataProvider  orderDataProvider
	storage       orderStorage
	assetProvider assetProvider
}

func NewOrderSync(logger logrus.FieldLogger, dataProvider orderDataProvider, storage orderStorage, assetProvider assetProvider) (*Order, error) {
	if logger == nil || dataProvider == nil || storage == nil || assetProvider == nil {
		return nil, internal.NewInvalidDependenciesErr("NewOrderSync")
	}

	return &Order{
		logger:        logger,
		dataProvider:  dataProvider,
		storage:       storage,
		assetProvider: assetProvider,
	}, nil
}

func (o *Order) SyncMarket(market *types.Market) error {
	mId := converter.GetMarketId(market.GetBase(), market.GetQuote())

	buys, err := o.dataProvider.GetActiveBuyOrders(mId)
	if err != nil {
		return err
	}

	sells, err := o.dataProvider.GetActiveSellOrders(mId)
	if err != nil {
		o.logger.WithError(err).Error("error getting syncing sell orders")
		return err
	}

	list := append(buys, sells...)
	err = o.syncList(list, market)
	if err != nil {
		o.logger.WithError(err).Error("error syncing sell orders")
		return err
	}

	return nil
}

func (o *Order) syncList(source []types.AggregatedOrder, market *types.Market) error {
	if len(source) == 0 {
		o.logger.Info("no active orders found")

		return nil
	}

	conv, err := converter.NewTypesConverter(o.assetProvider, market)
	if err != nil {
		return err
	}

	entities := o.convertAggregatedOrder(source, conv)
	if len(entities) == 0 {
		o.logger.Info("no converter orders found")

		return nil
	}

	return o.storage.Upsert(entities, []string{converter.GetMarketId(market.GetBase(), market.GetQuote())})
}

func (o *Order) convertAggregatedOrder(source []types.AggregatedOrder, conv *converter.TypesConverter) (entities []*entity.MarketOrder) {
	for _, order := range source {
		e, err := conv.AggregatedOrderToOrderEntity(&order)
		if err != nil {
			o.logger.WithError(err).Error("error converting order proto to entity")
			continue
		}

		entities = append(entities, e)
	}

	return entities
}
