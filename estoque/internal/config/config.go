package config

import (
	"errors"
	"os"
)

type Config struct {
	PortaHTTP string
	UrlBanco  string
}

func Carregar() (*Config, error) {
	URL := os.Getenv("DATABASE_URL")
	if len(URL) == 0 {
		return nil, errors.New("DATABASE_URL não definida")
	}
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "7080"
	}

	return &Config{PortaHTTP: port, UrlBanco: URL}, nil
}
