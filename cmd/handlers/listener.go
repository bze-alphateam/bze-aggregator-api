package handlers

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/service/client"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/listener"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
	types2 "github.com/tendermint/tendermint/abci/types"
	"strings"
)

const (
	historyBatchSize = 150
	lockMarketsKey   = "sync:listener:lock:markets"
)

type locker interface {
	Lock(key string)
	Unlock(key string)
}

type Listener struct {
	logger    logrus.FieldLogger
	h         historyStorage
	i         intervalStorage
	o         orderStorage
	m         marketStorage
	mProvider marketProvider
	locker    locker

	markets map[string]types.Market
}

func NewListener(logger logrus.FieldLogger, h historyStorage, i intervalStorage, o orderStorage, m marketStorage, mProvider marketProvider, locker locker) (*Listener, error) {
	if logger == nil || h == nil || i == nil || o == nil || m == nil || mProvider == nil || locker == nil {
		return nil, internal.NewInvalidDependenciesErr("NewListener")
	}

	err := m.SyncMarkets()
	if err != nil {
		return nil, err
	}

	markets, err := getMarketsMap(mProvider)
	if err != nil {
		return nil, err
	}

	return &Listener{
		logger:    logger,
		h:         h,
		i:         i,
		o:         o,
		m:         m,
		mProvider: mProvider,
		locker:    locker,
		markets:   markets,
	}, nil
}

func (l *Listener) ListenAndSync() error {
	conn, err := client.GetWsClient()
	if err != nil {
		return err
	}
	l.logger.Debug("connection established")

	blockchain, err := listener.NewTradebinListener(conn, l.logger)
	if err != nil {
		return err
	}
	l.logger.Debug("created blockchain listener")

	msgChan := make(chan types2.Event)

	go func() {
		err := blockchain.Listen(msgChan)
		if err != nil {
			l.logger.WithError(err).Error("error listening for messages")
		}
	}()

	for msg := range msgChan {
		l.handleMessage(msg)
	}

	return nil
}

func (l *Listener) handleMessage(event types2.Event) {
	eventLogger := l.logger.WithField("event", event.Type)
	m := l.getEventMarket(event)

	switch event.Type {
	case "bze.tradebin.v1.MarketCreatedEvent":
		eventLogger.Info("syncing markets")
		err := l.m.SyncMarkets()
		if err != nil {
			eventLogger.WithError(err).Error("error syncing markets")
		}

		//when a new market is created we should refresh our markets list that we keep in memory
		l.lockMarkets()
		defer l.unlockMarkets()
		l.markets, err = getMarketsMap(l.mProvider)
		if err != nil {
			eventLogger.WithError(err).Error("error when trying to resync all markets")
		}
	case "bze.tradebin.v1.OrderExecutedEvent":
		eventLogger.Info("syncing history")
		if m == nil {
			eventLogger.Error("could not find market for this event")
			break
		}
		err := l.h.SyncHistory(m, historyBatchSize)
		if err != nil {
			eventLogger.WithError(err).Error("error syncing history")
		}

		err = l.i.SyncIntervals(m)
		if err != nil {
			eventLogger.WithError(err).Error("error syncing intervals")
		}

		fallthrough
	case "bze.tradebin.v1.OrderCanceledEvent":
		fallthrough
	case "bze.tradebin.v1.OrderSavedEvent":
		eventLogger.Info("syncing orders")
		if m == nil {
			eventLogger.Error("could not find market for this event")
			break
		}
		err := l.o.SyncMarket(m)
		if err != nil {
			eventLogger.WithError(err).Error("error syncing orders")
		}
	}

	eventLogger.Debug("message handled")
}

func (l *Listener) lockMarkets() {
	l.locker.Lock(lockMarketsKey)
}

func (l *Listener) unlockMarkets() {
	l.locker.Unlock(lockMarketsKey)
}

func (l *Listener) getEventMarket(event types2.Event) *types.Market {
	for _, attr := range event.Attributes {
		if string(attr.Key) == "market_id" {
			mId := strings.Trim(string(attr.Value), "\"")
			m, ok := l.markets[mId]
			if ok {
				return &m
			}

			return nil
		}
	}

	return nil
}

func getMarketsMap(mProvider marketProvider) (map[string]types.Market, error) {
	mTypes, err := mProvider.GetAllMarkets()
	if err != nil {
		return nil, err
	}

	markets := make(map[string]types.Market, len(mTypes))
	for _, mType := range mTypes {
		markets[converter.GetMarketId(mType.GetBase(), mType.GetQuote())] = mType
	}

	return markets, nil
}
