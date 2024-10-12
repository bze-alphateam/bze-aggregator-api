package internal

import (
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/sirupsen/logrus"
)

func NewLogger(config *config.AppConfig) (logrus.FieldLogger, error) {
	logger := logrus.New()

	parsedLogLevel, err := logrus.ParseLevel(config.Logging.Level)
	if err != nil {
		logger.Fatal("error on parsing logging level: %s", err)
	}

	logger.SetLevel(parsedLogLevel)

	return logger, nil
}
