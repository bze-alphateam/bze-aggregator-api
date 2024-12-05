package request

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
	"github.com/labstack/echo/v4"
)

const (
	defaultDenom = chain_registry.DenomUbze
)

type SupplyParams struct {
	Denom string `query:"denom"`
}

func NewSupplyParams(ctx echo.Context) (*SupplyParams, error) {
	mhr := &SupplyParams{}
	if err := ctx.Bind(mhr); err != nil {
		return nil, err
	}

	if mhr.Denom == "" {
		mhr.Denom = defaultDenom
	}

	return mhr, nil
}
