package sapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultWebhookTolerance is how far a signed timestamp may be from now.
const DefaultWebhookTolerance = 300 * time.Second

// TenantWebhookTolerance is the tenant scheme's default: tenant webhooks sign the time the event was dispatched and
// keep it on every retry (5 tries over about 3 hours). Pair it with de-duplication on the event id.
const TenantWebhookTolerance = 12600 * time.Second

// WebhookSchemes are the signature schemes Superroute's senders use.
var WebhookSchemes = []string{"standard", "tenant", "partner_locker", "tpd", "loading_plan", "open_platform", "dataset"}

const typePrefix = "ca.superroute."

// WebhookVerificationError is a webhook that failed verification. Code is one of unknown_scheme, invalid_secret,
// missing_header, invalid_timestamp, timestamp_out_of_tolerance, invalid_signature, invalid_payload. Answer it with
// 400 (or 401) and do not act on the body.
type WebhookVerificationError struct {
	Code    string
	Message string
}

func (e *WebhookVerificationError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func whFail(code, message string) error {
	return &WebhookVerificationError{Code: code, Message: message}
}

// WebhookOptions configure VerifyWebhook. Scheme is "standard" (Standard Webhooks) when empty; Tolerance is
// DefaultWebhookTolerance (TenantWebhookTolerance for "tenant") when zero; Now is time.Now() when zero.
type WebhookOptions struct {
	Scheme    string
	Tolerance time.Duration
	Now       time.Time
}

// WebhookEvent is a verified webhook. Nullable fields are nil when the webhook does not carry them. Type is the
// Superroute event type without the CloudEvents "ca.superroute." prefix; Format is native, ce_binary or
// ce_structured; Data is the body, or a structured envelope's data; Payload is the whole parsed body.
type WebhookEvent struct {
	ID       *string
	Type     *string
	Time     *string
	Subject  *string
	Source   *string
	Business *int64
	Attempt  *int64
	Format   string
	Data     any
	Payload  any
}

// VerifyWebhook verifies a Superroute webhook and answers its event. Pass the raw body exactly as received, the
// request headers, and the endpoint's secret (a whsec_… secret or the plain secret; several during a rotation):
//
//	event, err := sapp.VerifyWebhook(body, r.Header, []string{secret}, sapp.WebhookOptions{})
//
// Errors are *WebhookVerificationError.
func VerifyWebhook(body []byte, headers http.Header, secrets []string, opts WebhookOptions) (*WebhookEvent, error) {
	read := func(name string) string { return headers.Get(name) }
	return verifyWebhook(body, read, secrets, opts)
}

// VerifyWebhookMap is VerifyWebhook for headers in a plain map; names are matched in any case.
func VerifyWebhookMap(body []byte, headers map[string]string, secrets []string, opts WebhookOptions) (*WebhookEvent, error) {
	lower := make(map[string]string, len(headers))
	for k, v := range headers {
		lower[strings.ToLower(k)] = v
	}
	return verifyWebhook(body, func(name string) string { return lower[strings.ToLower(name)] }, secrets, opts)
}

var (
	strictBase64 = regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`)
	digits       = regexp.MustCompile(`^\d+$`)
	integer      = regexp.MustCompile(`^-?\d+$`)
	splitter     = regexp.MustCompile(`[ ,]+`)
)

func keyOf(secret string) ([]byte, error) {
	if secret == "" {
		return nil, whFail("invalid_secret", "The webhook secret is empty.")
	}
	if strings.HasPrefix(secret, "whsec_") {
		raw := secret[len("whsec_"):]
		if !strictBase64.MatchString(raw) || len(raw)%4 != 0 {
			return nil, whFail("invalid_secret", "The whsec_ secret is not valid base64.")
		}
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil || len(key) == 0 {
			return nil, whFail("invalid_secret", "The whsec_ secret is not valid base64.")
		}
		return key, nil
	}
	return []byte(secret), nil
}

func mac(key []byte, parts ...[]byte) []byte {
	h := hmac.New(sha256.New, key)
	for _, p := range parts {
		h.Write(p)
	}
	return h.Sum(nil)
}

func need(read func(string) string, name string) (string, error) {
	v := read(name)
	if v == "" {
		return "", whFail("missing_header", fmt.Sprintf("The %s header is missing.", name))
	}
	return v, nil
}

func checkTime(raw string, now time.Time, tolerance time.Duration) (string, error) {
	raw = strings.TrimSpace(raw)
	if !digits.MatchString(raw) {
		return "", whFail("invalid_timestamp", "The webhook timestamp is not a number of seconds.")
	}
	ts, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return "", whFail("invalid_timestamp", "The webhook timestamp is not a number of seconds.")
	}
	diff := now.Unix() - ts
	if diff < 0 {
		diff = -diff
	}
	if diff > int64(tolerance/time.Second) {
		return "", whFail("timestamp_out_of_tolerance", "The webhook timestamp is outside the tolerance.")
	}
	return raw, nil
}

func entries(header, prefix string) []string {
	var out []string
	for _, s := range splitter.Split(header, -1) {
		if strings.HasPrefix(s, prefix) {
			out = append(out, s[len(prefix):])
		}
	}
	return out
}

func checkWebhookSignature(scheme string, read func(string) string, body []byte, keys [][]byte, now time.Time, tolerance time.Duration) error {
	var given []string
	var expected func(key []byte) string
	b64 := base64.StdEncoding.EncodeToString
	switch scheme {
	case "standard":
		id, err := need(read, "webhook-id")
		if err != nil {
			return err
		}
		ts, err := need(read, "webhook-timestamp")
		if err != nil {
			return err
		}
		header, err := need(read, "webhook-signature")
		if err != nil {
			return err
		}
		if ts, err = checkTime(ts, now, tolerance); err != nil {
			return err
		}
		for _, s := range strings.Split(header, " ") {
			if strings.HasPrefix(s, "v1,") {
				given = append(given, s[3:])
			}
		}
		expected = func(key []byte) string { return b64(mac(key, []byte(id+"."+ts+"."), body)) }
	case "tenant":
		ts, err := need(read, "X-Webhook-Timestamp")
		if err != nil {
			return err
		}
		header, err := need(read, "X-Webhook-Signature-V2")
		if err != nil {
			return err
		}
		if ts, err = checkTime(ts, now, tolerance); err != nil {
			return err
		}
		given = []string{strings.TrimSpace(header)}
		expected = func(key []byte) string { return hex.EncodeToString(mac(key, []byte(ts+"."), body)) }
	case "partner_locker", "tpd":
		ts, err := need(read, "X-Webhook-Timestamp")
		if err != nil {
			return err
		}
		delivery, err := need(read, "X-Webhook-Delivery-Id")
		if err != nil {
			return err
		}
		header, err := need(read, "X-Webhook-Signature")
		if err != nil {
			return err
		}
		if ts, err = checkTime(ts, now, tolerance); err != nil {
			return err
		}
		given = entries(header, "v1=")
		expected = func(key []byte) string { return b64(mac(key, []byte(ts+"\n"+delivery+"\n"), body)) }
	case "loading_plan", "open_platform":
		prefix := "X-Webhook"
		if scheme == "open_platform" {
			prefix = "X-Open"
		}
		ts, err := need(read, prefix+"-Timestamp")
		if err != nil {
			return err
		}
		header, err := need(read, prefix+"-Signature")
		if err != nil {
			return err
		}
		if ts, err = checkTime(ts, now, tolerance); err != nil {
			return err
		}
		given = entries(header, "v1=")
		expected = func(key []byte) string { return hex.EncodeToString(mac(key, []byte(ts+"."), body)) }
	case "dataset":
		header, err := need(read, "X-Webhook-Signature")
		if err != nil {
			return err
		}
		// Sent but not signed: it still turns away a plain replay of an old request.
		if ts := read("X-Webhook-Timestamp"); ts != "" {
			if _, err := checkTime(ts, now, tolerance); err != nil {
				return err
			}
		}
		given = []string{strings.TrimSpace(header)}
		expected = func(key []byte) string { return hex.EncodeToString(mac(key, body)) }
	default:
		return whFail("unknown_scheme", "Unknown webhook scheme "+scheme+".")
	}
	for _, key := range keys {
		sig := []byte(expected(key))
		for _, g := range given {
			if subtle.ConstantTimeCompare([]byte(g), sig) == 1 {
				return nil
			}
		}
	}
	return whFail("invalid_signature", "No signature matches the secret.")
}

func verifyWebhook(body []byte, read func(string) string, secrets []string, opts WebhookOptions) (*WebhookEvent, error) {
	scheme := opts.Scheme
	if scheme == "" {
		scheme = "standard"
	}
	known := false
	for _, s := range WebhookSchemes {
		known = known || s == scheme
	}
	if !known {
		return nil, whFail("unknown_scheme", "Unknown webhook scheme "+scheme+".")
	}
	if len(secrets) == 0 {
		return nil, whFail("invalid_secret", "No webhook secret was given.")
	}
	keys := make([][]byte, 0, len(secrets))
	for _, s := range secrets {
		k, err := keyOf(s)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	tolerance := opts.Tolerance
	if tolerance == 0 {
		tolerance = DefaultWebhookTolerance
		if scheme == "tenant" {
			tolerance = TenantWebhookTolerance
		}
	}
	if err := checkWebhookSignature(scheme, read, body, keys, now, tolerance); err != nil {
		return nil, err
	}
	var payload any
	dec := json.NewDecoder(strings.NewReader(string(body)))
	if err := dec.Decode(&payload); err != nil || dec.More() {
		return nil, whFail("invalid_payload", "The webhook body is not JSON.")
	}
	return eventOf(read, payload), nil
}

func strPtr(v any) *string {
	if s, ok := v.(string); ok && s != "" {
		return &s
	}
	return nil
}

func intPtr(v any) *int64 {
	switch x := v.(type) {
	case float64:
		if x == float64(int64(x)) {
			n := int64(x)
			return &n
		}
	case string:
		if integer.MatchString(x) {
			if n, err := strconv.ParseInt(x, 10, 64); err == nil {
				return &n
			}
		}
	}
	return nil
}

func stripType(t *string) *string {
	if t != nil && strings.HasPrefix(*t, typePrefix) {
		s := (*t)[len(typePrefix):]
		return &s
	}
	return t
}

func eventOf(read func(string) string, payload any) *WebhookEvent {
	object, _ := payload.(map[string]any)
	if object == nil {
		object = map[string]any{}
	}
	_, hasData := object["data"]
	contentType := strings.ToLower(read("Content-Type"))
	if strings.HasPrefix(contentType, "application/cloudevents+json") ||
		(object["specversion"] != nil && hasData && strPtr(object["type"]) != nil && strPtr(object["id"]) != nil) {
		return &WebhookEvent{
			ID: strPtr(object["id"]), Type: stripType(strPtr(object["type"])), Time: strPtr(object["time"]),
			Subject: strPtr(object["subject"]), Source: strPtr(object["source"]), Business: intPtr(object["srbusiness"]),
			Format: "ce_structured", Data: object["data"], Payload: payload,
		}
	}
	h := func(name string) any { return read(name) }
	if read("ce-specversion") != "" {
		return &WebhookEvent{
			ID: strPtr(h("ce-id")), Type: stripType(strPtr(h("ce-type"))), Time: strPtr(h("ce-time")),
			Subject: strPtr(h("ce-subject")), Source: strPtr(h("ce-source")), Business: intPtr(h("ce-srbusiness")),
			Attempt: intPtr(h("ce-srattempt")), Format: "ce_binary", Data: payload, Payload: payload,
		}
	}
	first := func(values ...any) *string {
		for _, v := range values {
			if s := strPtr(v); s != nil {
				return s
			}
		}
		return nil
	}
	return &WebhookEvent{
		ID:     first(h("webhook-id"), h("X-Webhook-Event-Id"), h("X-Webhook-Id"), h("X-Open-Delivery"), object["event_id"]),
		Type:   first(h("X-Webhook-Event-Type"), h("X-Open-Event"), h("X-Webhook-Event"), object["event_type"], object["event"]),
		Time:   first(object["occurred_at"], object["event_time"], object["timestamp"]),
		Format: "native", Data: payload, Payload: payload,
	}
}
