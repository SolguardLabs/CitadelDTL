package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"citadeldtl/src/domain"
)

func TestSnapshotEndpointReturnsStructuredStateAndSecurityHeaders(t *testing.T) {
	service, err := NewService()
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/v1/snapshot", nil)
	response := httptest.NewRecorder()
	NewHTTPServer(service).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	if response.Header().Get("x-content-type-options") != "nosniff" || response.Header().Get("cache-control") != "no-store" {
		t.Fatalf("missing response hardening headers: %#v", response.Header())
	}
	var snapshot domain.Snapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Summary.TotalAccounts == 0 {
		t.Fatal("expected bootstrapped accounts")
	}
}

func TestAuditEndpointRejectsUnsupportedMethod(t *testing.T) {
	service, err := NewService()
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/v1/audit", nil)
	response := httptest.NewRecorder()
	NewHTTPServer(service).ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}
