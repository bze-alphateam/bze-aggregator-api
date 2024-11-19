package sync

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	tradebinTypes "github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
)

type marketProvider interface {
	GetAllMarkets() ([]tradebinTypes.Market, error)
}

type marketRepo interface {
	SaveIfNotExists(items []*entity.Market) error
}

type Market struct {
	storage  marketRepo
	logger   logrus.FieldLogger
	provider marketProvider
}

func NewMarketSync(logger logrus.FieldLogger, storage marketRepo, provider marketProvider) (*Market, error) {
	if storage == nil || logger == nil || provider == nil {
		return nil, internal.NewInvalidDependenciesErr("NewMarketSync")
	}

	return &Market{
		storage:  storage,
		logger:   logger.WithField("service", "MarketSync"),
		provider: provider,
	}, nil
}

func (m *Market) SyncMarkets() error {
	list, err := m.provider.GetAllMarkets()
	if err != nil {
		return err
	}

	m.logger.Infof("saving %d markets", len(list))

	var entities []*entity.Market
	for _, source := range list {
		target := converter.NewMarketEntity(&source)
		entities = append(entities, target)
	}

	err = m.storage.SaveIfNotExists(entities)
	if err != nil {
		return err
	}

	return nil
}
