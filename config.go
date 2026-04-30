package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var requiredMessagePatternGroups = []string{"amount", "account"}

type Config struct {
	Spreadsheet SpreadsheetConfig `yaml:"spreadsheet"`
	Banks       []BankConfig      `yaml:"banks"`
	Timezone    string            `yaml:"timezone"`
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

		if len(bank.Patterns) == 0 {
			return nil, fmt.Errorf("bank %q must include at least one message pattern", name)
		}

		messageMatchers := make([]messageMatcher, 0, len(bank.Patterns))
		for _, pattern := range bank.Patterns {
			patternName := strings.TrimSpace(pattern.Name)
			if patternName == "" {
				return nil, fmt.Errorf("bank %q has a message pattern with empty name", name)
			}

			direction := strings.ToLower(strings.TrimSpace(pattern.Direction))
			if direction != "credit" && direction != "debit" {
				return nil, fmt.Errorf("bank %q pattern %q must use direction credit or debit", name, patternName)
			}

			patternValue := strings.TrimSpace(pattern.Pattern)
			if patternValue == "" {
				return nil, fmt.Errorf("bank %q pattern %q cannot be empty", name, patternName)
			}

			re, err := regexp.Compile(patternValue)
			if err != nil {
				return nil, fmt.Errorf("invalid bank pattern %q for bank %q: %w", patternName, name, err)
			}
			if err := validateMessagePattern(re, name, patternName); err != nil {
				return nil, err
			}

			messageMatchers = append(messageMatchers, messageMatcher{
				name:      patternName,
				direction: direction,
				regex:     re,
			})
		}

		matchers = append(matchers, bankMatcher{
			name:        name,
			senderRegex: senderMatchers,
			messages:    messageMatchers,
		})
	}
	return matchers, nil
}

func validateMessagePattern(re *regexp.Regexp, bankName, patternName string) error {
	groups := make(map[string]struct{})
	for _, group := range re.SubexpNames() {
		if group != "" {
			groups[group] = struct{}{}
		}
	}

	missing := make([]string, 0, len(requiredMessagePatternGroups))
	for _, group := range requiredMessagePatternGroups {
		if _, ok := groups[group]; !ok {
			missing = append(missing, group)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("bank %q pattern %q missing named groups: %s", bankName, patternName, strings.Join(missing, ", "))
	}

	if _, ok := groups["description"]; ok {
		return nil
	}
	if _, ok := groups["merchant"]; ok {
		return nil
	}

	return fmt.Errorf("bank %q pattern %q must include one named group: description or merchant", bankName, patternName)
}
