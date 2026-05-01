package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeHTTPReturnsMethodNotAllowed(t *testing.T) {
	t.Parallel()

	h := testHandler(t, &fakeSheetStore{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()

	h.serveHTTP(resp, req)

	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("StatusCode = %d, want %d", resp.Code, http.StatusMethodNotAllowed)
	}
}

func TestServeHTTPReturnsUnauthorizedWithoutToken(t *testing.T) {
	t.Parallel()

	h := testHandler(t, &fakeSheetStore{})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(testPurchaseRequest().Body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	h.serveHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("StatusCode = %d, want %d", resp.Code, http.StatusUnauthorized)
	}
}

func TestServeHTTPAppendsRowWhenRequestIsValid(t *testing.T) {
	t.Parallel()

	sheets := &fakeSheetStore{}
	h := testHandler(t, sheets)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(testPurchaseRequest().Body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-auth-token", "secret")
	resp := httptest.NewRecorder()

	h.serveHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.Code, http.StatusOK)
	}
	if len(sheets.appendedRows) != 1 {
		t.Fatalf("Append calls = %d, want 1", len(sheets.appendedRows))
	}
}
