package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

// loginSessionEntry holds a login session and its expiry time.
type loginSessionEntry struct {
	session   LoginSession
	createdAt time.Time
}

const loginSessionTTL = 10 * time.Minute

// loginSessions stores active login sessions by session ID.
type loginSessions struct {
	mu       sync.Mutex
	sessions map[string]*loginSessionEntry
}

func newLoginSessions() *loginSessions {
	ls := &loginSessions{sessions: make(map[string]*loginSessionEntry)}
	go ls.cleanupLoop()
	return ls
}

func (ls *loginSessions) create(session LoginSession) string {
	id := generateSessionID()
	ls.mu.Lock()
	ls.sessions[id] = &loginSessionEntry{session: session, createdAt: time.Now()}
	ls.mu.Unlock()
	return id
}

func (ls *loginSessions) get(id string) (LoginSession, bool) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	entry, ok := ls.sessions[id]
	if !ok {
		return nil, false
	}
	if time.Since(entry.createdAt) > loginSessionTTL {
		delete(ls.sessions, id)
		return nil, false
	}
	return entry.session, true
}

func (ls *loginSessions) remove(id string) {
	ls.mu.Lock()
	delete(ls.sessions, id)
	ls.mu.Unlock()
}

func (ls *loginSessions) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ls.mu.Lock()
		for id, entry := range ls.sessions {
			if time.Since(entry.createdAt) > loginSessionTTL {
				delete(ls.sessions, id)
			}
		}
		ls.mu.Unlock()
	}
}

func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// handleLoginFlows returns available login flows.
func (s *Server) handleLoginFlows(w http.ResponseWriter, r *http.Request) {
	if s.loginProvider == nil {
		writeError(w, http.StatusServiceUnavailable, "login not available", "LOGIN_UNAVAILABLE")
		return
	}
	flows := s.loginProvider.GetLoginFlows()
	writeJSON(w, http.StatusOK, LoginFlowsResponse{Flows: flows})
}

// handleLoginStart initiates a new login process.
func (s *Server) handleLoginStart(w http.ResponseWriter, r *http.Request) {
	if s.loginProvider == nil {
		writeError(w, http.StatusServiceUnavailable, "login not available", "LOGIN_UNAVAILABLE")
		return
	}

	var req LoginStartRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Flow == "" {
		writeError(w, http.StatusBadRequest, "flow is required", "INVALID_REQUEST")
		return
	}

	session, err := s.loginProvider.StartLogin(r.Context(), req.Flow)
	if err != nil {
		s.log.Err(err).Str("flow", req.Flow).Msg("Failed to start login")
		writeError(w, http.StatusInternalServerError, err.Error(), "LOGIN_FAILED")
		return
	}

	sessionID := s.loginSessions.create(session)
	s.log.Info().Str("session_id", sessionID).Str("flow", req.Flow).Str("step", session.StepID()).Msg("Login session started")

	writeJSON(w, http.StatusOK, LoginStepResponse{
		SessionID:    sessionID,
		StepID:       session.StepID(),
		Instructions: session.Instructions(),
		Fields:       session.Fields(),
	})
}

// handleLoginStep submits user input for the current login step.
func (s *Server) handleLoginStep(w http.ResponseWriter, r *http.Request) {
	var req LoginStepRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required", "INVALID_REQUEST")
		return
	}
	if len(req.Input) == 0 {
		writeError(w, http.StatusBadRequest, "input is required", "INVALID_REQUEST")
		return
	}

	session, ok := s.loginSessions.get(req.SessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "session not found or expired", "SESSION_NOT_FOUND")
		return
	}

	nextSession, complete, err := session.Submit(r.Context(), req.Input)
	if err != nil {
		s.log.Err(err).Str("session_id", req.SessionID).Msg("Login step failed")
		writeError(w, http.StatusInternalServerError, err.Error(), "LOGIN_STEP_FAILED")
		return
	}

	if complete != nil {
		s.loginSessions.remove(req.SessionID)
		s.log.Info().Str("session_id", req.SessionID).Str("login_id", complete.LoginID).Msg("Login completed")
		writeJSON(w, http.StatusOK, LoginStepResponse{
			SessionID: req.SessionID,
			StepID:    "complete",
			Complete:  complete,
		})
		return
	}

	s.log.Info().Str("session_id", req.SessionID).Str("step", nextSession.StepID()).Msg("Login step completed")
	writeJSON(w, http.StatusOK, LoginStepResponse{
		SessionID:    req.SessionID,
		StepID:       nextSession.StepID(),
		Instructions: nextSession.Instructions(),
		Fields:       nextSession.Fields(),
	})
}
