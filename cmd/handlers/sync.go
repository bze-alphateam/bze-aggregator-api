package handlers

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
)

func getMarkets(provider marketProvider, logger logrus.FieldLogger) []types.Market {
	res, err := provider.GetAllMarkets()
	if err != nil {
		logger.WithError(err).Error("could not get markets")
		return nil
	}

	return res
}

func syncMarket(marketId string, provider marketProvider, logger logrus.FieldLogger, syncFunc func(m *types.Market) error) error {
	all := getMarkets(provider, logger)
	if len(all) == 0 {
		return fmt.Errorf("no markets found")
	}

	for _, m := range all {
		mId := converter.GetMarketId(m.GetBase(), m.GetQuote())
		if mId == marketId {
			return syncFunc(&m)
		}
	}

	return fmt.Errorf("market %s not found", marketId)
}

func syncAll(provider marketProvider, logger logrus.FieldLogger, syncFunc func(m *types.Market) error) {
	res := getMarkets(provider, logger)
	if len(res) == 0 {
		logger.Error("could not fetch markets")
		return
	}

	for _, m := range res {
		mId := converter.GetMarketId(m.GetBase(), m.GetQuote())
		l := logger.WithField("market_id", mId)

		err := syncFunc(&m)
		if err != nil {
			l.WithError(err).Error("could not sync market orders")
			continue
		}

		l.Info("market orders synced")
	}
}
