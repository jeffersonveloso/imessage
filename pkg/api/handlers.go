package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lrhodin/imessage/pkg/rustpushgo"
)

// buildConversation normalizes the recipient and builds a WrappedConversation for DMs.
func (s *Server) buildConversation(client IMClient, to string, isSMS bool) rustpushgo.WrappedConversation {
	normalized := client.NormalizeIdentifier(to)
	return rustpushgo.WrappedConversation{
		Participants: []string{client.Handle(), normalized},
		IsSms:        isSMS,
	}
}

// reactionNameToCode maps API reaction names to rustpush tapback codes.
func reactionNameToCode(name string) (uint32, error) {
	switch name {
	case "heart":
		return 0, nil
	case "like":
		return 1, nil
	case "dislike":
		return 2, nil
	case "laugh":
		return 3, nil
	case "emphasize":
		return 4, nil
	case "question":
		return 5, nil
	default:
		return 0, fmt.Errorf("unknown reaction: %s", name)
	}
}

// --- Handlers ---

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeJSON(w, http.StatusOK, StatusResponse{Connected: false})
		return
	}
	writeJSON(w, http.StatusOK, StatusResponse{
		Connected:  true,
		Handle:     client.Handle(),
		AllHandles: client.AllHandles(),
	})
}

func (s *Server) handleHandles(w http.ResponseWriter, r *http.Request) {
	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}
	writeJSON(w, http.StatusOK, HandlesResponse{Handles: client.AllHandles()})
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	var req ValidateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Targets) == 0 {
		writeError(w, http.StatusBadRequest, "targets is required and must not be empty", "INVALID_REQUEST")
		return
	}

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	normalized := make([]string, len(req.Targets))
	for i, t := range req.Targets {
		normalized[i] = client.NormalizeIdentifier(t)
	}

	valid := client.ValidateTargets(normalized, client.Handle())

	validSet := make(map[string]bool, len(valid))
	for _, v := range valid {
		validSet[v] = true
	}
	var invalid []string
	for _, t := range normalized {
		if !validSet[t] {
			invalid = append(invalid, t)
		}
	}
	if invalid == nil {
		invalid = []string{}
	}
	if valid == nil {
		valid = []string{}
	}

	writeJSON(w, http.StatusOK, ValidateResponse{Valid: valid, Invalid: invalid})
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	var req SendRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.To == "" || req.Text == "" {
		writeError(w, http.StatusBadRequest, "to and text are required", "INVALID_REQUEST")
		return
	}

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	conv := s.buildConversation(client, req.To, req.IsSMS)
	uuid, err := client.SendMessage(conv, req.Text, client.Handle(), req.ReplyTo, req.ReplyPart)
	if err != nil {
		s.log.Err(err).Str("to", req.To).Msg("Failed to send message")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to send: %v", err), "SEND_FAILED")
		return
	}

	s.log.Info().Str("uuid", uuid).Str("to", req.To).Msg("Message sent via API")
	writeJSON(w, http.StatusOK, SendResponse{UUID: uuid, Status: "sent"})
}

func (s *Server) handleSendMedia(w http.ResponseWriter, r *http.Request) {
	var req SendMediaRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.To == "" || req.Data == "" || req.MimeType == "" || req.Filename == "" {
		writeError(w, http.StatusBadRequest, "to, data, mime_type, and filename are required", "INVALID_REQUEST")
		return
	}

	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "data must be valid base64", "INVALID_REQUEST")
		return
	}

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	conv := s.buildConversation(client, req.To, req.IsSMS)
	uti := client.MimeToUTI(req.MimeType)
	uuid, err := client.SendAttachment(conv, data, req.MimeType, uti, req.Filename, client.Handle(), req.ReplyTo, req.ReplyPart)
	if err != nil {
		s.log.Err(err).Str("to", req.To).Str("filename", req.Filename).Msg("Failed to send attachment")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to send attachment: %v", err), "SEND_FAILED")
		return
	}

	s.log.Info().Str("uuid", uuid).Str("to", req.To).Str("filename", req.Filename).Msg("Attachment sent via API")
	writeJSON(w, http.StatusOK, SendResponse{UUID: uuid, Status: "sent"})
}

func (s *Server) handleReact(w http.ResponseWriter, r *http.Request) {
	var req ReactRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.To == "" || req.TargetUUID == "" || req.Reaction == "" {
		writeError(w, http.StatusBadRequest, "to, target_uuid, and reaction are required", "INVALID_REQUEST")
		return
	}

	var reactionCode uint32
	var emoji *string

	if req.Reaction == "emoji" {
		if req.Emoji == nil || *req.Emoji == "" {
			writeError(w, http.StatusBadRequest, "emoji field is required when reaction is 'emoji'", "INVALID_REQUEST")
			return
		}
		reactionCode = 6
		emoji = req.Emoji
	} else {
		code, err := reactionNameToCode(req.Reaction)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
			return
		}
		reactionCode = code
	}

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	conv := s.buildConversation(client, req.To, req.IsSMS)
	uuid, err := client.SendTapback(conv, req.TargetUUID, req.TargetPart, reactionCode, emoji, req.Remove, client.Handle())
	if err != nil {
		s.log.Err(err).Str("to", req.To).Str("target", req.TargetUUID).Msg("Failed to send reaction")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to send reaction: %v", err), "SEND_FAILED")
		return
	}

	s.log.Info().Str("uuid", uuid).Str("to", req.To).Str("reaction", req.Reaction).Msg("Reaction sent via API")
	writeJSON(w, http.StatusOK, SendResponse{UUID: uuid, Status: "sent"})
}

func (s *Server) handleEdit(w http.ResponseWriter, r *http.Request) {
	var req EditRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.To == "" || req.TargetUUID == "" || req.NewText == "" {
		writeError(w, http.StatusBadRequest, "to, target_uuid, and new_text are required", "INVALID_REQUEST")
		return
	}

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	conv := s.buildConversation(client, req.To, req.IsSMS)
	uuid, err := client.SendEdit(conv, req.TargetUUID, 0, req.NewText, client.Handle())
	if err != nil {
		s.log.Err(err).Str("to", req.To).Str("target", req.TargetUUID).Msg("Failed to send edit")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to send edit: %v", err), "SEND_FAILED")
		return
	}

	s.log.Info().Str("uuid", uuid).Str("to", req.To).Msg("Edit sent via API")
	writeJSON(w, http.StatusOK, SendResponse{UUID: uuid, Status: "sent"})
}

func (s *Server) handleUnsend(w http.ResponseWriter, r *http.Request) {
	var req UnsendRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.To == "" || req.TargetUUID == "" {
		writeError(w, http.StatusBadRequest, "to and target_uuid are required", "INVALID_REQUEST")
		return
	}

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	conv := s.buildConversation(client, req.To, req.IsSMS)
	uuid, err := client.SendUnsend(conv, req.TargetUUID, 0, client.Handle())
	if err != nil {
		s.log.Err(err).Str("to", req.To).Str("target", req.TargetUUID).Msg("Failed to send unsend")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to unsend: %v", err), "SEND_FAILED")
		return
	}

	s.log.Info().Str("uuid", uuid).Str("to", req.To).Msg("Unsend sent via API")
	writeJSON(w, http.StatusOK, SendResponse{UUID: uuid, Status: "sent"})
}

func (s *Server) handleTyping(w http.ResponseWriter, r *http.Request) {
	var req TypingRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.To == "" {
		writeError(w, http.StatusBadRequest, "to is required", "INVALID_REQUEST")
		return
	}

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	conv := s.buildConversation(client, req.To, req.IsSMS)
	err = client.SendTyping(conv, req.Typing, client.Handle())
	if err != nil {
		s.log.Err(err).Str("to", req.To).Msg("Failed to send typing indicator")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to send typing: %v", err), "SEND_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, OkResponse{Status: "ok"})
}

func (s *Server) handleReadReceipt(w http.ResponseWriter, r *http.Request) {
	var req ReadReceiptRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.To == "" {
		writeError(w, http.StatusBadRequest, "to is required", "INVALID_REQUEST")
		return
	}

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	conv := s.buildConversation(client, req.To, req.IsSMS)
	err = client.SendReadReceipt(conv, client.Handle(), req.ForUUID)
	if err != nil {
		s.log.Err(err).Str("to", req.To).Msg("Failed to send read receipt")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to send read receipt: %v", err), "SEND_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, OkResponse{Status: "ok"})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "no active session to disconnect", "NOT_CONNECTED")
		return
	}

	handle := client.Handle()
	client.Disconnect()

	s.log.Info().Str("handle", handle).Msg("Client disconnected via API")
	writeJSON(w, http.StatusOK, OkResponse{Status: "disconnected"})
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg, code string) {
	writeJSON(w, status, ErrorResponse{Error: msg, Code: code})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json", "INVALID_REQUEST")
		return false
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err), "INVALID_REQUEST")
		return false
	}
	return true
}
