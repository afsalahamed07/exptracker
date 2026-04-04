package main

import "google.golang.org/api/sheets/v4"

func (s googleSheetStore) AppendRow(spreadsheetID, sheetName string, row []interface{}) error {
	values := &sheets.ValueRange{Values: [][]interface{}{row}}
	_, err := s.service.Spreadsheets.Values.Append(spreadsheetID, sheetName, values).
		ValueInputOption("RAW").
		InsertDataOption("INSERT_ROWS").
		Do()
	return err
}
