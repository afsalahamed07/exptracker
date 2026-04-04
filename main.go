package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

func main() {
	h, err := newHandler()
	if err != nil {
		log.Fatal(err)
	}
	lambda.Start(h.handle)
}

func newHandler() (*handler, error) {
	cfg, err := loadConfig("config.yml")
	if err != nil {
		return nil, err
	}

	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	bankMatchers, err := compileBankMatchers(cfg.Banks)
	if err != nil {
		return nil, fmt.Errorf("compile banks: %w", err)
	}

	authToken := strings.TrimSpace(os.Getenv("AUTH_TOKEN"))
	if authToken == "" {
		return nil, errors.New("AUTH_TOKEN env var is required")
	}

	spreadsheetURL := strings.TrimSpace(os.Getenv("SPREADSHEET_URL"))
	if spreadsheetURL == "" {
		spreadsheetURL = strings.TrimSpace(cfg.Spreadsheet.URL)
	}
	if spreadsheetURL == "" {
		return nil, errors.New("spreadsheet.url is required in config.yml or SPREADSHEET_URL env var")
	}

	spreadsheetID, err := spreadsheetIDFromURL(spreadsheetURL)
	if err != nil {
		return nil, err
	}

	credsJSON := strings.TrimSpace(os.Getenv("GOOGLE_CREDENTIALS_JSON"))
	if credsJSON == "" {
		return nil, errors.New("GOOGLE_CREDENTIALS_JSON env var is required")
	}

	srv, err := sheets.NewService(context.Background(),
		option.WithCredentialsJSON([]byte(credsJSON)),
		option.WithScopes(sheets.SpreadsheetsScope),
	)
	if err != nil {
		return nil, fmt.Errorf("init sheets service: %w", err)
	}

	loc := time.UTC
	if strings.TrimSpace(cfg.Timezone) != "" {
		loc, err = time.LoadLocation(cfg.Timezone)
		if err != nil {
			return nil, fmt.Errorf("invalid timezone: %w", err)
		}
	}

	return &handler{
		config:        cfg,
		bankMatchers:  bankMatchers,
		sheets:        googleSheetStore{service: srv},
		spreadsheetID: spreadsheetID,
		authToken:     authToken,
		location:      loc,
	}, nil
}

func (h *handler) handle(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	if req.RequestContext.HTTP.Method != http.MethodPost {
		return jsonResponse(http.StatusMethodNotAllowed, "method not allowed"), nil
	}

	if err := h.validateAuth(req.Headers); err != nil {
		return jsonResponse(http.StatusUnauthorized, "unauthorized"), nil
	}

	payload, err := parsePayload(req, h.location)
	if err != nil {
		return jsonResponse(http.StatusBadRequest, err.Error()), nil
	}

	matcher, err := h.matcherForSender(payload.Sender)
	if err != nil {
		return jsonResponse(http.StatusForbidden, "sender not allowed"), nil
	}

	parsed, err := h.parseSMS(payload, matcher)
	if err != nil {
		return jsonResponse(http.StatusBadRequest, err.Error()), nil
	}

	if err := h.appendToSheet(ctx, parsed); err != nil {
		log.Printf("append failure: %v", err)
		return jsonResponse(http.StatusInternalServerError, "failed to append to sheet"), nil
	}

	return jsonResponse(http.StatusOK, "ok"), nil
}

func (h *handler) validateAuth(headers map[string]string) error {
	token := headerValue(headers, "x-auth-token")
	if token == "" {
		return errors.New("missing auth token")
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(h.authToken)) != 1 {
		return errors.New("invalid auth token")
	}
	return nil
}

func (h *handler) appendToSheet(_ context.Context, tx ParsedTransaction) error {
	row := []interface{}{
		tx.ReceivedAt,
		tx.Amount,
		tx.Currency,
		tx.Merchant,
		tx.Balance,
		tx.AccountMask,
		tx.RawMessage,
		tx.Sender,
		tx.BalanceCurrency,
		tx.BankName,
		tx.DeviceID,
	}

	err := h.sheets.AppendRow(h.spreadsheetID, h.config.Spreadsheet.SheetName, row)
	if err != nil {
		return fmt.Errorf("append to sheet: %w", err)
	}
	return nil
}

func jsonResponse(status int, message string) events.APIGatewayV2HTTPResponse {
	payload, _ := json.Marshal(map[string]string{"message": message})
	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(payload),
	}
}

func headerValue(headers map[string]string, key string) string {
	if value, ok := headers[key]; ok {
		return value
	}
	lowerKey := strings.ToLower(key)
	for k, v := range headers {
		if strings.ToLower(k) == lowerKey {
			return v
		}
	}
	return ""
}

func spreadsheetIDFromURL(url string) (string, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return "", errors.New("spreadsheet url is empty")
	}

	if strings.Contains(url, "/d/") {
		parts := strings.Split(url, "/d/")
		if len(parts) < 2 {
			return "", errors.New("invalid spreadsheet url")
		}
		tail := parts[1]
		id := strings.Split(tail, "/")[0]
		if id == "" {
			return "", errors.New("spreadsheet id not found in url")
		}
		return id, nil
	}

	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return "", errors.New("spreadsheet url missing /d/ segment")
	}

	return url, nil
}
