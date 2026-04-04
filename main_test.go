package main

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

type fakeSheetStore struct {
	appendErr    error
	appendedRows [][]interface{}
}

func (f *fakeSheetStore) AppendRow(_, _ string, row []interface{}) error {
	if f.appendErr != nil {
		return f.appendErr
	}
	f.appendedRows = append(f.appendedRows, row)
	return nil
}

func TestParsePayloadNormalizesAndConvertsTimezone(t *testing.T) {
	t.Parallel()

	loc := time.FixedZone("UTC+0530", 5*60*60+30*60)
	req := events.APIGatewayV2HTTPRequest{
		Body: `{
			"sender":" NationsSMS ",
			"message":" A TRANSACTION of LKR 1,185.94 was approved on your A/C No. 200680****580 at UBER EATS      CBH. Current Bal LKR 95200.23 ",
			"received_at":"2026-02-04T12:34:56Z",
			"device_id":" pixel-8 "
		}`,
	}

	payload, err := parsePayload(req, loc)
	if err != nil {
		t.Fatalf("parsePayload() error = %v", err)
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
	if payload.ReceivedAt != "2026-02-04T18:04:56+05:30" {
		t.Fatalf("ReceivedAt = %q, want converted timestamp", payload.ReceivedAt)
	}
}

func TestCompileBankMatchersRejectsMissingNamedGroups(t *testing.T) {
	t.Parallel()

	_, err := compileBankMatchers([]BankConfig{{
		Name:    "broken",
		Senders: []string{`^NationsSMS$`},
		Pattern: `^A TRANSACTION of (?P<currency>[A-Z]{3}) (?P<amount>[0-9,]+\.[0-9]{2}) at (?P<merchant>.+)$`,
	}})
	if err == nil {
		t.Fatal("compileBankMatchers() error = nil, want missing-group error")
	}
}

func TestCompileBankMatchersAcceptsCurrencyFallbackGroup(t *testing.T) {
	t.Parallel()

	_, err := compileBankMatchers([]BankConfig{{
		Name:    "nations",
		Senders: []string{`^NationsSMS$`},
		Pattern: `^A TRANSACTION of (?P<amount>[0-9,]+\.[0-9]{2}) was approved on your A/C No\. (?P<account>[0-9*]+) at (?P<merchant>.+?)\. Current Bal (?P<balance_currency>[A-Z]{3}) (?P<balance>[0-9,]+\.[0-9]{2})$`,
	}})
	if err != nil {
		t.Fatalf("compileBankMatchers() error = %v", err)
	}
}

func TestSpreadsheetIDFromURL(t *testing.T) {
	t.Parallel()

	id, err := spreadsheetIDFromURL("https://docs.google.com/spreadsheets/d/abc123/edit#gid=0")
	if err != nil {
		t.Fatalf("spreadsheetIDFromURL() error = %v", err)
	}
	if id != "abc123" {
		t.Fatalf("spreadsheetIDFromURL() = %q, want %q", id, "abc123")
	}
}

func TestValidateBankPatternRequiresCurrencyGroup(t *testing.T) {
	t.Parallel()

	re := regexp.MustCompile(`^(?P<amount>[0-9,]+\.[0-9]{2}) (?P<merchant>.+) (?P<account>[0-9*]+) (?P<balance>[0-9,]+\.[0-9]{2})$`)
	err := validateBankPattern(re, "broken")
	if err == nil {
		t.Fatal("validateBankPattern() error = nil, want currency-group error")
	}
}

func TestMatcherForSenderRejectsAmbiguousSender(t *testing.T) {
	t.Parallel()

	h := &handler{bankMatchers: []bankMatcher{
		{name: "one", senderRegex: []*regexp.Regexp{regexp.MustCompile(`^NationsSMS$`)}},
		{name: "two", senderRegex: []*regexp.Regexp{regexp.MustCompile(`^NationsSMS$`)}},
	}}

	_, err := h.matcherForSender("NationsSMS")
	if err == nil {
		t.Fatal("matcherForSender() error = nil, want ambiguity error")
	}
}

func TestHandleReturnsServerErrorWhenAppendFails(t *testing.T) {
	t.Parallel()

	sheets := &fakeSheetStore{appendErr: errors.New("append failed")}
	h := testHandler(t, sheets)

	resp, err := h.handle(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("handle() error = %v", err)
	}
	if resp.StatusCode != 500 {
		t.Fatalf("StatusCode = %d, want 500", resp.StatusCode)
	}
}

func TestHandleAppendsRowWhenRequestIsValid(t *testing.T) {
	t.Parallel()

	sheets := &fakeSheetStore{}
	h := testHandler(t, sheets)

	resp, err := h.handle(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("handle() error = %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if len(sheets.appendedRows) != 1 {
		t.Fatalf("Append calls = %d, want 1", len(sheets.appendedRows))
	}
	if len(sheets.appendedRows[0]) != 11 {
		t.Fatalf("Appended row columns = %d, want 11", len(sheets.appendedRows[0]))
	}
}

func testHandler(t *testing.T, sheets *fakeSheetStore) *handler {
	t.Helper()

	bankMatchers, err := compileBankMatchers([]BankConfig{{
		Name:    "nations",
		Senders: []string{`^NationsSMS$`},
		Pattern: `^A TRANSACTION of (?P<currency>[A-Z]{3}) (?P<amount>[0-9,]+\.[0-9]{2}) was approved on your A/C No\. (?P<account>[0-9*]+) at (?P<merchant>.+?)\. Current Bal (?P<balance_currency>[A-Z]{3}) (?P<balance>[0-9,]+\.[0-9]{2})$`,
	}})
	if err != nil {
		t.Fatalf("compileBankMatchers() error = %v", err)
	}

	return &handler{
		config: Config{
			Spreadsheet: SpreadsheetConfig{
				SheetName: "Transactions",
			},
		},
		bankMatchers:  bankMatchers,
		sheets:        sheets,
		spreadsheetID: "sheet-id",
		authToken:     "secret",
		location:      time.UTC,
	}
}

func testRequest() events.APIGatewayV2HTTPRequest {
	return events.APIGatewayV2HTTPRequest{
		Body: `{
			"sender":"NationsSMS",
			"message":"A TRANSACTION of LKR 1,185.94 was approved on your A/C No. 200680****580 at UBER EATS      CBH. Current Bal LKR 95200.23",
			"received_at":"2026-02-04T12:34:56Z",
			"device_id":"pixel-8"
		}`,
		Headers: map[string]string{
			"x-auth-token": "secret",
		},
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: "POST",
			},
		},
	}
}
