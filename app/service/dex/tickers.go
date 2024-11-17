package dex

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/response"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/calculator"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
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
		logger: logger,
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
		bid := sdk.MustNewDecFromStr(buy.Price)
		ticker.SetBid(bid.MustFloat64())
	}

	sell, err := t.oRepo.GetLowestSell(market.MarketID)
	if err != nil {
		return err
	}
	if sell != nil {
		ask := sdk.MustNewDecFromStr(sell.Price)
		ticker.SetAsk(ask.MustFloat64())
	}

	intervals, err := t.iRepo.GetIntervalsByExecutedAt(market.MarketID, time.Now().Add(-time.Hour*tickersHours), intervalLength)
	if err != nil {
		return err
	}

	openPrice := sdk.ZeroDec()
	if len(intervals) > 0 {
		op := sdk.MustNewDecFromStr(intervals[0].OpenPrice)
		openPrice = openPrice.Add(op)
		ticker.SetOpenPrice(openPrice.MustFloat64())
	}

	high := sdk.ZeroDec()
	low := sdk.ZeroDec()
	bVolume := sdk.ZeroDec()
	qVolume := sdk.ZeroDec()

	for _, i := range intervals {
		base := sdk.MustNewDecFromStr(i.BaseVolume)
		quote := sdk.MustNewDecFromStr(i.QuoteVolume)
		bVolume = bVolume.Add(base)
		qVolume = qVolume.Add(quote)

		iHigh := sdk.MustNewDecFromStr(i.HighestPrice)
		iLow := sdk.MustNewDecFromStr(i.LowestPrice)
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

	priceChange := sdk.ZeroDec()
	if market.LastPrice.Valid {
		lastPrice := sdk.MustNewDecFromStr(market.LastPrice.String)
		ticker.SetLastPrice(lastPrice.MustFloat64())
		priceChange = calculator.CalculatePriceChange(openPrice, lastPrice)
	}

	ticker.SetChange(converter.DecToFloat32Rounded(priceChange))

	return nil
}
