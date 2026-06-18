package api

import (
	"fmt"

	"go.uber.org/zap"
)

func NewLogger(environment string) (*zap.SugaredLogger, error) {
	var logger *zap.Logger
	var err error

	switch environment {
	case "development", "test":
		logger, err = zap.NewDevelopment()
	case "production":
		logger, err = zap.NewProduction()
	default:
		return nil, fmt.Errorf(
			"cannot create logger for environment %q",
			environment,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}

	return logger.Sugar(), nil
}
