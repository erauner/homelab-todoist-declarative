package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const EnvToken = "TODOIST_API_TOKEN"

// ConfigFilePath is the fallback token file used when TODOIST_API_TOKEN is not set.
// Convention:
//   ~/.config/todoist/config.json
// with JSON payload:
//   {"token": "..."}
func ConfigFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "todoist", "config.json"), nil
}

type fileConfig struct {
	Token string `json:"token"`
}

type TokenSource string

const (
	SourceEnv  TokenSource = "env"
	SourceFile TokenSource = "file"
)

// DiscoverToken returns the Todoist API token and its discovery source.
// The token must be treated as a secret and never logged or printed.
func DiscoverToken() (string, TokenSource, error) {
	if t := os.Getenv(EnvToken); t != "" {
		return t, SourceEnv, nil
	}

	p, err := ConfigFilePath()
	if err != nil {
		return "", "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", fmt.Errorf("%s not set and token file not found at %s", EnvToken, p)
		}
		return "", "", fmt.Errorf("read token file %s: %w", p, err)
	}
	var cfg fileConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return "", "", fmt.Errorf("parse token file %s: %w", p, err)
	}
	if cfg.Token == "" {
		return "", "", fmt.Errorf("token file %s missing required key \"token\"", p)
	}
	return cfg.Token, SourceFile, nil
}
