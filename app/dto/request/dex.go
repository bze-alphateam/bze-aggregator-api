package request

import "github.com/labstack/echo/v4"

const (
	formatCoingecko = "coingecko"
)

type TickersParams struct {
	Format string `query:"format"`
}

func NewTickersParams(ctx echo.Context) (*TickersParams, error) {
	mhr := &TickersParams{}
	if err := ctx.Bind(mhr); err != nil {
		return nil, err
	}

	//allow only known formats
	// if we don't know the format remove the param(ignore it)
	if mhr.Format != formatCoingecko {
		mhr.Format = ""
	}

	return mhr, nil
}

func (p *TickersParams) IsCoingeckoFormat() bool {
	return p.Format == formatCoingecko
}
