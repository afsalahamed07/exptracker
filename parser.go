package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

func parsePayload(req events.APIGatewayV2HTTPRequest) (SMSPayload, error) {
	if strings.TrimSpace(req.Body) == "" {
		return SMSPayload{}, errors.New("empty body")
	}

	var payload SMSPayload
	if err := json.Unmarshal([]byte(req.Body), &payload); err != nil {
		return SMSPayload{}, errors.New("invalid json body")
	}

	payload.Sender = strings.TrimSpace(payload.Sender)
	payload.Message = strings.TrimSpace(payload.Message)
	payload.ReceivedAt = strings.TrimSpace(payload.ReceivedAt)
	payload.DeviceID = strings.TrimSpace(payload.DeviceID)

	if payload.Sender == "" || payload.Message == "" || payload.ReceivedAt == "" {
		return SMSPayload{}, errors.New("sender, message, and received_at are required")
	}

	parsedTime, err := time.Parse(time.RFC3339, payload.ReceivedAt)
	if err != nil {
		return SMSPayload{}, errors.New("received_at must be RFC3339")
	}

	payload.ReceivedAt = parsedTime.Format(time.RFC3339)

	return payload, nil
}

func (h *handler) matcherForSender(sender string) (bankMatcher, error) {
	sender = strings.TrimSpace(sender)
	if sender == "" {
		return bankMatcher{}, errors.New("sender not allowed")
	}

	var matched *bankMatcher
	for i := range h.bankMatchers {
		matcher := &h.bankMatchers[i]
		for _, senderRegex := range matcher.senderRegex {
			if !senderRegex.MatchString(sender) {
				continue
			}
			if matched != nil {
				return bankMatcher{}, fmt.Errorf("sender %q matched multiple bank configurations", sender)
			}
			matched = matcher
			break
		}
	}
	if matched == nil {
		return bankMatcher{}, errors.New("sender not allowed")
	}
	return *matched, nil
}

func (h *handler) parseSMS(payload SMSPayload, matcher bankMatcher) (ParsedTransaction, error) {
	for _, message := range matcher.messages {
		match := message.regex.FindStringSubmatch(payload.Message)
		if len(match) == 0 {
			continue
		}

		parts := mapSubexp(message.regex, match)
		amount, err := requiredField("amount", normalizeNumber(parts["amount"]))
		if err != nil {
			return ParsedTransaction{}, err
		}

		accountMask, err := requiredField("account", strings.TrimSpace(parts["account"]))
		if err != nil {
			return ParsedTransaction{}, err
		}

		description := normalizeWhitespace(parts["description"])
		if description == "" {
			description = normalizeWhitespace(parts["merchant"])
		}
		description, err = requiredField("description", description)
		if err != nil {
			return ParsedTransaction{}, err
		}

		return ParsedTransaction{
			Sender:      payload.Sender,
			ReceivedAt:  payload.ReceivedAt,
			DeviceID:    payload.DeviceID,
			Description: description,
			AccountMask: accountMask,
			Amount:      amount,
			Direction:   message.direction,
			BankName:    matcher.name,
		}, nil
	}

	return ParsedTransaction{}, fmt.Errorf("message did not match any pattern for %s", matcher.name)
}

func mapSubexp(re *regexp.Regexp, match []string) map[string]string {
	results := make(map[string]string)
	for i, name := range re.SubexpNames() {
		if i == 0 || name == "" {
			continue
		}
		results[name] = strings.TrimSpace(match[i])
	}
	return results
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func normalizeNumber(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), ",", "")
}

func requiredField(name, value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("%s not found", name)
	}
	return value, nil
}
