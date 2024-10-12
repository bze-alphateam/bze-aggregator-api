package handlers

import (
	"github.com/sirupsen/logrus"
)

type orderStorage interface {
	//SaveMarkets(market []tradebinTypes.Market) error
}

type MarketOrderSync struct {
	grpc    grpc
	storage orderStorage
	logger  logrus.FieldLogger
}
