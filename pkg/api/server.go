package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
)

// Config holds the HTTP API and webhook configuration.
type Config struct {
	Listen        string
	APIKey        string
	WebhookURL    string
	WebhookSecret string
}

// Server is the HTTP REST API that talks directly to the iMessage client.
type Server struct {
	config        Config
	provider      IMClientProvider
	loginProvider LoginProvider
	webhook       *WebhookDispatcher
	loginSessions *loginSessions
	httpSrv       *http.Server
	log           zerolog.Logger
}

// New creates an API server. Call Start() to begin listening.
// loginProvider may be nil if login via API is not needed.
func New(cfg Config, provider IMClientProvider, loginProvider LoginProvider, log zerolog.Logger) *Server {
	s := &Server{
		config:        cfg,
		provider:      provider,
		loginProvider: loginProvider,
		log:           log,
	}

	if cfg.WebhookURL != "" {
		s.webhook = NewWebhookDispatcher(cfg.WebhookURL, cfg.WebhookSecret, log)
	}

	if loginProvider != nil {
		s.loginSessions = newLoginSessions()
	}

	mux := http.NewServeMux()

	// Query
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/handles", s.handleHandles)
	mux.HandleFunc("POST /api/v1/validate", s.handleValidate)

	// Send
	mux.HandleFunc("POST /api/v1/send", s.handleSend)
	mux.HandleFunc("POST /api/v1/send-media", s.handleSendMedia)
	mux.HandleFunc("POST /api/v1/react", s.handleReact)
	mux.HandleFunc("POST /api/v1/edit", s.handleEdit)
	mux.HandleFunc("POST /api/v1/unsend", s.handleUnsend)
	mux.HandleFunc("POST /api/v1/typing", s.handleTyping)
	mux.HandleFunc("POST /api/v1/read-receipt", s.handleReadReceipt)

	// Login
	mux.HandleFunc("GET /api/v1/login/flows", s.handleLoginFlows)
	mux.HandleFunc("POST /api/v1/login/start", s.handleLoginStart)
	mux.HandleFunc("POST /api/v1/login/step", s.handleLoginStep)

	// Session
	mux.HandleFunc("POST /api/v1/logout", s.handleLogout)

	// Top-level mux: docs served without auth, everything else requires auth.
	root := http.NewServeMux()
	root.HandleFunc("GET /api/v1/openapi.json", handleOpenAPISpec)
	root.HandleFunc("GET /api/v1/docs", handleSwaggerUI)
	root.Handle("/", s.authMiddleware(mux))

	s.httpSrv = &http.Server{
		Addr:    cfg.Listen,
		Handler: root,
	}

	return s
}

// Start begins listening in a background goroutine.
func (s *Server) Start() {
	if s.config.APIKey == "" {
		s.log.Warn().Msg("HTTP API enabled but api_key is empty — all requests will be rejected")
	}

	go func() {
		s.log.Info().Str("listen", s.config.Listen).Msg("HTTP API server starting")
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Err(err).Msg("HTTP API server failed")
		}
	}()
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

// Webhook returns the webhook dispatcher (nil if not configured).
func (s *Server) Webhook() *WebhookDispatcher {
	return s.webhook
}

// authMiddleware validates the Bearer token on every request.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := s.config.APIKey
		if expected == "" {
			writeError(w, http.StatusUnauthorized, "api_key not configured", "UNAUTHORIZED")
			return
		}

		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == auth || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid or missing bearer token", "UNAUTHORIZED")
			return
		}

		next.ServeHTTP(w, r)
	})
}
