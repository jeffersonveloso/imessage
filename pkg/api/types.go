package api

// --- HTTP Request types ---

type SendRequest struct {
	To        string  `json:"to"`
	Text      string  `json:"text"`
	IsSMS     bool    `json:"is_sms,omitempty"`
	ReplyTo   *string `json:"reply_to,omitempty"`
	ReplyPart *string `json:"reply_part,omitempty"`
	EffectID  *string `json:"effect_id,omitempty"`
	Subject   *string `json:"subject,omitempty"`
}

type SendMediaRequest struct {
	To        string  `json:"to"`
	Data      string  `json:"data"` // base64-encoded
	MimeType  string  `json:"mime_type"`
	Filename  string  `json:"filename"`
	IsSMS     bool    `json:"is_sms,omitempty"`
	ReplyTo   *string `json:"reply_to,omitempty"`
	ReplyPart *string `json:"reply_part,omitempty"`
	EffectID  *string `json:"effect_id,omitempty"`
	Subject   *string `json:"subject,omitempty"`
	Caption   *string `json:"caption,omitempty"`
}

type ReactRequest struct {
	To         string  `json:"to"`
	TargetUUID string  `json:"target_uuid"`
	TargetPart uint64  `json:"target_part"`
	Reaction   string  `json:"reaction"` // heart, like, dislike, laugh, emphasize, question, emoji
	Emoji      *string `json:"emoji,omitempty"`
	Remove     bool    `json:"remove,omitempty"`
	IsSMS      bool    `json:"is_sms,omitempty"`
}

type EditRequest struct {
	To         string `json:"to"`
	TargetUUID string `json:"target_uuid"`
	NewText    string `json:"new_text"`
	IsSMS      bool   `json:"is_sms,omitempty"`
}

type UnsendRequest struct {
	To         string `json:"to"`
	TargetUUID string `json:"target_uuid"`
	IsSMS      bool   `json:"is_sms,omitempty"`
}

type TypingRequest struct {
	To     string `json:"to"`
	Typing bool   `json:"typing"`
	IsSMS  bool   `json:"is_sms,omitempty"`
}

type ReadReceiptRequest struct {
	To      string  `json:"to"`
	ForUUID *string `json:"for_uuid,omitempty"`
	IsSMS   bool    `json:"is_sms,omitempty"`
}

type ValidateRequest struct {
	Targets []string `json:"targets"`
}

// --- HTTP Response types ---

type SendResponse struct {
	UUID   string `json:"uuid"`
	Status string `json:"status"`
}

type StatusResponse struct {
	Connected  bool     `json:"connected"`
	Handle     string   `json:"handle,omitempty"`
	AllHandles []string `json:"all_handles,omitempty"`
}

type HandlesResponse struct {
	Handles []string `json:"handles"`
}

type ValidateResponse struct {
	Valid   []string `json:"valid"`
	Invalid []string `json:"invalid"`
}

type OkResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// --- Login request/response types ---

type LoginStartRequest struct {
	Flow string `json:"flow"`
}

type LoginStepRequest struct {
	SessionID string            `json:"session_id"`
	Input     map[string]string `json:"input"`
}

type LoginFlowsResponse struct {
	Flows []LoginFlowInfo `json:"flows"`
}

type LoginStepResponse struct {
	SessionID    string         `json:"session_id"`
	StepID       string         `json:"step_id"`
	Instructions string         `json:"instructions,omitempty"`
	Fields       []LoginField   `json:"fields,omitempty"`
	Complete     *LoginComplete `json:"complete,omitempty"`
}

// --- Webhook event types ---
//
// Every webhook event has a category that groups related event types:
//
//   connection      — connected, disconnected
//   message         — message (new incoming message)
//   message_update  — edit, unsend, reaction (modifications to existing messages)
//   message_receipt — typing, read_receipt, delivered (delivery/read indicators)

type WebhookEvent struct {
	Type      string `json:"type"`      // message, reaction, typing, read_receipt, delivered, edit, unsend, connected, disconnected
	Category  string `json:"category"`  // connection, message, message_update, message_receipt
	Timestamp uint64 `json:"timestamp"`
	Data      any    `json:"data"`
}

// --- Webhook categories ---

const (
	WebhookCategoryConnection     = "connection"
	WebhookCategoryMessage        = "message"
	WebhookCategoryMessageUpdate  = "message_update"
	WebhookCategoryMessageReceipt = "message_receipt"
)

type WebhookMessageData struct {
	UUID          string   `json:"uuid"`
	Sender        string   `json:"sender"`
	Text          *string  `json:"text,omitempty"`
	Subject       *string  `json:"subject,omitempty"`
	Participants  []string `json:"participants"`
	GroupName     *string  `json:"group_name,omitempty"`
	IsGroup       bool     `json:"is_group"`
	IsSMS         bool     `json:"is_sms"`
	ReplyTo       *string  `json:"reply_to,omitempty"`
	HasAttachment bool     `json:"has_attachment"`
}

type WebhookReactionData struct {
	UUID         string   `json:"uuid"`
	Sender       string   `json:"sender"`
	TargetUUID   string   `json:"target_uuid"`
	TargetPart   *uint64  `json:"target_part,omitempty"`
	Reaction     *uint32  `json:"reaction,omitempty"`
	Emoji        *string  `json:"emoji,omitempty"`
	Remove       bool     `json:"remove"`
	Participants []string `json:"participants"`
	GroupName    *string  `json:"group_name,omitempty"`
	IsGroup      bool     `json:"is_group"`
	IsSMS        bool     `json:"is_sms"`
}

type WebhookTypingData struct {
	Sender       string   `json:"sender"`
	Typing       bool     `json:"typing"`
	Participants []string `json:"participants"`
	GroupName    *string  `json:"group_name,omitempty"`
	IsGroup      bool     `json:"is_group"`
	IsSMS        bool     `json:"is_sms"`
}

type WebhookReadReceiptData struct {
	Sender       string   `json:"sender"`
	Participants []string `json:"participants"`
	GroupName    *string  `json:"group_name,omitempty"`
	IsGroup      bool     `json:"is_group"`
	IsSMS        bool     `json:"is_sms"`
}

type WebhookDeliveredData struct {
	Sender       string   `json:"sender"`
	Participants []string `json:"participants"`
	GroupName    *string  `json:"group_name,omitempty"`
	IsGroup      bool     `json:"is_group"`
	IsSMS        bool     `json:"is_sms"`
}

type WebhookEditData struct {
	UUID         string   `json:"uuid"`
	Sender       string   `json:"sender"`
	TargetUUID   string   `json:"target_uuid"`
	NewText      *string  `json:"new_text,omitempty"`
	Participants []string `json:"participants"`
	GroupName    *string  `json:"group_name,omitempty"`
	IsGroup      bool     `json:"is_group"`
	IsSMS        bool     `json:"is_sms"`
}

type WebhookUnsendData struct {
	UUID         string   `json:"uuid"`
	Sender       string   `json:"sender"`
	TargetUUID   string   `json:"target_uuid"`
	Participants []string `json:"participants"`
	GroupName    *string  `json:"group_name,omitempty"`
	IsGroup      bool     `json:"is_group"`
	IsSMS        bool     `json:"is_sms"`
}

type WebhookConnectionData struct {
	Handle     string   `json:"handle"`
	AllHandles []string `json:"all_handles"`
}
