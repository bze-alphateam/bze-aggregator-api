package request

import "github.com/labstack/echo/v4"

type TickersParams struct {
	Format string `query:"format"`
}

func NewTickersParams(ctx echo.Context) (*TickersParams, error) {
	tp := &TickersParams{}
	if err := ctx.Bind(tp); err != nil {
		return nil, err
	}

	setAllowedFormat(tp)

	return tp, nil
}

func (p *TickersParams) IsCoingeckoFormat() bool {
	return p.Format == formatCoingecko
}

func (p *TickersParams) SetFormat(format string) {
	p.Format = format
}

func (p *TickersParams) GetFormat() string {
	return p.Format
}
