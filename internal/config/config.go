package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the traffic generator.
type Config struct {
	BaseURL              string
	APIKey               string
	TargetTPS            float64
	MaxRPS               float64
	ModelRefreshInterval time.Duration
	RequestTimeout       time.Duration
	StatsInterval        time.Duration
}

// Default returns a Config with sensible default values.
func Default() Config {
	return Config{
		BaseURL:              "https://inference.darkbloom.ai",
		TargetTPS:            500,
		MaxRPS:               4.5,
		ModelRefreshInterval: 5 * time.Minute,
		RequestTimeout:       120 * time.Second,
		StatsInterval:        10 * time.Second,
	}
}

// FromEnv overrides config fields from environment variables:
//   - DARKBLOOM_BASE_URL
//   - DARKBLOOM_API_KEY
//   - DARKBLOOM_TARGET_TPS
//   - DARKBLOOM_MAX_RPS
func (c *Config) FromEnv() {
	if v := os.Getenv("DARKBLOOM_BASE_URL"); v != "" {
		c.BaseURL = v
	}
	if v := os.Getenv("DARKBLOOM_API_KEY"); v != "" {
		c.APIKey = v
	}
	if v := os.Getenv("DARKBLOOM_TARGET_TPS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.TargetTPS = f
		}
	}
	if v := os.Getenv("DARKBLOOM_MAX_RPS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.MaxRPS = f
		}
	}
}

// Validate checks that the Config has all required fields set to valid values.
// It returns an error describing the first validation failure encountered.
func (c Config) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("BaseURL must not be empty")
	}
	if c.APIKey == "" {
		return fmt.Errorf("APIKey must not be empty")
	}
	if c.TargetTPS <= 0 {
		return fmt.Errorf("TargetTPS must be positive, got %f", c.TargetTPS)
	}
	if c.MaxRPS <= 0 || c.MaxRPS > 10 {
		return fmt.Errorf("MaxRPS must be in (0, 10], got %f", c.MaxRPS)
	}
	return nil
}
