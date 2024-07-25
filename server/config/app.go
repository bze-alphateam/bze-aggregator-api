package config

import (
	"github.com/joho/godotenv"
)

const (
	defaultPort         = "8000"
	defaultLoggingLevel = "info"
)

type Logging struct {
	Level string
}

type Server struct {
	Port string
}

type AppConfig struct {
	Server  Server
	Logging Logging
}

func NewAppConfig() *AppConfig {

	return loadDefaultConfig()
}

func loadDefaultConfig() *AppConfig {
	envFile, err := godotenv.Read(".env")
	port := defaultPort
	logLevel := defaultLoggingLevel
	if err != nil {
		return &AppConfig{
			Server: Server{
				Port: port,
			},
			Logging: Logging{
				Level: logLevel,
			},
		}
	}

	port, ok := envFile["HTTP_PORT"]
	if !ok {
		port = defaultPort
	}

	logLevel, ok = envFile["LOG_LEVEL"]
	if !ok {
		logLevel = defaultLoggingLevel
	}

	return &AppConfig{
		Server: Server{
			Port: port,
		},
		Logging: Logging{
			Level: logLevel,
		},
	}
}
