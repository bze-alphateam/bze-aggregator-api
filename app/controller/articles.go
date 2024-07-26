package controller

import (
	"errors"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"net/http"
)

type ArticlesService interface {
	GetLatestArticles() []dto.Article
}

type ArticlesController struct {
	service ArticlesService
	logger  logrus.FieldLogger
}

func NewArticlesController(logger logrus.FieldLogger, service ArticlesService) (*ArticlesController, error) {
	if logger == nil || service == nil {
		return nil, errors.New("invalid dependencies provided to articles controller")
	}

	return &ArticlesController{service: service, logger: logger}, nil
}

func (c *ArticlesController) MediumArticlesHandler(ctx echo.Context) error {

	return ctx.JSON(http.StatusOK, c.service.GetLatestArticles())
}
