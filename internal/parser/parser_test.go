package parser

import (
	"regexp"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"sms-ingest/internal/config"
)

func TestParsePayloadNormalizesAndValidatesTimestamp(t *testing.T) {
	t.Parallel()

	req := events.APIGatewayV2HTTPRequest{
		Body: `{
			"sender":" NationsSMS ",
			"message":" A TRANSACTION of LKR 1,185.94 was approved on your A/C No. 200680****580 at UBER EATS      CBH. Current Bal LKR 95200.23 ",
			"received_at":"2026-02-04T12:34:56Z",
			"device_id":" pixel-8 "
		}`,
	}

	payload, err := ParsePayload(req)
	if err != nil {
		t.Fatalf("ParsePayload() error = %v", err)
	}

	if payload.Sender != "NationsSMS" {
		t.Fatalf("Sender = %q, want %q", payload.Sender, "NationsSMS")
	}
	if payload.Message != "A TRANSACTION of LKR 1,185.94 was approved on your A/C No. 200680****580 at UBER EATS      CBH. Current Bal LKR 95200.23" {
		t.Fatalf("Message was not trimmed: %q", payload.Message)
	}
	if payload.DeviceID != "pixel-8" {
		t.Fatalf("DeviceID = %q, want %q", payload.DeviceID, "pixel-8")
	}
	if payload.ReceivedAt != "2026-02-04T12:34:56Z" {
		t.Fatalf("ReceivedAt = %q, want validated timestamp", payload.ReceivedAt)
	}
}

func TestCompileBankMatchersRejectsMissingNamedGroups(t *testing.T) {
	t.Parallel()

	_, err := CompileBankMatchers([]config.BankConfig{{
		Name:    "broken",
		Senders: []string{`^NationsSMS$`},
		Patterns: []config.MessagePatternConfig{{
			Name:      "purchase",
			Direction: "debit",
			Pattern:   `^A TRANSACTION of LKR (?P<amount>[0-9,]+\.[0-9]{2}) at (?P<merchant>.+)$`,
		}},
	}})
	if err == nil {
		t.Fatal("CompileBankMatchers() error = nil, want missing-group error")
	}
}

func TestCompileBankMatchersAcceptsDescriptionGroup(t *testing.T) {
	t.Parallel()

	_, err := CompileBankMatchers([]config.BankConfig{{
		Name:    "nations",
		Senders: []string{`^NationsSMS$`},
		Patterns: []config.MessagePatternConfig{{
			Name:      "purchase",
			Direction: "debit",
			Pattern:   `^A TRANSACTION of LKR (?P<amount>[0-9,]+\.[0-9]{2}) was approved on your A/C No\. (?P<account>[0-9*]+) at (?P<description>.+?)\. Current Bal LKR [0-9,]+\.[0-9]{2}$`,
		}},
	}})
	if err != nil {
		t.Fatalf("CompileBankMatchers() error = %v", err)
	}
}

func TestValidateMessagePatternRequiresDescriptionOrMerchantGroup(t *testing.T) {
	t.Parallel()

	re := regexp.MustCompile(`^(?P<amount>[0-9,]+\.[0-9]{2}) (?P<account>[0-9*]+)$`)
	err := validateMessagePattern(re, "broken-bank", "broken-pattern")
	if err == nil {
		t.Fatal("validateMessagePattern() error = nil, want description-group error")
	}
}

func TestMatcherForSenderRejectsAmbiguousSender(t *testing.T) {
	t.Parallel()

	matchers := []BankMatcher{
		{name: "one", senderRegex: []*regexp.Regexp{regexp.MustCompile(`^NationsSMS$`)}},
		{name: "two", senderRegex: []*regexp.Regexp{regexp.MustCompile(`^NationsSMS$`)}},
	}

	_, err := MatcherForSender(matchers, "NationsSMS")
	if err == nil {
		t.Fatal("MatcherForSender() error = nil, want ambiguity error")
	}
}
