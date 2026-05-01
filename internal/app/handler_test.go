package app

import (
	"context"
	"errors"
	"testing"
)

func TestHandleReturnsServerErrorWhenAppendFails(t *testing.T) {
	t.Parallel()

	sheets := &fakeSheetStore{appendErr: errors.New("append failed")}
	h := testHandler(t, sheets)

	resp, err := h.Handle(context.Background(), testPurchaseRequest())
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if resp.StatusCode != 500 {
		t.Fatalf("StatusCode = %d, want 500", resp.StatusCode)
	}
}

func TestHandleAppendsRowWhenPurchaseIsValid(t *testing.T) {
	t.Parallel()

	sheets := &fakeSheetStore{}
	h := testHandler(t, sheets)

	resp, err := h.Handle(context.Background(), testPurchaseRequest())
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if len(sheets.appendedRows) != 1 {
		t.Fatalf("Append calls = %d, want 1", len(sheets.appendedRows))
	}
	if len(sheets.appendedRows[0]) != 8 {
		t.Fatalf("Appended row columns = %d, want 8", len(sheets.appendedRows[0]))
	}
	if got := sheets.appendedRows[0][2]; got != "debit" {
		t.Fatalf("Direction = %v, want debit", got)
	}
	if got := sheets.appendedRows[0][3]; got != "UBER EATS CBH" {
		t.Fatalf("Description = %v, want UBER EATS CBH", got)
	}
}

func TestHandleAppendsRowWhenPosReversalIsValid(t *testing.T) {
	t.Parallel()

	sheets := &fakeSheetStore{}
	h := testHandler(t, sheets)

	resp, err := h.Handle(context.Background(), testPosReversalRequest())
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if got := sheets.appendedRows[0][1]; got != "298.15" {
		t.Fatalf("Amount = %v, want 298.15", got)
	}
	if got := sheets.appendedRows[0][2]; got != "credit" {
		t.Fatalf("Direction = %v, want credit", got)
	}
	if got := sheets.appendedRows[0][3]; got != "POS Reversal Transaction - UBER _" {
		t.Fatalf("Description = %v, want POS Reversal Transaction - UBER _", got)
	}
}

func TestHandleAppendsRowWhenCreditMessageIsValid(t *testing.T) {
	t.Parallel()

	sheets := &fakeSheetStore{}
	h := testHandler(t, sheets)

	resp, err := h.Handle(context.Background(), testCreditRequest())
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if got := sheets.appendedRows[0][2]; got != "credit" {
		t.Fatalf("Direction = %v, want credit", got)
	}
	if got := sheets.appendedRows[0][3]; got != "ISLIPS" {
		t.Fatalf("Description = %v, want ISLIPS", got)
	}
}

func TestHandleAppendsRowWhenCreditCeftsMessageIsValid(t *testing.T) {
	t.Parallel()

	sheets := &fakeSheetStore{}
	h := testHandler(t, sheets)

	resp, err := h.Handle(context.Background(), testCreditCeftsRequest())
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if got := sheets.appendedRows[0][1]; got != "20000.00" {
		t.Fatalf("Amount = %v, want 20000.00", got)
	}
	if got := sheets.appendedRows[0][2]; got != "credit" {
		t.Fatalf("Direction = %v, want credit", got)
	}
	if got := sheets.appendedRows[0][3]; got != "ONLINE FUND TRF VIA CEFTS" {
		t.Fatalf("Description = %v, want ONLINE FUND TRF VIA CEFTS", got)
	}
}
