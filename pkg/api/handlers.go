package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/lrhodin/imessage/pkg/rustpushgo"
)

const maxMediaSize = 100 << 20 // 100 MB

// buildConversationFromRequest validates the target fields and builds a WrappedConversation.
// Exactly one of 'to' (DM) or 'participants' (group) must be provided.
func (s *Server) buildConversationFromRequest(client IMClient, to string, participants []string, groupName *string, isSMS bool) (rustpushgo.WrappedConversation, error) {
	hasDM := to != ""
	hasGroup := len(participants) > 0
	if hasDM == hasGroup {
		return rustpushgo.WrappedConversation{}, fmt.Errorf("provide either 'to' (DM) or 'participants' (group), not both")
	}
	if hasGroup {
		conv := client.BuildGroupConversation(participants, groupName)
		conv.IsSms = isSMS
		return conv, nil
	}
	normalized := client.NormalizeIdentifier(to)
	return rustpushgo.WrappedConversation{
		Participants: []string{client.Handle(), normalized},
		IsSms:        isSMS,
	}, nil
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
	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "text is required", "INVALID_REQUEST")
		return
	}

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	conv, err := s.buildConversationFromRequest(client, req.To, req.Participants, req.GroupName, req.IsSMS)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
		return
	}
	uuid, err := client.SendMessage(conv, req.Text, client.Handle(), req.ReplyTo, req.ReplyPart, req.EffectID, req.Subject)
	if err != nil {
		s.log.Err(err).Str("to", req.To).Msg("Failed to send message")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to send: %v", err), "SEND_FAILED")
		return
	}

	s.log.Info().Str("uuid", uuid).Str("to", req.To).Msg("Message sent via API")
	writeJSON(w, http.StatusOK, SendResponse{UUID: uuid, Status: "sent"})
}

func (s *Server) handleSendMedia(w http.ResponseWriter, r *http.Request) {
	var (
		req  SendMediaRequest
		data []byte
		err  error
	)

	ct := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "multipart/form-data"):
		data, req, err = parseMultipartMedia(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
			return
		}

	case ct == "application/json":
		if !decodeJSON(w, r, &req) {
			return
		}

		switch {
		case req.Data != "" && req.URL != "":
			writeError(w, http.StatusBadRequest, "provide either data or url, not both", "INVALID_REQUEST")
			return
		case req.URL != "":
			var fetchedMime, fetchedFilename string
			data, fetchedMime, fetchedFilename, err = fetchMediaFromURL(r.Context(), req.URL)
			if err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to fetch URL: %v", err), "INVALID_REQUEST")
				return
			}
			if req.MimeType == "" {
				req.MimeType = fetchedMime
			}
			if req.Filename == "" {
				req.Filename = fetchedFilename
			}
		case req.Data != "":
			data, err = base64.StdEncoding.DecodeString(req.Data)
			if err != nil {
				writeError(w, http.StatusBadRequest, "data must be valid base64", "INVALID_REQUEST")
				return
			}
		default:
			writeError(w, http.StatusBadRequest, "either data or url is required", "INVALID_REQUEST")
			return
		}

	default:
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json or multipart/form-data", "INVALID_REQUEST")
		return
	}

	if req.MimeType == "" || req.Filename == "" {
		writeError(w, http.StatusBadRequest, "mime_type and filename are required", "INVALID_REQUEST")
		return
	}

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	conv, err := s.buildConversationFromRequest(client, req.To, req.Participants, req.GroupName, req.IsSMS)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
		return
	}
	uti := client.MimeToUTI(req.MimeType)
	uuid, err := client.SendAttachment(conv, data, req.MimeType, uti, req.Filename, client.Handle(), req.ReplyTo, req.ReplyPart, req.EffectID, req.Subject, req.Caption)
	if err != nil {
		s.log.Err(err).Str("to", req.To).Str("filename", req.Filename).Msg("Failed to send attachment")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to send attachment: %v", err), "SEND_FAILED")
		return
	}

	s.log.Info().Str("uuid", uuid).Str("to", req.To).Str("filename", req.Filename).Msg("Attachment sent via API")
	writeJSON(w, http.StatusOK, SendResponse{UUID: uuid, Status: "sent"})
}

// parseMultipartMedia extracts file data and metadata from a multipart/form-data request.
func parseMultipartMedia(r *http.Request) ([]byte, SendMediaRequest, error) {
	if err := r.ParseMultipartForm(maxMediaSize); err != nil {
		return nil, SendMediaRequest{}, fmt.Errorf("failed to parse multipart form: %v", err)
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, SendMediaRequest{}, fmt.Errorf("file field is required: %v", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxMediaSize+1))
	if err != nil {
		return nil, SendMediaRequest{}, fmt.Errorf("failed to read file: %v", err)
	}
	if int64(len(data)) > maxMediaSize {
		return nil, SendMediaRequest{}, fmt.Errorf("file exceeds maximum size of 100 MB")
	}

	req := SendMediaRequest{
		To:       r.FormValue("to"),
		MimeType: r.FormValue("mime_type"),
		Filename: r.FormValue("filename"),
		IsSMS:    r.FormValue("is_sms") == "true",
	}
	if v := r.FormValue("participants"); v != "" {
		req.Participants = strings.Split(v, ",")
	}
	if v := r.FormValue("group_name"); v != "" {
		req.GroupName = &v
	}

	// Auto-detect mime type from the file header if not provided
	if req.MimeType == "" {
		req.MimeType = header.Header.Get("Content-Type")
	}
	if req.MimeType == "" {
		req.MimeType = http.DetectContentType(data)
	}

	// Auto-detect filename from the upload header if not provided
	if req.Filename == "" {
		req.Filename = header.Filename
	}

	// Optional pointer fields from form values
	if v := r.FormValue("reply_to"); v != "" {
		req.ReplyTo = &v
	}
	if v := r.FormValue("reply_part"); v != "" {
		req.ReplyPart = &v
	}
	if v := r.FormValue("effect_id"); v != "" {
		req.EffectID = &v
	}
	if v := r.FormValue("subject"); v != "" {
		req.Subject = &v
	}
	if v := r.FormValue("caption"); v != "" {
		req.Caption = &v
	}

	return data, req, nil
}

// fetchMediaFromURL downloads a file from the given URL and returns its data, mime type, and filename.
func fetchMediaFromURL(ctx context.Context, rawURL string) ([]byte, string, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid URL: %v", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, "", "", fmt.Errorf("URL scheme must be http or https")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("URL returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMediaSize+1))
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to read response: %v", err)
	}
	if int64(len(data)) > maxMediaSize {
		return nil, "", "", fmt.Errorf("file exceeds maximum size of 100 MB")
	}

	// Detect mime type from Content-Type header
	mimeType := ""
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		mimeType, _, _ = mime.ParseMediaType(ct)
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}

	// Detect filename from Content-Disposition header or URL path
	filename := ""
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		_, params, _ := mime.ParseMediaType(cd)
		filename = params["filename"]
	}
	if filename == "" {
		filename = path.Base(parsed.Path)
		if filename == "." || filename == "/" {
			filename = "download"
		}
	}

	return data, mimeType, filename, nil
}

func (s *Server) handleReact(w http.ResponseWriter, r *http.Request) {
	var req ReactRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.TargetUUID == "" || req.Reaction == "" {
		writeError(w, http.StatusBadRequest, "target_uuid and reaction are required", "INVALID_REQUEST")
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

	conv, err := s.buildConversationFromRequest(client, req.To, req.Participants, req.GroupName, req.IsSMS)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
		return
	}
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
	if req.TargetUUID == "" || req.NewText == "" {
		writeError(w, http.StatusBadRequest, "target_uuid and new_text are required", "INVALID_REQUEST")
		return
	}

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	conv, err := s.buildConversationFromRequest(client, req.To, req.Participants, req.GroupName, req.IsSMS)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
		return
	}
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
	if req.TargetUUID == "" {
		writeError(w, http.StatusBadRequest, "target_uuid is required", "INVALID_REQUEST")
		return
	}

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	conv, err := s.buildConversationFromRequest(client, req.To, req.Participants, req.GroupName, req.IsSMS)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
		return
	}
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

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	conv, err := s.buildConversationFromRequest(client, req.To, req.Participants, req.GroupName, req.IsSMS)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
		return
	}
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

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	conv, err := s.buildConversationFromRequest(client, req.To, req.Participants, req.GroupName, req.IsSMS)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
		return
	}
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

func (s *Server) handleChatInfo(w http.ResponseWriter, r *http.Request) {
	participantsParam := r.URL.Query().Get("participants")
	if participantsParam == "" {
		writeError(w, http.StatusBadRequest, "participants query parameter is required", "INVALID_REQUEST")
		return
	}
	participants := strings.Split(participantsParam, ",")

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	info, err := client.GetChatInfo(participants)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get chat info: %v", err), "LOOKUP_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleContact(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id query parameter is required", "INVALID_REQUEST")
		return
	}

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	contact, err := client.GetContact(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get contact: %v", err), "LOOKUP_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, contact)
}

func (s *Server) handleDeleteChat(w http.ResponseWriter, r *http.Request) {
	var req DeleteChatRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Participants) == 0 {
		writeError(w, http.StatusBadRequest, "participants is required", "INVALID_REQUEST")
		return
	}

	client, err := s.provider.GetActiveClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "NOT_CONNECTED")
		return
	}

	err = client.DeleteChat(req.Participants, req.GroupName)
	if err != nil {
		s.log.Err(err).Msg("Failed to delete chat")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete chat: %v", err), "DELETE_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, OkResponse{Status: "deleted"})
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
