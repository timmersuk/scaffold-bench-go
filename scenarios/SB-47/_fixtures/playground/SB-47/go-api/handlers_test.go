package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleStats_PostDoesNotPanic(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/stats", strings.NewReader(`{"Key":"alpha"}`))
	w := httptest.NewRecorder()
	handleStats(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleStats_GetReturnsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	w := httptest.NewRecorder()
	handleStats(w, req)
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}
}
