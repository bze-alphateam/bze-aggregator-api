package factory

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/controller"
	"github.com/sirupsen/logrus"
)

type ControllerFactory struct {
	logger logrus.FieldLogger
}

func NewControllerFactory(logger logrus.FieldLogger) (*ControllerFactory, error) {
	if logger == nil {
		return nil, fmt.Errorf("could not instantiate controller factory: invalid dependencies")
	}

	return &ControllerFactory{
		logger: logger,
	}, nil
}

func (c *ControllerFactory) GetSupplyController() (*controller.SupplyController, error) {

	return controller.NewSupplyController(c.logger, nil)
}
