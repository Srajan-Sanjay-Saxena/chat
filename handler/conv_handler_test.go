package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestWriteJSONError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSONError(rec, http.StatusBadRequest, "bad request")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code: got=%d want=%d", rec.Code, http.StatusBadRequest)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("unexpected content type: got=%q want=%q", got, "application/json")
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if payload["error"] != "bad request" {
		t.Fatalf("unexpected error body: %#v", payload)
	}
}

func TestConvIdExtract(t *testing.T) {
	validID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/conversation/members?conversation_id="+validID.String(), nil)

	got, err := ConvIdExtract(req)
	if err != nil {
		t.Fatalf("ConvIdExtract returned error: %v", err)
	}
	if got != validID {
		t.Fatalf("unexpected conversation id: got=%s want=%s", got, validID)
	}
}

func TestConvIdExtractMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/conversation/members", nil)

	_, err := ConvIdExtract(req)
	if err == nil {
		t.Fatalf("expected error for missing conversation_id")
	}
}
