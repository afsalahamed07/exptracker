// Package parser provides functionality to compile bank matchers from configuration and validate message patterns.
package parser

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"sms-ingest/internal/config"
)

var requiredMessagePatternGroups = []string{"amount", "account"}

func CompileBankMatchers(banks []config.BankConfig) ([]BankMatcher, error) {
	matchers := make([]BankMatcher, 0, len(banks))
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

		matchers = append(matchers, BankMatcher{
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
