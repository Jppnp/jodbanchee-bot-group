// Package config reads settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// Config holds runtime settings. See .env.example for descriptions.
type Config struct {
	ChannelSecret      string
	ChannelAccessToken string
	Port               string
	DatabaseURL        string // when set, Postgres is used instead of DataFile
	DataFile           string
	Location           *time.Location
	LineAPIBase        string
}

// Load reads the configuration from the environment.
func Load() (Config, error) {
	c := Config{
		ChannelSecret:      os.Getenv("LINE_CHANNEL_SECRET"),
		ChannelAccessToken: os.Getenv("LINE_CHANNEL_ACCESS_TOKEN"),
		Port:               getenv("PORT", "8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		DataFile:           getenv("DATA_FILE", "data/ledger.json"),
		LineAPIBase:        getenv("LINE_API_BASE", ""),
	}

	var errs []error
	if c.ChannelSecret == "" {
		errs = append(errs, errors.New("LINE_CHANNEL_SECRET is required"))
	}
	if c.ChannelAccessToken == "" {
		errs = append(errs, errors.New("LINE_CHANNEL_ACCESS_TOKEN is required"))
	}

	tz := getenv("TIMEZONE", "Asia/Bangkok")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		errs = append(errs, fmt.Errorf("TIMEZONE %q: %w", tz, err))
	}
	c.Location = loc

	return c, errors.Join(errs...)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
