package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// WebhookDispatcher sends event notifications to an external HTTP endpoint.
type WebhookDispatcher struct {
	url        string
	secret     string
	instanceID string
	client     *http.Client
	log        zerolog.Logger
}

// NewWebhookDispatcher creates a dispatcher that POSTs JSON events to url.
// If secret is non-empty, each request includes an X-Webhook-Signature header
// containing the HMAC-SHA256 hex digest of the body.
func NewWebhookDispatcher(url, secret string, log zerolog.Logger) *WebhookDispatcher {
	return &WebhookDispatcher{
		url:    url,
		secret: secret,
		client: &http.Client{Timeout: 10 * time.Second},
		log:    log.With().Str("component", "webhook").Logger(),
	}
}

// SetInstanceID sets the caller-provided instance ID that will be included
// in every dispatched webhook event.
func (w *WebhookDispatcher) SetInstanceID(id string) {
	w.instanceID = id
}

// GetInstanceID returns the current instance ID.
func (w *WebhookDispatcher) GetInstanceID() string {
	return w.instanceID
}

// Dispatch sends the event asynchronously. It never blocks the caller.
func (w *WebhookDispatcher) Dispatch(event WebhookEvent) {
	event.InstanceID = w.instanceID
	go w.deliver(event)
}

// deliver serialises the event and POSTs it with retries.
func (w *WebhookDispatcher) deliver(event WebhookEvent) {
	body, err := json.Marshal(event)
	if err != nil {
		w.log.Err(err).Str("type", event.Type).Msg("Failed to marshal webhook event")
		return
	}

	maxAttempts := 3
	backoff := 1 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := w.post(body, event.Type); err != nil {
			w.log.Warn().
				Err(err).
				Str("type", event.Type).
				Int("attempt", attempt).
				Msg("Webhook delivery failed")

			if attempt < maxAttempts {
				time.Sleep(backoff)
				backoff *= 2
			}
			continue
		}
		w.log.Debug().Str("type", event.Type).Msg("Webhook delivered")
		return
	}

	w.log.Error().Str("type", event.Type).Msg("Webhook delivery failed after all retries")
}

// post performs a single POST request.
func (w *WebhookDispatcher) post(body []byte, eventType string) error {
	req, err := http.NewRequest(http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Event", eventType)

	if w.secret != "" {
		mac := hmac.New(sha256.New, []byte(w.secret))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Webhook-Signature", sig)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return &webhookHTTPError{StatusCode: resp.StatusCode}
	}
	return nil
}

type webhookHTTPError struct {
	StatusCode int
}

func (e *webhookHTTPError) Error() string {
	return http.StatusText(e.StatusCode)
}
