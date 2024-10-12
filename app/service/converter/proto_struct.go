package converter

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	tradebinTypes "github.com/bze-alphateam/bze/x/tradebin/types"
)

func NewMarketEntity(source *tradebinTypes.Market) *entity.Market {
	return &entity.Market{
		MarketID:  fmt.Sprintf("%s/%s", source.GetBase(), source.GetQuote()),
		Base:      source.GetBase(),
		Quote:     source.GetQuote(),
		CreatedBy: source.GetCreator(),
	}
}
