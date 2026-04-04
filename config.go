package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	requiredBankPatternGroups = []string{"amount", "merchant", "account", "balance"}
	currencyBankPatternGroups = []string{"currency", "balance_currency"}
)

type Config struct {
	Spreadsheet SpreadsheetConfig `yaml:"spreadsheet"`
	Banks       []BankConfig      `yaml:"banks"`
	Timezone    string            `yaml:"timezone"`
}

type SpreadsheetConfig struct {
	URL       string `yaml:"url"`
	SheetName string `yaml:"sheet_name"`
}

type BankConfig struct {
	Name    string   `yaml:"name"`
	Senders []string `yaml:"senders"`
	Pattern string   `yaml:"pattern"`
}

func loadConfig(path string) (Config, error) {
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

func validateConfig(cfg Config) error {
	if cfg.Spreadsheet.SheetName == "" {
		return errors.New("spreadsheet.sheet_name is required in config.yml")
	}

	if len(cfg.Banks) == 0 {
		return errors.New("banks must include at least one entry")
	}

	return nil
}

func compileBankMatchers(banks []BankConfig) ([]bankMatcher, error) {
	matchers := make([]bankMatcher, 0, len(banks))
	for _, bank := range banks {
		name := strings.TrimSpace(bank.Name)
		if name == "" {
			return nil, errors.New("bank name cannot be empty")
		}

		senderMatchers := make([]*regexp.Regexp, 0, len(bank.Senders))
		for _, senderPattern := range bank.Senders {
			senderPattern = strings.TrimSpace(senderPattern)
			if senderPattern == "" {
				continue
			}
			senderRegex, err := regexp.Compile(senderPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid sender pattern for bank %q: %w", name, err)
			}
			senderMatchers = append(senderMatchers, senderRegex)
		}
		if len(senderMatchers) == 0 {
			return nil, fmt.Errorf("bank %q must include at least one sender pattern", name)
		}

		if strings.TrimSpace(bank.Pattern) == "" {
			return nil, errors.New("bank pattern cannot be empty")
		}
		re, err := regexp.Compile(bank.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid bank pattern %q: %w", name, err)
		}
		if err := validateBankPattern(re, name); err != nil {
			return nil, err
		}
		matchers = append(matchers, bankMatcher{
			name:         name,
			senderRegex:  senderMatchers,
			messageRegex: re,
		})
	}
	return matchers, nil
}

func validateBankPattern(re *regexp.Regexp, name string) error {
	groups := make(map[string]struct{})
	for _, group := range re.SubexpNames() {
		if group != "" {
			groups[group] = struct{}{}
		}
	}

	missing := make([]string, 0, len(requiredBankPatternGroups))
	for _, group := range requiredBankPatternGroups {
		if _, ok := groups[group]; !ok {
			missing = append(missing, group)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("bank pattern %q missing named groups: %s", name, strings.Join(missing, ", "))
	}

	for _, group := range currencyBankPatternGroups {
		if _, ok := groups[group]; ok {
			return nil
		}
	}

	return fmt.Errorf("bank pattern %q must include at least one named group: %s", name, strings.Join(currencyBankPatternGroups, " or "))
}
