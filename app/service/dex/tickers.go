package dex

import (
	"sync"
	"time"

	"cosmossdk.io/math"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/response"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/calculator"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
)

const (
	intervalLength = 5 //minutes

	tickersHours = 24
)

type ticker interface {
	SetMarketDetails(base, quote, marketId string)
	SetLastPrice(price float64)
	SetBaseVolume(baseVolume float64)
	SetQuoteVolume(quoteVolume float64)
	SetBid(bid float64)
	SetAsk(ask float64)
	SetHigh(high float64)
	SetLow(low float64)
	SetChange(change float32)
	SetOpenPrice(price float64)
}

type marketRepo interface {
	GetMarketsWithLastExecuted(hours int) ([]entity.MarketWithLastPrice, error)
}

type intervalsRepo interface {
	GetIntervalsByExecutedAt(marketId string, executedAt time.Time, length int) ([]entity.MarketHistoryInterval, error)
}

type tickersOrdersRepo interface {
	GetHighestBuy(marketId string) (*entity.MarketOrder, error)
	GetLowestSell(marketId string) (*entity.MarketOrder, error)
}

type Tickers struct {
	logger logrus.FieldLogger
	mRepo  marketRepo
	iRepo  intervalsRepo
	oRepo  tickersOrdersRepo
}

func NewTickersService(logger logrus.FieldLogger, mRepo marketRepo, iRepo intervalsRepo, oRepo tickersOrdersRepo) (*Tickers, error) {
	if logger == nil || mRepo == nil || iRepo == nil || oRepo == nil {
		return nil, internal.NewInvalidDependenciesErr("NewTickersService")
	}

	return &Tickers{
		logger: logger.WithField("service", "Dex.TickersService"),
		mRepo:  mRepo,
		iRepo:  iRepo,
		oRepo:  oRepo,
	}, nil
}

func (t *Tickers) GetCoingeckoTickers() ([]*response.CoingeckoTicker, error) {
	markets, err := t.mRepo.GetMarketsWithLastExecuted(tickersHours)
	if err != nil {
		return nil, err
	}

	mux := &sync.Mutex{}
	wg := &sync.WaitGroup{}

	var gErr error
	var tickers []*response.CoingeckoTicker
	for _, market := range markets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ti := response.CoingeckoTicker{}
			err = t.buildTicker(market, &ti)
			if err != nil {
				gErr = err
			}

			mux.Lock()
			defer mux.Unlock()
			tickers = append(tickers, &ti)
		}()
	}

	wg.Wait()

	return tickers, gErr
}

func (t *Tickers) GetTickers() ([]*response.Ticker, error) {
	markets, err := t.mRepo.GetMarketsWithLastExecuted(tickersHours)
	if err != nil {
		return nil, err
	}

	mux := &sync.Mutex{}
	wg := &sync.WaitGroup{}

	var gErr error
	var tickers []*response.Ticker
	for _, market := range markets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ti := response.Ticker{}
			err = t.buildTicker(market, &ti)
			if err != nil {
				gErr = err
			}

			mux.Lock()
			defer mux.Unlock()
			tickers = append(tickers, &ti)
		}()
	}

	wg.Wait()

	return tickers, gErr
}

func (t *Tickers) buildTicker(market entity.MarketWithLastPrice, ticker ticker) error {
	ticker.SetMarketDetails(market.Base, market.Quote, market.MarketID)

	buy, err := t.oRepo.GetHighestBuy(market.MarketID)
	if err != nil {
		return err
	}
	if buy != nil {
		bid := math.LegacyMustNewDecFromStr(buy.Price)
		ticker.SetBid(bid.MustFloat64())
	}

	sell, err := t.oRepo.GetLowestSell(market.MarketID)
	if err != nil {
		return err
	}
	if sell != nil {
		ask := math.LegacyMustNewDecFromStr(sell.Price)
		ticker.SetAsk(ask.MustFloat64())
	}

	intervals, err := t.iRepo.GetIntervalsByExecutedAt(market.MarketID, time.Now().Add(-time.Hour*tickersHours), intervalLength)
	if err != nil {
		return err
	}

	openPrice := math.LegacyZeroDec()
	if len(intervals) > 0 {
		op := math.LegacyMustNewDecFromStr(intervals[0].OpenPrice)
		openPrice = openPrice.Add(op)
		ticker.SetOpenPrice(openPrice.MustFloat64())
	}

	high := math.LegacyZeroDec()
	low := math.LegacyZeroDec()
	bVolume := math.LegacyZeroDec()
	qVolume := math.LegacyZeroDec()

	for _, i := range intervals {
		base := math.LegacyMustNewDecFromStr(i.BaseVolume)
		quote := math.LegacyMustNewDecFromStr(i.QuoteVolume)
		bVolume = bVolume.Add(base)
		qVolume = qVolume.Add(quote)

		iHigh := math.LegacyMustNewDecFromStr(i.HighestPrice)
		iLow := math.LegacyMustNewDecFromStr(i.LowestPrice)
		if iHigh.GT(high) {
			high = iHigh
		}

		if iLow.LT(low) || low.IsZero() {
			low = iLow
		}
	}

	ticker.SetQuoteVolume(qVolume.MustFloat64())
	ticker.SetBaseVolume(bVolume.MustFloat64())
	ticker.SetHigh(high.MustFloat64())
	ticker.SetLow(low.MustFloat64())
	ticker.SetLastPrice(0)

	priceChange := math.LegacyZeroDec()
	if market.LastPrice.Valid {
		lastPrice := math.LegacyMustNewDecFromStr(market.LastPrice.String)
		ticker.SetLastPrice(lastPrice.MustFloat64())
		priceChange = calculator.CalculatePriceChange(openPrice, lastPrice)
	}

	ticker.SetChange(converter.DecToFloat32Rounded(priceChange))

	return nil
}
