package connector

import (
	"github.com/lrhodin/imessage/pkg/api"
	"github.com/lrhodin/imessage/pkg/rustpushgo"
)

// imClientAdapter wraps the internal *IMClient so it satisfies
// the api.IMClient interface without leaking connector internals.
type imClientAdapter struct {
	client *IMClient
}

var _ api.IMClient = (*imClientAdapter)(nil)

func (a *imClientAdapter) Handle() string        { return a.client.handle }
func (a *imClientAdapter) AllHandles() []string   { return a.client.allHandles }
func (a *imClientAdapter) IsLoggedIn() bool       { return a.client.IsLoggedIn() }
func (a *imClientAdapter) Disconnect()            { a.client.Disconnect() }
func (a *imClientAdapter) NormalizeIdentifier(id string) string {
	return normalizeIdentifierForPortalID(id)
}
func (a *imClientAdapter) MimeToUTI(mime string) string { return mimeToUTI(mime) }

func (a *imClientAdapter) SendMessage(conv rustpushgo.WrappedConversation, text, handle string, replyGuid, replyPart, effect, subject *string) (string, error) {
	return a.client.client.SendMessage(conv, text, handle, replyGuid, replyPart, effect, subject)
}

func (a *imClientAdapter) SendAttachment(conv rustpushgo.WrappedConversation, data []byte, mime, uti, filename, handle string, replyGuid, replyPart, effect, subject, caption *string) (string, error) {
	return a.client.client.SendAttachment(conv, data, mime, uti, filename, handle, replyGuid, replyPart, effect, subject, caption)
}

func (a *imClientAdapter) SendTapback(conv rustpushgo.WrappedConversation, targetUuid string, targetPart uint64, reaction uint32, emoji *string, remove bool, handle string) (string, error) {
	return a.client.client.SendTapback(conv, targetUuid, targetPart, reaction, emoji, remove, handle)
}

func (a *imClientAdapter) SendEdit(conv rustpushgo.WrappedConversation, targetUuid string, editPart uint64, newText, handle string) (string, error) {
	return a.client.client.SendEdit(conv, targetUuid, editPart, newText, handle)
}

func (a *imClientAdapter) SendUnsend(conv rustpushgo.WrappedConversation, targetUuid string, editPart uint64, handle string) (string, error) {
	return a.client.client.SendUnsend(conv, targetUuid, editPart, handle)
}

func (a *imClientAdapter) SendTyping(conv rustpushgo.WrappedConversation, typing bool, handle string) error {
	return a.client.client.SendTyping(conv, typing, handle)
}

func (a *imClientAdapter) SendReadReceipt(conv rustpushgo.WrappedConversation, handle string, forUuid *string) error {
	return a.client.client.SendReadReceipt(conv, handle, forUuid)
}

func (a *imClientAdapter) ValidateTargets(targets []string, handle string) []string {
	return a.client.client.ValidateTargets(targets, handle)
}
