package sync

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	tradebinTypes "github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
)

type marketRepo interface {
	SaveIfNotExists(items []*entity.Market) error
}

type Market struct {
	storage marketRepo
	logger  logrus.FieldLogger
}

func NewMarketSync(logger logrus.FieldLogger, storage marketRepo) (*Market, error) {
	if storage == nil || logger == nil {
		return nil, internal.NewInvalidDependenciesErr("NewMarketSync")
	}

	return &Market{
		storage: storage,
		logger:  logger,
	}, nil
}

func (m *Market) SaveMarkets(list []tradebinTypes.Market) error {
	m.logger.Infof("saving %d markets", len(list))

	var entities []*entity.Market
	for _, source := range list {
		target := converter.NewMarketEntity(&source)
		entities = append(entities, target)
	}

	err := m.storage.SaveIfNotExists(entities)
	if err != nil {
		return err
	}

	return nil
}
