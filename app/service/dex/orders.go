package dex

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/response"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type marketOrdersResult interface {
	AddBid(price, volume string)
	AddAsk(price, volume string)
	SetTime(t time.Time)
}

type ordersRepo interface {
	GetMarketOrdersWithDepth(marketId, orderType string, limit int) ([]entity.MarketOrder, error)
}

type ordersMarketRepo interface {
	GetMarket(marketId string) (*entity.Market, error)
}

type OrdersService struct {
	logger   logrus.FieldLogger
	oRepo    ordersRepo
	mRepo    ordersMarketRepo
	cgMerges CoingeckoMerges
}

func NewOrdersService(logger logrus.FieldLogger, oRepo ordersRepo, mRepo ordersMarketRepo, cgMerges CoingeckoMerges) (*OrdersService, error) {
	if logger == nil || oRepo == nil || mRepo == nil {
		return nil, internal.NewInvalidDependenciesErr("NewOrdersService")
	}

	return &OrdersService{
		logger:   logger.WithField("service", "Dex.OrdersService"),
		oRepo:    oRepo,
		mRepo:    mRepo,
		cgMerges: cgMerges,
	}, nil
}

func (o *OrdersService) GetMarketOrders(marketId string, depth int) (*response.Orders, error) {
	market, err := o.mRepo.GetMarket(marketId)
	if err != nil {
		return nil, err
	}

	if market == nil {
		return nil, fmt.Errorf("market not found")
	}

	buys, sells, err := o.getMarketOrders(marketId, depth)
	if err != nil {
		return nil, err
	}

	res := &response.Orders{
		MarketId:  marketId,
		Timestamp: fmt.Sprintf("%d", time.Now().Unix()),
	}

	o.hydrateResponse(res, buys, sells)

	return res, nil
}

// GetCoingeckoMarketOrders returns a market's book under its ticker id. For a
// market other markets are folded into (see CoingeckoMerges), it is the book of
// the first folded market that has orders, so the depth matches the reported
// volume; the market's own book only when none has any. Books are not mixed,
// a mixed one could cross.
func (o *OrdersService) GetCoingeckoMarketOrders(marketId string, depth int) (*response.CoingeckoOrders, error) {
	market, err := o.mRepo.GetMarket(marketId)
	if err != nil {
		return nil, err
	}

	if market == nil {
		return nil, fmt.Errorf("market not found")
	}

	var buys, sells []entity.MarketOrder
	for _, folded := range o.cgMerges.foldedInto(marketId) {
		buys, sells, err = o.getMarketOrders(folded, depth)
		if err != nil {
			return nil, err
		}

		if len(buys) > 0 || len(sells) > 0 {
			break
		}
	}

	if len(buys) == 0 && len(sells) == 0 {
		buys, sells, err = o.getMarketOrders(marketId, depth)
		if err != nil {
			return nil, err
		}
	}

	res := &response.CoingeckoOrders{
		TickerId:  fmt.Sprintf("%s_%s", market.Base, market.Quote),
		Timestamp: fmt.Sprintf("%d", time.Now().Unix()),
	}

	o.hydrateResponse(res, buys, sells)

	return res, nil
}

func (o *OrdersService) hydrateResponse(res marketOrdersResult, buys, sells []entity.MarketOrder) {
	newestTime := time.Now()
	for _, buy := range buys {
		if buy.CreatedAt.After(newestTime) {
			newestTime = buy.CreatedAt
		}
		res.AddBid(buy.Price, buy.Amount)
	}

	for _, sell := range sells {
		if sell.CreatedAt.After(newestTime) {
			newestTime = sell.CreatedAt
		}
		res.AddAsk(sell.Price, sell.Amount)
	}

}

func (o *OrdersService) getMarketOrders(marketId string, depth int) (buys, sells []entity.MarketOrder, err error) {
	limit := depth / 2

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		var er error
		buys, er = o.oRepo.GetMarketOrdersWithDepth(marketId, entity.OrderTypeBuy, limit)
		if er != nil {
			err = er
		}
	}()

	go func() {
		defer wg.Done()
		var er error
		sells, er = o.oRepo.GetMarketOrdersWithDepth(marketId, entity.OrderTypeSell, limit)
		if er != nil {
			err = er
		}
	}()

	wg.Wait()

	return
}
