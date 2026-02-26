package connector

import (
	"context"
	"fmt"
	"sort"
	"strings"

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

func (a *imClientAdapter) BuildGroupConversation(participants []string, groupName *string) rustpushgo.WrappedConversation {
	// Normalize and sort participants to compute portal ID (same as makePortalKey group branch).
	sorted := make([]string, 0, len(participants))
	for _, p := range participants {
		normalized := normalizeIdentifierForPortalID(p)
		if normalized == "" || a.client.isMyHandle(normalized) {
			continue
		}
		sorted = append(sorted, normalized)
	}
	sorted = append(sorted, normalizeIdentifierForPortalID(a.client.handle))
	sort.Strings(sorted)
	deduped := sorted[:0]
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			deduped = append(deduped, s)
		}
	}
	portalID := strings.Join(deduped, ",")

	// Look up the persistent group UUID (sender_guid) so outbound messages
	// route to the correct iMessage group thread.
	var senderGuid *string
	a.client.imGroupGuidsMu.RLock()
	if guid, ok := a.client.imGroupGuids[portalID]; ok {
		senderGuid = &guid
	}
	a.client.imGroupGuidsMu.RUnlock()

	// Also check gid:-prefixed portal IDs.
	if senderGuid == nil {
		a.client.imGroupGuidsMu.RLock()
		for pid, guid := range a.client.imGroupGuids {
			if strings.HasPrefix(pid, "gid:") {
				// Check if the participants match by looking up cached participants.
				a.client.imGroupParticipantsMu.RLock()
				cachedParts := a.client.imGroupParticipants[pid]
				a.client.imGroupParticipantsMu.RUnlock()
				if participantsMatchPortalID(cachedParts, portalID) {
					senderGuid = &guid
					break
				}
			}
		}
		a.client.imGroupGuidsMu.RUnlock()
	}

	return rustpushgo.WrappedConversation{
		Participants: deduped,
		GroupName:    groupName,
		SenderGuid:  senderGuid,
	}
}

// participantsMatchPortalID checks if cached participants form the same portal ID.
func participantsMatchPortalID(cached []string, portalID string) bool {
	if len(cached) == 0 {
		return false
	}
	sorted := make([]string, len(cached))
	copy(sorted, cached)
	sort.Strings(sorted)
	deduped := sorted[:0]
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			deduped = append(deduped, s)
		}
	}
	return strings.Join(deduped, ",") == portalID
}

func (a *imClientAdapter) GetChatInfo(participants []string) (*api.ChatInfoResponse, error) {
	// Normalize participants to compute portal ID.
	sorted := make([]string, 0, len(participants))
	for _, p := range participants {
		normalized := normalizeIdentifierForPortalID(p)
		if normalized != "" {
			sorted = append(sorted, normalized)
		}
	}
	sort.Strings(sorted)
	deduped := sorted[:0]
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			deduped = append(deduped, s)
		}
	}
	portalID := strings.Join(deduped, ",")
	isGroup := len(deduped) > 2

	// Look up cached group name.
	var groupName *string
	a.client.imGroupNamesMu.RLock()
	if name, ok := a.client.imGroupNames[portalID]; ok {
		groupName = &name
	}
	a.client.imGroupNamesMu.RUnlock()

	// Also check gid:-prefixed portal IDs for group name.
	if groupName == nil {
		a.client.imGroupNamesMu.RLock()
		for pid, name := range a.client.imGroupNames {
			if strings.HasPrefix(pid, "gid:") {
				a.client.imGroupParticipantsMu.RLock()
				cachedParts := a.client.imGroupParticipants[pid]
				a.client.imGroupParticipantsMu.RUnlock()
				if participantsMatchPortalID(cachedParts, portalID) {
					nameCopy := name
					groupName = &nameCopy
					break
				}
			}
		}
		a.client.imGroupNamesMu.RUnlock()
	}

	// Look up cached participants (may have more members than the query).
	fullParticipants := deduped
	a.client.imGroupParticipantsMu.RLock()
	if cached, ok := a.client.imGroupParticipants[portalID]; ok && len(cached) > 0 {
		fullParticipants = cached
	}
	a.client.imGroupParticipantsMu.RUnlock()

	return &api.ChatInfoResponse{
		Participants: fullParticipants,
		GroupName:    groupName,
		IsGroup:      isGroup,
	}, nil
}

func (a *imClientAdapter) GetContact(identifier string) (*api.ContactResponse, error) {
	normalized := normalizeIdentifierForPortalID(identifier)
	if normalized == "" {
		return nil, fmt.Errorf("invalid identifier")
	}

	displayName := a.client.resolveContactDisplayname(normalized)

	resp := &api.ContactResponse{
		Identifier:  normalized,
		DisplayName: displayName,
	}

	// If contacts are available, enrich with phone/email info.
	if a.client.contacts != nil {
		localID := stripIdentifierPrefix(normalized)
		if contact, _ := a.client.contacts.GetContactInfo(localID); contact != nil {
			resp.Phones = contact.Phones
			resp.Emails = contact.Emails
		}
	}

	return resp, nil
}

func (a *imClientAdapter) DeleteChat(participants []string, groupName *string) error {
	if a.client.cloudStore == nil {
		return fmt.Errorf("cloud store not available")
	}

	// Normalize participants to compute portal ID (same as makePortalKey).
	sorted := make([]string, 0, len(participants))
	for _, p := range participants {
		normalized := normalizeIdentifierForPortalID(p)
		if normalized == "" || a.client.isMyHandle(normalized) {
			continue
		}
		sorted = append(sorted, normalized)
	}
	sorted = append(sorted, normalizeIdentifierForPortalID(a.client.handle))
	sort.Strings(sorted)
	deduped := sorted[:0]
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			deduped = append(deduped, s)
		}
	}
	portalID := strings.Join(deduped, ",")

	ctx := context.Background()
	return a.client.cloudStore.deleteLocalChatByPortalID(ctx, portalID)
}

func (a *imClientAdapter) SendDeliveryReceipt(conv rustpushgo.WrappedConversation, handle string) error {
	return a.client.client.SendDeliveryReceipt(conv, handle)
}

func (a *imClientAdapter) SetHandle(handle string) error {
	// Validate the handle is in the list of available handles.
	found := false
	for _, h := range a.client.allHandles {
		if h == handle {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("handle %q not found in available handles", handle)
	}

	a.client.handle = handle

	// Persist to metadata.
	meta, ok := a.client.UserLogin.Metadata.(*UserLoginMetadata)
	if !ok {
		return fmt.Errorf("unexpected metadata type")
	}
	meta.PreferredHandle = handle
	if err := a.client.UserLogin.Save(context.Background()); err != nil {
		return fmt.Errorf("failed to persist metadata: %w", err)
	}
	return nil
}

func (a *imClientAdapter) GetStatusInfo() (contactsCount *int, contactsReady *bool, cloudSyncDone *bool) {
	if a.client.contacts != nil {
		count := len(a.client.contacts.GetAllContacts())
		contactsCount = &count
	}

	a.client.contactsReadyLock.RLock()
	ready := a.client.contactsReady
	a.client.contactsReadyLock.RUnlock()
	contactsReady = &ready

	a.client.cloudSyncDoneLock.RLock()
	syncDone := a.client.cloudSyncDone
	a.client.cloudSyncDoneLock.RUnlock()
	cloudSyncDone = &syncDone

	return
}

func (a *imClientAdapter) ClearInstanceID() error {
	meta, ok := a.client.UserLogin.Metadata.(*UserLoginMetadata)
	if !ok {
		return fmt.Errorf("unexpected metadata type")
	}
	if meta.InstanceID == "" {
		return nil
	}
	meta.InstanceID = ""
	if err := a.client.UserLogin.Save(context.Background()); err != nil {
		return fmt.Errorf("failed to persist metadata: %w", err)
	}
	// Also update the backup session file.
	saveSessionState(a.client.UserLogin.Log.With().Str("action", "clear_instance_id").Logger(), PersistedSessionState{
		IDSIdentity:              meta.IDSIdentity,
		APSState:                 meta.APSState,
		IDSUsers:                 meta.IDSUsers,
		PreferredHandle:          meta.PreferredHandle,
		Platform:                 meta.Platform,
		HardwareKey:              meta.HardwareKey,
		DeviceID:                 meta.DeviceID,
		AccountUsername:          meta.AccountUsername,
		AccountHashedPasswordHex: meta.AccountHashedPasswordHex,
		AccountPET:               meta.AccountPET,
		AccountADSID:             meta.AccountADSID,
		AccountDSID:              meta.AccountDSID,
		AccountSPDBase64:         meta.AccountSPDBase64,
		MmeDelegateJSON:          meta.MmeDelegateJSON,
		InstanceID:               "", // cleared
	})
	return nil
}
