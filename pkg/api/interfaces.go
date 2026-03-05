package api

import (
	"context"

	"github.com/lrhodin/imessage/pkg/rustpushgo"
)

// IMClientProvider locates the active iMessage client.
// Implemented by the connector package.
type IMClientProvider interface {
	GetActiveClient() (IMClient, error)
	// GetAnyClient returns any cached login client, even if disconnected.
	// Used by logout to clean up sessions that exist in the DB but aren't connected.
	GetAnyClient() (IMClient, error)
	Reconnect(ctx context.Context) error
}

// LoginProvider exposes the bridge's login flows over HTTP.
// Implemented by the connector package using bridgev2 LoginProcess.
type LoginProvider interface {
	GetLoginFlows() []LoginFlowInfo
	StartLogin(ctx context.Context, flowID string) (LoginSession, error)
}

// LoginSession represents an in-progress multi-step login.
type LoginSession interface {
	StepID() string
	Instructions() string
	Fields() []LoginField
	Submit(ctx context.Context, input map[string]string) (LoginSession, *LoginComplete, error)
}

// LoginFlowInfo describes an available login flow.
type LoginFlowInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// LoginField describes one input field in a login step.
type LoginField struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type,omitempty"`
	Options []string `json:"options,omitempty"`
}

// LoginComplete is returned when the login flow finishes successfully.
type LoginComplete struct {
	LoginID string `json:"login_id"`
	Message string `json:"message"`
}

// IMClient abstracts iMessage send operations.
// The connector wraps its internal client to satisfy this interface,
// keeping pkg/api free of connector imports.
type IMClient interface {
	Handle() string
	AllHandles() []string
	IsLoggedIn() bool
	Disconnect()
	CleanupSession()
	NormalizeIdentifier(identifier string) string
	MimeToUTI(mime string) string

	BuildGroupConversation(participants []string, groupName *string) rustpushgo.WrappedConversation
	GetChatInfo(participants []string) (*ChatInfoResponse, error)
	GetContact(identifier string) (*ContactResponse, error)
	DeleteChat(participants []string, groupName *string) error
	SendMoveToRecycleBin(participants []string, groupName *string, isSMS bool) error
	ClearInstanceID() error
	DeleteLogin(ctx context.Context) error
	SetHandle(handle string) error
	GetStatusInfo() (contactsCount *int, contactsReady *bool, cloudSyncDone *bool)
	GetAllChats() []ChatListEntry

	SendMessage(conv rustpushgo.WrappedConversation, text, handle string, replyGuid, replyPart, effect, subject *string) (string, error)
	SendAttachment(conv rustpushgo.WrappedConversation, data []byte, mime, uti, filename, handle string, replyGuid, replyPart, effect, subject, caption *string) (string, error)
	SendTapback(conv rustpushgo.WrappedConversation, targetUuid string, targetPart uint64, reaction uint32, emoji *string, remove bool, handle string) (string, error)
	SendEdit(conv rustpushgo.WrappedConversation, targetUuid string, editPart uint64, newText, handle string) (string, error)
	SendUnsend(conv rustpushgo.WrappedConversation, targetUuid string, editPart uint64, handle string) (string, error)
	SendTyping(conv rustpushgo.WrappedConversation, typing bool, handle string) error
	SendReadReceipt(conv rustpushgo.WrappedConversation, handle string, forUuid *string) error
	SendDeliveryReceipt(conv rustpushgo.WrappedConversation, handle string) error
	ValidateTargets(targets []string, handle string) []string
}
