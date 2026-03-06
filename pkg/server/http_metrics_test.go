package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetricsMiddleware_RecordsRequest(t *testing.T) {
	called := false
	h := metricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}), "test_route")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if !called {
		t.Fatalf("expected wrapped handler to be called")
	}
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Result().StatusCode)
	}
}

