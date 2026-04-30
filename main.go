package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if os.Getenv("LOCAL_HTTP") == "1" {
		if err := runLocalHTTP(h); err != nil {
			h.logger.Errorf("local HTTP server stopped: %v", err)
			os.Exit(1)
		}
		return
	}
	lambda.Start(h.handle)
}

func newHandler() (*handler, error) {
	cfg, err := loadConfig("config.yml")
	if err != nil {
		return nil, err
	}

	spreadsheetID, err := spreadsheetID()
	if err != nil {
		return nil, fmt.Errorf("get spreadsheet ID: %w", err)
	}

	bankMatchers, err := compileBankMatchers(cfg.Banks)
	if err != nil {
		return nil, fmt.Errorf("compile banks: %w", err)
	}

	authToken := strings.TrimSpace(os.Getenv("AUTH_TOKEN"))
	if authToken == "" {
		return nil, errors.New("AUTH_TOKEN env var is required")
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

	logger := newLogger(os.Getenv("LOG_LEVEL"))

	return &handler{
		config:        cfg,
		bankMatchers:  bankMatchers,
		sheets:        googleSheetStore{service: srv},
		spreadsheetID: spreadsheetID,
		authToken:     authToken,
		location:      loc,
		logger:        logger,
	}, nil
}

func (h *handler) handle(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	h.logger.Debugf("received request: method=%s headers=%v body=%s", req.RequestContext.HTTP.Method, maskedHeaders(req.Headers), req.Body)
	if req.RequestContext.HTTP.Method != http.MethodPost {
		h.logger.Warnf("request rejected: method=%s", req.RequestContext.HTTP.Method)
		return jsonResponse(http.StatusMethodNotAllowed, "method not allowed"), nil
	}

	if err := h.validateAuth(req.Headers); err != nil {
		h.logger.Warnf("request unauthorized: %v", err)
		return jsonResponse(http.StatusUnauthorized, "unauthorized"), nil
	}

	payload, err := parsePayload(req, h.location)
	if err != nil {
		h.logger.Warnf("invalid payload: %v", err)
		return jsonResponse(http.StatusBadRequest, err.Error()), nil
	}

	matcher, err := h.matcherForSender(payload.Sender)
	if err != nil {
		h.logger.Warnf("sender rejected: sender=%q err=%v", payload.Sender, err)
		return jsonResponse(http.StatusForbidden, "sender not allowed"), nil
	}

	parsed, err := h.parseSMS(payload, matcher)
	if err != nil {
		h.logger.Warnf("sms parse failed: sender=%q bank=%q err=%v", payload.Sender, matcher.name, err)
		return jsonResponse(http.StatusBadRequest, err.Error()), nil
	}

	if err := h.appendToSheet(ctx, parsed); err != nil {
		h.logger.Errorf("append failed: sender=%q bank=%q err=%v", parsed.Sender, parsed.BankName, err)
		return jsonResponse(http.StatusInternalServerError, "failed to append to sheet"), nil
	}
	h.logger.Debugf("transaction appended: sender=%q bank=%q amount=%s direction=%s description=%q", parsed.Sender, parsed.BankName, parsed.Amount, parsed.Direction, parsed.Description)

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
		tx.Direction,
		tx.Description,
		tx.AccountMask,
		tx.Sender,
		tx.DeviceID,
		tx.BankName,
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

func maskedHeaders(headers map[string]string) map[string]string {
	masked := make(map[string]string, len(headers))
	for key, value := range headers {
		if strings.EqualFold(key, "x-auth-token") {
			masked[key] = "[redacted]"
			continue
		}
		masked[key] = value
	}
	return masked
}
