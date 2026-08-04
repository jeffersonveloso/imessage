package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
)

type unavailableClientProvider struct{}

func (unavailableClientProvider) GetActiveClient() (IMClient, error) {
	return nil, errors.New("not connected")
}

func (unavailableClientProvider) GetAnyClient() (IMClient, error) {
	return nil, errors.New("not connected")
}

func (unavailableClientProvider) Reconnect(context.Context) error {
	return errors.New("not connected")
}

func TestServerAuthenticationAndPublicDocs(t *testing.T) {
	server := New(
		Config{APIKey: "test-secret"},
		unavailableClientProvider{},
		nil,
		zerolog.Nop(),
	)

	t.Run("docs do not require authentication", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/docs", nil)
		res := httptest.NewRecorder()

		server.httpSrv.Handler.ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
		}
	})

	t.Run("API rejects a missing token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
		res := httptest.NewRecorder()

		server.httpSrv.Handler.ServeHTTP(res, req)

		if res.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, res.Code)
		}
		var body ErrorResponse
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body.Code != "UNAUTHORIZED" {
			t.Fatalf("expected UNAUTHORIZED code, got %q", body.Code)
		}
	})

	t.Run("valid token reaches the API handler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
		req.Header.Set("Authorization", "Bearer test-secret")
		res := httptest.NewRecorder()

		server.httpSrv.Handler.ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
		}
		var body StatusResponse
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body.Connected {
			t.Fatal("expected disconnected status from unavailable provider")
		}
	})
}
