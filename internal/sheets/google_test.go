package sheets

import "testing"

func TestSpreadsheetIDFromURL(t *testing.T) {
	t.Parallel()

	id, err := SpreadsheetIDFromURL("https://docs.google.com/spreadsheets/d/abc123/edit#gid=0")
	if err != nil {
		t.Fatalf("SpreadsheetIDFromURL() error = %v", err)
	}
	if id != "abc123" {
		t.Fatalf("SpreadsheetIDFromURL() = %q, want %q", id, "abc123")
	}
}
