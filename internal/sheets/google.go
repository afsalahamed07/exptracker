package sheets

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/api/option"
	googlesheets "google.golang.org/api/sheets/v4"
)

type Store struct {
	service       *googlesheets.Service
	spreadsheetID string
}

var spreadsheetURLPattern = regexp.MustCompile(`/spreadsheets/d/([^/?#]+)`)

func NewStore(url string, credentials string) (Store, error) {
	spreadsheetID, err := SpreadsheetIDFromURL(url)
	if err != nil {
		return Store{}, fmt.Errorf("get spreadsheet ID: %w", err)
	}

	srv, err := googlesheets.NewService(context.Background(),
		option.WithCredentialsJSON([]byte(credentials)),
		option.WithScopes(googlesheets.SpreadsheetsScope),
	)
	if err != nil {
		return Store{}, fmt.Errorf("init sheets service: %w", err)
	}

	return Store{service: srv, spreadsheetID: spreadsheetID}, nil
}

func (s Store) AppendRow(sheetName string, row []any) error {
	values := &googlesheets.ValueRange{Values: [][]any{row}}

	_, err := s.service.Spreadsheets.Values.Append(s.spreadsheetID, sheetName, values).
		ValueInputOption("RAW").
		InsertDataOption("INSERT_ROWS").
		Do()
	return err
}

func SpreadsheetIDFromURL(url string) (string, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return "", errors.New("no spreadsheet url provided. Please set the spreadsheet_url environment variable")
	}

	matches := spreadsheetURLPattern.FindStringSubmatch(url)
	if len(matches) == 2 {
		return matches[1], nil
	}

	return "", errors.New("invalid spreadsheet url provided. Please check the url and try again")
}
