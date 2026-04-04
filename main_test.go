package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
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
		Patterns: []MessagePatternConfig{{
			Name:      "purchase",
			Direction: "debit",
			Pattern:   `^A TRANSACTION of LKR (?P<amount>[0-9,]+\.[0-9]{2}) at (?P<merchant>.+)$`,
		}},
	}})
	if err == nil {
		t.Fatal("compileBankMatchers() error = nil, want missing-group error")
	}
}

func TestCompileBankMatchersAcceptsDescriptionGroup(t *testing.T) {
	t.Parallel()

	_, err := compileBankMatchers([]BankConfig{{
		Name:    "nations",
		Senders: []string{`^NationsSMS$`},
		Patterns: []MessagePatternConfig{{
			Name:      "purchase",
			Direction: "debit",
			Pattern:   `^A TRANSACTION of LKR (?P<amount>[0-9,]+\.[0-9]{2}) was approved on your A/C No\. (?P<account>[0-9*]+) at (?P<description>.+?)\. Current Bal LKR [0-9,]+\.[0-9]{2}$`,
		}},
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

	resp, err := h.handle(context.Background(), testPurchaseRequest())
	if err != nil {
		t.Fatalf("handle() error = %v", err)
	}
	if resp.StatusCode != 500 {
		t.Fatalf("StatusCode = %d, want 500", resp.StatusCode)
	}
}

func TestHandleAppendsRowWhenPurchaseIsValid(t *testing.T) {
	t.Parallel()

	sheets := &fakeSheetStore{}
	h := testHandler(t, sheets)

	resp, err := h.handle(context.Background(), testPurchaseRequest())
	if err != nil {
		t.Fatalf("handle() error = %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if len(sheets.appendedRows) != 1 {
		t.Fatalf("Append calls = %d, want 1", len(sheets.appendedRows))
	}
	if len(sheets.appendedRows[0]) != 8 {
		t.Fatalf("Appended row columns = %d, want 8", len(sheets.appendedRows[0]))
	}
	if got := sheets.appendedRows[0][2]; got != "debit" {
		t.Fatalf("Direction = %v, want debit", got)
	}
	if got := sheets.appendedRows[0][3]; got != "UBER EATS CBH" {
		t.Fatalf("Description = %v, want UBER EATS CBH", got)
	}
}

func TestHandleAppendsRowWhenCreditMessageIsValid(t *testing.T) {
	t.Parallel()

	sheets := &fakeSheetStore{}
	h := testHandler(t, sheets)

	resp, err := h.handle(context.Background(), testCreditRequest())
	if err != nil {
		t.Fatalf("handle() error = %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if got := sheets.appendedRows[0][2]; got != "credit" {
		t.Fatalf("Direction = %v, want credit", got)
	}
	if got := sheets.appendedRows[0][3]; got != "ISLIPS" {
		t.Fatalf("Description = %v, want ISLIPS", got)
	}
}

func TestServeHTTPReturnsMethodNotAllowed(t *testing.T) {
	t.Parallel()

	h := testHandler(t, &fakeSheetStore{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()

	h.serveHTTP(resp, req)

	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("StatusCode = %d, want %d", resp.Code, http.StatusMethodNotAllowed)
	}
}

func TestServeHTTPReturnsUnauthorizedWithoutToken(t *testing.T) {
	t.Parallel()

	h := testHandler(t, &fakeSheetStore{})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(testPurchaseRequest().Body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	h.serveHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("StatusCode = %d, want %d", resp.Code, http.StatusUnauthorized)
	}
}

func TestServeHTTPAppendsRowWhenRequestIsValid(t *testing.T) {
	t.Parallel()

	sheets := &fakeSheetStore{}
	h := testHandler(t, sheets)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(testPurchaseRequest().Body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-auth-token", "secret")
	resp := httptest.NewRecorder()

	h.serveHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.Code, http.StatusOK)
	}
	if len(sheets.appendedRows) != 1 {
		t.Fatalf("Append calls = %d, want 1", len(sheets.appendedRows))
	}
}

func testHandler(t *testing.T, sheets *fakeSheetStore) *handler {
	t.Helper()

	bankMatchers, err := compileBankMatchers([]BankConfig{{
		Name:    "nations",
		Senders: []string{`^NationsSMS$`},
		Patterns: []MessagePatternConfig{
			{
				Name:      "purchase",
				Direction: "debit",
				Pattern:   `^A TRANSACTION of LKR (?P<amount>[0-9,]+\.[0-9]{2}) was approved on your A/C No\. (?P<account>[0-9*]+) at (?P<description>.+?)\. Current Bal LKR [0-9,]+\.[0-9]{2}$`,
			},
			{
				Name:      "islips",
				Direction: "credit",
				Pattern:   `^\s*(?P<description>ISLIPS)\s+was performed on your Account No\. (?P<account>[0-9A-Za-zXx*]+) for LKR (?P<amount>[0-9,]+\.[0-9]{2}) CR\.?` + `$`,
			},
		},
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
		logger:        newLogger("error"),
	}
}

func testPurchaseRequest() events.APIGatewayV2HTTPRequest {
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

func testCreditRequest() events.APIGatewayV2HTTPRequest {
	return events.APIGatewayV2HTTPRequest{
		Body: `{
			"sender":"NationsSMS",
			"message":" ISLIPS  was performed on your Account No. 200XXXXX9580 for LKR 174825.38 CR.",
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
