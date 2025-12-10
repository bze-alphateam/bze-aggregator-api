package handlers

import (
	"strings"

	"github.com/bze-alphateam/bze-aggregator-api/app/service/client"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/listener"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	types2 "github.com/cometbft/cometbft/abci/types"
	"github.com/sirupsen/logrus"
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
	lp        liquidityPoolStorage
	mProvider marketProvider
	locker    locker

	markets map[string]types.Market
}

func NewListener(logger logrus.FieldLogger, h historyStorage, i intervalStorage, o orderStorage, m marketStorage, lp liquidityPoolStorage, mProvider marketProvider, locker locker) (*Listener, error) {
	if logger == nil || h == nil || i == nil || o == nil || m == nil || lp == nil || mProvider == nil || locker == nil {
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
		lp:        lp,
		mProvider: mProvider,
		locker:    locker,
		markets:   markets,
	}, nil
}

func (l *Listener) ListenAndSync() error {
	defer l.logger.Info("ListenAndSync stopped")

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

	err = l.initialSync()
	if err != nil {
		l.logger.WithError(err).Error("error during initial sync")
		return err
	}

	msgChan := make(chan types2.Event)
	go func() {
		err := blockchain.Listen(msgChan)
		if err != nil {
			l.logger.WithError(err).Error("error listening for messages")
			close(msgChan)
		}
	}()

	for msg := range msgChan {
		go l.handleMessage(msg)
	}

	return nil
}

func (l *Listener) handleMessage(event types2.Event) {
	eventLogger := l.logger.WithField("event", event.Type)
	m := l.getEventMarket(event)

	switch event.Type {
	case "bze.tradebin.MarketCreatedEvent":
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
	case "bze.tradebin.OrderExecutedEvent":
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
	case "bze.tradebin.OrderCanceledEvent":
		fallthrough
	case "bze.tradebin.OrderSavedEvent":
		eventLogger.Info("syncing orders")
		if m == nil {
			eventLogger.Error("could not find market for this event")
			break
		}
		err := l.o.SyncMarket(m)
		if err != nil {
			eventLogger.WithError(err).Error("error syncing orders")
		}
	case "bze.tradebin.PoolCreatedEvent":
		eventLogger.Info("syncing liquidity pools after pool creation")
		err := l.lp.SyncLiquidityPools()
		if err != nil {
			eventLogger.WithError(err).Error("error syncing liquidity pools")
		}

		//refresh our markets list that we keep in memory
		l.lockMarkets()
		defer l.unlockMarkets()
		l.markets, err = getMarketsMap(l.mProvider)
		if err != nil {
			eventLogger.WithError(err).Error("error when trying to resync all markets")
		}
	case "bze.tradebin.LiquidityAddedEvent":
		fallthrough
	case "bze.tradebin.LiquidityRemovedEvent":
		poolId := l.getEventPoolId(event)
		if poolId == "" {
			eventLogger.Error("could not find pool_id for this event")
			break
		}

		eventLogger.Infof("syncing liquidity pool %s", poolId)
		err := l.lp.SyncLiquidityPoolById(poolId)
		if err != nil {
			eventLogger.WithError(err).Errorf("error syncing liquidity pool %s", poolId)
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

func (l *Listener) getEventPoolId(event types2.Event) string {
	for _, attr := range event.Attributes {
		if string(attr.Key) == "pool_id" {
			return strings.Trim(string(attr.Value), "\"")
		}
	}

	return ""
}

func (l *Listener) initialSync() (err error) {
	logger := l.logger.WithField("process", "initialSync")
	l.lockMarkets()
	defer l.unlockMarkets()
	logger.Info("syncing markets")
	err = l.m.SyncMarkets()

	logger.Info("syncing liquidity pools")
	err = l.lp.SyncLiquidityPools()
	if err != nil {
		logger.WithError(err).Error("error syncing liquidity pools")
	}

	l.markets, err = getMarketsMap(l.mProvider)
	if err != nil {
		return err
	}

	for _, m := range l.markets {
		logger = logger.WithField("market", converter.GetMarketId(m.GetBase(), m.GetQuote()))
		logger.Info("syncing history")
		err = l.h.SyncHistory(&m, 0)
		if err != nil {
			logger.WithError(err).Error("error syncing history")
			continue
		}

		logger.Info("syncing orders")
		err = l.o.SyncMarket(&m)
		if err != nil {
			logger.WithError(err).Error("error syncing orders")
			continue
		}

		logger.Info("syncing intervals")
		err = l.i.SyncIntervals(&m)
		if err != nil {
			logger.WithError(err).Error("error syncing intervals")
			continue
		}

		logger.Info("market synced")
	}

	l.logger.Info("initial sync finished")
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
