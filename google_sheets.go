package main

import (
	"errors"
	"os"
	"regexp"
	"strings"

	"google.golang.org/api/sheets/v4"
)

type googleSheetStore struct {
	service *sheets.Service
}

var spreadsheetURLPattern = regexp.MustCompile(`/spreadsheets/d/([^/?#]+)`)

func (s googleSheetStore) AppendRow(spreadsheetID, sheetName string, row []any) error {
	values := &sheets.ValueRange{Values: [][]any{row}}

	_, err := s.service.Spreadsheets.Values.Append(spreadsheetID, sheetName, values).
		ValueInputOption("RAW").
		InsertDataOption("INSERT_ROWS").
		Do()
	return err
}

// TODO: This is a bit of a hack to avoid having to parse the spreadsheet URL in the main function.
// We can probably do this better by having a separate function that takes care of parsing the URL and returning the spreadsheet ID.
// NOTE: Also thinking in terms of spreadsheetID is coupled to google sheets
func spreadsheetID() (string, error) {
	spreadsheetURL := strings.TrimSpace(os.Getenv("SPREADSHEET_URL"))
	if spreadsheetURL == "" {
		return "", errors.New("no spreadsheet url provided. Please set the spreadsheet_url environment variable")
	}

	return spreadsheetIDFromURL(spreadsheetURL)
}

/**
* Google Sheets URLs can come in different formats,
* but the spreadsheet ID is always located between the "/d/" segment
* and the next "/" segment (if it exists).
* i.e : https://docs.google.com/spreadsheets/d/id/edit?gid=0#gid=0
* */
func spreadsheetIDFromURL(url string) (string, error) {
	matches := spreadsheetURLPattern.FindStringSubmatch(url)
	if len(matches) == 2 {
		return matches[1], nil
	}

	return "", errors.New("invalid spreadsheet url provided. Please check the url and try again")
}
