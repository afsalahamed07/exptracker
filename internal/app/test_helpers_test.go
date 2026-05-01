package app

import (
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"sms-ingest/internal/config"
	"sms-ingest/internal/logging"
	"sms-ingest/internal/parser"
)

type fakeSheetStore struct {
	appendErr    error
	appendedRows [][]any
}

func (f *fakeSheetStore) AppendRow(_ string, row []any) error {
	if f.appendErr != nil {
		return f.appendErr
	}
	f.appendedRows = append(f.appendedRows, row)
	return nil
}

func testHandler(t *testing.T, sheets *fakeSheetStore) *Handler {
	t.Helper()

	bankMatchers, err := parser.CompileBankMatchers([]config.BankConfig{{
		Name:    "nations",
		Senders: []string{`^NationsSMS$`},
		Patterns: []config.MessagePatternConfig{
			{
				Name:      "pos_reversal",
				Direction: "credit",
				Pattern:   `^A TRANSACTION of LKR (?P<amount>[0-9,]+\.[0-9]{2}) was approved on your A/C No\. (?P<account>[0-9*]+) at (?P<description>POS Reversal Transaction - .+?)\. Current Bal LKR [0-9,]+\.[0-9]{2}\s*\.?$`,
			},
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
			{
				Name:      "online_fund_trf_via_cefts",
				Direction: "credit",
				Pattern:   `^(?P<description>ONLINE FUND TRF VIA CEFTS) was performed on your Account No\. (?P<account>[0-9A-Za-zXx*]+) for LKR (?P<amount>[0-9,]+\.[0-9]{2}) CR\. Current Bal LKR [0-9,]+\.[0-9]{2}(?: Call .+)?$`,
			},
		},
	}})
	if err != nil {
		t.Fatalf("CompileBankMatchers() error = %v", err)
	}

	return &Handler{
		config: config.Config{
			Spreadsheet: config.SpreadsheetConfig{
				SheetName: "Transactions",
			},
		},
		bankMatchers: bankMatchers,
		sheets:       sheets,
		authToken:    "secret",
		logger:       logging.New("error"),
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

func testCreditCeftsRequest() events.APIGatewayV2HTTPRequest {
	return events.APIGatewayV2HTTPRequest{
		Body: `{
			"sender":"NationsSMS",
			"message":"ONLINE FUND TRF VIA CEFTS was performed on your Account No. 2006xxxx9580 for LKR 20000.00 CR. Current Bal LKR 107514.98 Call 0114711411 for any inquiry.",
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

func testPosReversalRequest() events.APIGatewayV2HTTPRequest {
	return events.APIGatewayV2HTTPRequest{
		Body: `{
			"sender":"NationsSMS",
			"message":"A TRANSACTION of LKR 298.15 was approved on your A/C No. 200680****580 at POS Reversal Transaction - UBER   _. Current Bal LKR 88770.27",
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
