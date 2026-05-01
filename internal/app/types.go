package app

import (
	"sms-ingest/internal/config"
	"sms-ingest/internal/logging"
	"sms-ingest/internal/parser"
)

type Handler struct {
	config       config.Config
	bankMatchers []parser.BankMatcher
	sheets       sheetStore
	authToken    string
	logger       logging.Logger
}

type sheetStore interface {
	AppendRow(sheetName string, row []any) error
}

type envVars struct {
	spreadsheetURL    string
	authToken         string
	googleCredentials string
	logLevel          string
}
