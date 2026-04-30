package main

import (
	"errors"
	"regexp"
	"strings"

	"google.golang.org/api/sheets/v4"
)

type googleSheetStore struct {
	service       *sheets.Service
	spreadsheetID string
}

var spreadsheetURLPattern = regexp.MustCompile(`/spreadsheets/d/([^/?#]+)`)

func (s googleSheetStore) AppendRow(sheetName string, row []any) error {
	values := &sheets.ValueRange{Values: [][]any{row}}

	_, err := s.service.Spreadsheets.Values.Append(s.spreadsheetID, sheetName, values).
		ValueInputOption("RAW").
		InsertDataOption("INSERT_ROWS").
		Do()
	return err
}

/**
* Google Sheets URLs can come in different formats,
* but the spreadsheet ID is always located between the "/d/" segment
* and the next "/" segment (if it exists).
* i.e : https://docs.google.com/spreadsheets/d/id/edit?gid=0#gid=0
* */
func spreadsheetIDFromURL(url string) (string, error) {
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
