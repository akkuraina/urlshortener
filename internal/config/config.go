// Package config loads server settings from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime settings. The zero value is not usable; call Load.
type Config struct {
	Port int
}

// Load reads settings from the environment, falling back to defaults.
func Load() (Config, error) {
	cfg := Config{
		Port: 8080,
	}

	if v := os.Getenv("PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("config: invalid PORT %q: must be a number from 1 to 65535", v)
		}
		cfg.Port = port
	}

	return cfg, nil
}

// Addr returns the listen address in the form net/http expects, e.g. ":8080".
func (c Config) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}
