package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Spreadsheet SpreadsheetConfig `yaml:"spreadsheet"`
	Banks       []BankConfig      `yaml:"banks"`
}

type SpreadsheetConfig struct {
	URL        string `yaml:"url"`
	SheetName  string `yaml:"sheet_name"`
	ErrorSheet string `yaml:"error_sheet,omitempty"`
}

type BankConfig struct {
	Name     string                 `yaml:"name"`
	Senders  []string               `yaml:"senders"`
	Patterns []MessagePatternConfig `yaml:"patterns"`
}

type MessagePatternConfig struct {
	Name      string `yaml:"name"`
	Direction string `yaml:"direction"`
	Pattern   string `yaml:"pattern"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config.yml: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config.yml: %w", err)
	}
	return cfg, nil
}
