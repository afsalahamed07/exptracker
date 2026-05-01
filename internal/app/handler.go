package app

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"

	"sms-ingest/internal/config"
	"sms-ingest/internal/logging"
	"sms-ingest/internal/parser"
	"sms-ingest/internal/sheets"
)

func NewHandler() (*Handler, error) {
	cfg, err := config.Load("config.yml")
	if err != nil {
		return nil, err
	}

	env := loadEnvVars()

	bankMatchers, err := parser.CompileBankMatchers(cfg.Banks)
	if err != nil {
		return nil, fmt.Errorf("compile banks: %w", err)
	}

	googleSheetStore, err := sheets.NewStore(env.spreadsheetURL, env.googleCredentials)
	if err != nil {
		return nil, fmt.Errorf("init sheet store: %w", err)
	}

	logger := logging.New(env.logLevel)

	return &Handler{
		config:       cfg,
		bankMatchers: bankMatchers,
		sheets:       googleSheetStore,
		authToken:    env.authToken,
		logger:       logger,
	}, nil
}

func (h *Handler) Handle(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	h.logger.Debugf("received request: method=%s headers=%v body=%s", req.RequestContext.HTTP.Method, maskedHeaders(req.Headers), req.Body)
	if req.RequestContext.HTTP.Method != http.MethodPost {
		h.logger.Warnf("request rejected: method=%s", req.RequestContext.HTTP.Method)
		return jsonResponse(http.StatusMethodNotAllowed, "method not allowed"), nil
	}

	if err := h.validateAuth(req.Headers); err != nil {
		h.logger.Warnf("request unauthorized: %v", err)
		return jsonResponse(http.StatusUnauthorized, "unauthorized"), nil
	}

	payload, err := parser.ParsePayload(req)
	if err != nil {
		h.logger.Warnf("invalid payload: %v", err)
		return jsonResponse(http.StatusBadRequest, err.Error()), nil
	}

	matcher, err := parser.MatcherForSender(h.bankMatchers, payload.Sender)
	if err != nil {
		h.logger.Warnf("sender rejected: sender=%q err=%v", payload.Sender, err)
		return jsonResponse(http.StatusForbidden, "sender not allowed"), nil
	}

	parsed, err := parser.ParseSMS(payload, matcher)
	if err != nil {
		h.logger.Warnf("sms parse failed: sender=%q bank=%q err=%v", payload.Sender, matcher.Name(), err)
		return jsonResponse(http.StatusBadRequest, err.Error()), nil
	}

	if err := h.appendToSheet(ctx, parsed); err != nil {
		h.logger.Errorf("append failed: sender=%q bank=%q err=%v", parsed.Sender, parsed.BankName, err)
		return jsonResponse(http.StatusInternalServerError, "failed to append to sheet"), nil
	}
	h.logger.Debugf("transaction appended: sender=%q bank=%q amount=%s direction=%s description=%q", parsed.Sender, parsed.BankName, parsed.Amount, parsed.Direction, parsed.Description)

	return jsonResponse(http.StatusOK, "ok"), nil
}

func (h *Handler) validateAuth(headers map[string]string) error {
	token := headerValue(headers, "x-auth-token")
	if token == "" {
		return errors.New("missing auth token")
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(h.authToken)) != 1 {
		return errors.New("invalid auth token")
	}
	return nil
}

func (h *Handler) appendToSheet(_ context.Context, tx parser.ParsedTransaction) error {
	row := []any{
		tx.ReceivedAt,
		tx.Amount,
		tx.Direction,
		tx.Description,
		tx.AccountMask,
		tx.Sender,
		tx.DeviceID,
		tx.BankName,
	}

	err := h.sheets.AppendRow(h.config.Spreadsheet.SheetName, row)
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

func loadEnvVars() envVars {
	spreadsheetURL := strings.TrimSpace(os.Getenv("SPREADSHEET_URL"))
	authToken := strings.TrimSpace(os.Getenv("AUTH_TOKEN"))
	googleCredentials := strings.TrimSpace(os.Getenv("GOOGLE_CREDENTIALS_JSON"))
	logLevel := strings.TrimSpace(os.Getenv("LOG_LEVEL"))

	return envVars{
		spreadsheetURL:    spreadsheetURL,
		authToken:         authToken,
		googleCredentials: googleCredentials,
		logLevel:          logLevel,
	}
}
