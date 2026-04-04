package main

import (
	"regexp"
	"time"

	"google.golang.org/api/sheets/v4"
)

type SMSPayload struct {
	Sender     string `json:"sender"`
	Message    string `json:"message"`
	ReceivedAt string `json:"received_at"`
	DeviceID   string `json:"device_id"`
}

type ParsedTransaction struct {
	Sender      string
	ReceivedAt  string
	DeviceID    string
	Description string
	AccountMask string
	Amount      string
	Direction   string
	BankName    string
}

type handler struct {
	config        Config
	bankMatchers  []bankMatcher
	sheets        sheetStore
	spreadsheetID string
	authToken     string
	location      *time.Location
	logger        appLogger
}

type bankMatcher struct {
	name        string
	senderRegex []*regexp.Regexp
	messages    []messageMatcher
}

type messageMatcher struct {
	name      string
	direction string
	regex     *regexp.Regexp
}

type sheetStore interface {
	AppendRow(spreadsheetID, sheetName string, row []interface{}) error
}

type googleSheetStore struct {
	service *sheets.Service
}
