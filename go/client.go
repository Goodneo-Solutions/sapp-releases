// Package sapp is sApp for Go: the partner app API with sign-in, Open Platform capabilities, GraphQL and webhook
// verification for Superroute (https://open.superroute.ca/developer/sapp).
//
// Answers are JSON decoded into any: map[string]any, []any, string, float64, bool or nil.
package sapp

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Version is the sApp version.
const Version = "0.2.5"

// DefaultBaseURL is where the partner app API and the Open Platform answer.
const DefaultBaseURL = "https://open.superroute.ca"

const (
	tokenExchange = "urn:ietf:params:oauth:grant-type:token-exchange"
	idTokenType   = "urn:ietf:params:oauth:token-type:id_token"
	// refreshMargin: refresh this long before the access token expires, so a call never starts with a token about to lapse.
	refreshMargin = 60 * time.Second
)

// ------------------------------------------------------------------------------------------------ errors

// Error is every failed call. Status is the HTTP status (0 when the request never got an answer), Code the server's
// error code (invalid_token, insufficient_scope, access_denied, a platform app code such as APP_ACCESS_DENIED, …)
// when it sent one, Body the parsed answer.
//
// An answer {"result": false, …} is a failed call too, whatever its status (Superroute sometimes answers a refusal
// with 200): Errors holds its field messages, Reasons and Data what it sent beside them.
//
// Headers are the refused answer's headers (names in lower case, several values of one name joined with ", "; empty
// when there was no answer), RetryAfter its Retry-After in whole seconds (nil when absent or not understood) and
// RequestID the answer's request_id, else its X-Request-Id header.
type Error struct {
	Status    int
	Code      string
	Message   string
	Body      any
	RequestID string
	// Errors is field -> messages ({"email": ["The email is taken."]}); nil when the answer named none.
	Errors map[string][]string
	// Reasons is the answer's "reasons", as sent (a list or a map), or nil.
	Reasons any
	// Data is the answer's "data", as sent, or nil.
	Data any
	// Headers are the refused answer's headers: names in lower case, several values of one name joined with ", ".
	// Empty when the request never got an answer.
	Headers map[string]string
	// RetryAfter is the answer's Retry-After in whole seconds (an HTTP-date as the seconds until it, at least 0), or nil.
	RetryAfter *int
	cause      error
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Code != "" {
		return e.Code
	}
	return fmt.Sprintf("Request failed with status %d", e.Status)
}

// Unwrap answers the transport error of a network failure.
func (e *Error) Unwrap() error { return e.cause }

// SignInRequired is true when the person must sign in again: the session ended or was revoked.
func (e *Error) SignInRequired() bool { return e.Status == 401 || e.Code == "invalid_grant" }

var upperCode = regexp.MustCompile(`^[A-Z_]+$`)

// lowerHeaders answers the headers with names in lower case and several values of one name joined with ", ".
func lowerHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for name, values := range h {
		key := strings.ToLower(name)
		joined := strings.Join(values, ", ")
		if prev, ok := out[key]; ok {
			joined = prev + ", " + joined
		}
		out[key] = joined
	}
	return out
}

// retryAfterSeconds reads Retry-After in whole seconds: a non-negative integer as is, an HTTP-date as the seconds
// until it (rounded up, at least 0), anything else nil.
func retryAfterSeconds(value string, now time.Time) *int {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.Trim(value, "0123456789") == "" {
		n, err := strconv.Atoi(value)
		if err != nil {
			return nil
		}
		return &n
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return nil
	}
	secs := int(math.Ceil(when.Sub(now).Seconds()))
	if secs < 0 {
		secs = 0
	}
	return &secs
}

// withAnswer fills the answer's headers, Retry-After and (when the error has none) X-Request-Id into e.
func (e *Error) withAnswer(headers map[string]string) *Error {
	if headers == nil {
		headers = map[string]string{}
	}
	e.Headers = headers
	e.RetryAfter = retryAfterSeconds(headers["retry-after"], time.Now())
	if e.RequestID == "" {
		e.RequestID = headers["x-request-id"]
	}
	return e
}

func errorFrom(status int, body any, requestID string) *Error {
	b, _ := body.(map[string]any)
	str := func(k string) string { s, _ := b[k].(string); return s }
	scalar := func(k string) string {
		switch v := b[k].(type) {
		case string:
			return v
		case nil, map[string]any, []any:
			return ""
		default:
			return text(v)
		}
	}
	code := scalar("error")
	if code == "" {
		code = scalar("code")
	}
	if msg := str("message"); code == "" && upperCode.MatchString(msg) {
		code = msg
	}
	text := str("error_description")
	if text == "" {
		if list, ok := b["message"].([]any); ok {
			parts := make([]string, 0, len(list))
			for _, p := range list {
				parts = append(parts, fmt.Sprint(p))
			}
			text = strings.Join(parts, " ")
		} else if s := str("message"); s != "" {
			text = s
		} else {
			text = str("title")
		}
	}
	if id := str("request_id"); id != "" {
		requestID = id
	}
	return &Error{Status: status, Code: code, Message: text, Body: body, RequestID: requestID,
		Errors: fieldErrors(b["errors"]), Reasons: b["reasons"], Data: b["data"]}
}

func fieldErrors(raw any) map[string][]string {
	m, ok := raw.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}
	out := make(map[string][]string, len(m))
	for field, v := range m {
		if v == nil {
			continue
		}
		if items, ok := v.([]any); ok {
			msgs := make([]string, 0, len(items))
			for _, item := range items {
				msgs = append(msgs, text(item))
			}
			out[field] = msgs
		} else {
			out[field] = []string{text(v)}
		}
	}
	return out
}

// refused is true for status 400 or more, or a JSON object answering result false (even with a 2xx status).
func refused(status int, body any) bool {
	if status >= 400 {
		return true
	}
	b, ok := body.(map[string]any)
	return ok && b["result"] == false
}

// protectedHeaders are set by the SDK itself; a header argument never replaces them.
var protectedHeaders = map[string]bool{
	"authorization": true, "x-app-key": true, "x-sapp": true, "accept": true, "accept-language": true, "content-type": true,
}

// uuid4 answers a random (version 4) UUID, for an idempotency key the caller did not give.
func uuid4() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// callHeaders are the header parameters of one call, worked out once so every attempt of the call sends the same
// values: an operation that declares Idempotency-Key gets a fresh key when the caller gave none.
func callHeaders(op Operation, params map[string]any) map[string]string {
	out := map[string]string{}
	for _, name := range op.Headers {
		if protectedHeaders[strings.ToLower(name)] {
			continue
		}
		if v := text(params[name]); v != "" {
			out[name] = v
		} else if strings.EqualFold(name, "Idempotency-Key") {
			out[name] = uuid4()
		}
	}
	return out
}

// withCallHeaders adds extra to headers without replacing a header the SDK set (compared without regard to case).
func withCallHeaders(headers, extra map[string]string) map[string]string {
	taken := make(map[string]bool, len(headers))
	for k := range headers {
		taken[strings.ToLower(k)] = true
	}
	for k, v := range extra {
		if !taken[strings.ToLower(k)] {
			headers[k] = v
		}
	}
	return headers
}

// ------------------------------------------------------------------------------------------------ tokens

// Tokens are a signed-in person's tokens.
type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scope        []string  `json:"scope"`
}

// TokenStore is where the tokens are kept. Get answers nil when nobody is signed in.
type TokenStore interface {
	Get(ctx context.Context) (*Tokens, error)
	Set(ctx context.Context, tokens *Tokens) error
	Clear(ctx context.Context) error
}

// MemoryTokenStore keeps the tokens in memory only.
type MemoryTokenStore struct {
	mu     sync.Mutex
	tokens *Tokens
}

// NewMemoryTokenStore answers a store holding initial (nil for none).
func NewMemoryTokenStore(initial *Tokens) *MemoryTokenStore {
	return &MemoryTokenStore{tokens: initial}
}

// Get answers the tokens, or nil.
func (s *MemoryTokenStore) Get(context.Context) (*Tokens, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tokens == nil {
		return nil, nil
	}
	t := *s.tokens
	return &t, nil
}

// Set keeps the tokens.
func (s *MemoryTokenStore) Set(_ context.Context, tokens *Tokens) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := *tokens
	s.tokens = &t
	return nil
}

// Clear forgets the tokens.
func (s *MemoryTokenStore) Clear(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens = nil
	return nil
}

func tokensFrom(body any, now time.Time, headers ...map[string]string) (*Tokens, error) {
	b, _ := body.(map[string]any)
	access, _ := b["access_token"].(string)
	if access == "" {
		e := &Error{Status: 200, Code: "invalid_response", Message: "The token answer had no access token.", Body: body}
		if id, _ := b["request_id"].(string); id != "" {
			e.RequestID = id
		}
		if len(headers) > 0 {
			e.withAnswer(headers[0])
		}
		return nil, e
	}
	refresh, _ := b["refresh_token"].(string)
	expires := 3600.0
	if n, ok := b["expires_in"].(float64); ok && n > 0 {
		expires = n
	}
	scope := []string{}
	if s, ok := b["scope"].(string); ok {
		scope = strings.Fields(s)
	}
	return &Tokens{AccessToken: access, RefreshToken: refresh, ExpiresAt: now.Add(time.Duration(expires * float64(time.Second))), Scope: scope}, nil
}

// ------------------------------------------------------------------------------------------------ PKCE

func base64URL(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func randomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64URL(b)
}

// CodeChallenge is the S256 code challenge of a verifier (RFC 7636 §4.2).
func CodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64URL(sum[:])
}

// PKCE is a verifier and its challenge.
type PKCE struct {
	Verifier  string
	Challenge string
	Method    string
}

// NewPKCE answers a fresh PKCE pair.
func NewPKCE() PKCE {
	v := randomString(48)
	return PKCE{Verifier: v, Challenge: CodeChallenge(v), Method: "S256"}
}

// ------------------------------------------------------------------------------------------------ requests

var (
	pathParam   = regexp.MustCompile(`\{([^}]+)\}`)
	doubleSlash = regexp.MustCompile(`/{2,}`)
)

// deref turns typed nil pointers into nil and pointers into their values.
func deref(v any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		return rv.Elem().Interface()
	}
	return v
}

func text(v any) string {
	switch x := deref(v).(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "1"
		}
		return "0"
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1e15 {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case float32:
		return text(float64(x))
	case json.Number:
		return x.String()
	case map[string]any, []any:
		b, _ := json.Marshal(x)
		return string(b)
	default:
		return fmt.Sprint(x)
	}
}

// list answers the items of a slice value, or nil when v is not a slice.
func list(v any) ([]any, bool) {
	v = deref(v)
	if v == nil {
		return nil, false
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice || rv.Type().Elem().Kind() == reflect.Uint8 {
		return nil, false
	}
	out := make([]any, rv.Len())
	for i := range out {
		out[i] = rv.Index(i).Interface()
	}
	return out, true
}

func buildURL(base string, op Operation, params map[string]any, extra ...map[string]any) (string, error) {
	path := pathParam.ReplaceAllStringFunc(op.Path, func(m string) string {
		value := text(params[m[1:len(m)-1]])
		if value == "" {
			return ""
		}
		return url.PathEscape(value)
	})
	path = doubleSlash.ReplaceAllString(path, "/")
	path = strings.TrimRight(path, "/")
	if path == "" {
		path = "/"
	}
	u, err := url.Parse(base + path)
	if err != nil {
		return "", err
	}
	q := url.Values{}
	for _, name := range op.Query {
		v := deref(params[name])
		if v == nil {
			continue
		}
		if items, ok := list(v); ok {
			for _, item := range items {
				q.Add(name, text(item))
			}
			continue
		}
		q.Set(name, text(v))
	}
	u.RawQuery = q.Encode()
	// The extra query comes after the declared names and never replaces one of them.
	declared := make(map[string]bool, len(op.Query))
	for _, name := range op.Query {
		declared[name] = true
	}
	more := url.Values{}
	for _, m := range extra {
		for name, v := range m {
			v = deref(v)
			if name == "" || declared[name] || v == nil {
				continue
			}
			if items, ok := list(v); ok {
				for _, item := range items {
					if deref(item) != nil {
						more.Add(name, text(item))
					}
				}
				continue
			}
			more.Set(name, text(v))
		}
	}
	if len(more) > 0 {
		if u.RawQuery != "" {
			u.RawQuery += "&"
		}
		u.RawQuery += more.Encode()
	}
	return u.String(), nil
}

func encodeBody(kind BodyKind, body map[string]any, files []File) ([]byte, string, error) {
	switch kind {
	case BodyJSON:
		if body == nil {
			return nil, "", nil
		}
		b, err := json.Marshal(body)
		return b, "application/json", err
	case BodyForm:
		if body == nil {
			return nil, "", nil
		}
		form := url.Values{}
		for k, v := range body {
			if deref(v) != nil {
				form.Set(k, text(v))
			}
		}
		return []byte(form.Encode()), "application/x-www-form-urlencoded", nil
	case BodyMultipart:
		if body == nil && len(files) == 0 {
			return nil, "", nil
		}
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		for k, v := range body {
			if deref(v) == nil {
				continue
			}
			if items, ok := list(v); ok {
				for _, item := range items {
					_ = w.WriteField(k+"[]", text(item))
				}
				continue
			}
			_ = w.WriteField(k, text(v))
		}
		for _, f := range files {
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, f.Field, f.Filename))
			ct := f.ContentType
			if ct == "" {
				ct = "application/octet-stream"
			}
			h.Set("Content-Type", ct)
			part, err := w.CreatePart(h)
			if err != nil {
				return nil, "", err
			}
			if _, err := part.Write(f.Content); err != nil {
				return nil, "", err
			}
		}
		if err := w.Close(); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), w.FormDataContentType(), nil
	}
	return nil, "", nil
}

type request struct {
	method      string
	url         string
	headers     map[string]string
	body        []byte
	contentType string
}

type answer struct {
	status    int
	body      any
	requestID string
	retry     string
	headers   map[string]string
}

type transport struct {
	http    *http.Client
	retries int
	sleep   func(context.Context, time.Duration) error
}

func defaultSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// send makes the request. Repeatable calls are retried on a lost connection, 502–504, and 429 with Retry-After.
// Writes are never retried.
func (t *transport) send(ctx context.Context, r request, retryable bool) (*answer, error) {
	for attempt := 0; ; attempt++ {
		var body io.Reader
		if r.body != nil {
			body = bytes.NewReader(r.body)
		} else if r.method == "POST" || r.method == "PUT" || r.method == "PATCH" {
			body = bytes.NewReader(nil)
		}
		req, err := http.NewRequestWithContext(ctx, r.method, r.url, body)
		if err != nil {
			return nil, err
		}
		for k, v := range r.headers {
			req.Header.Set(k, v)
		}
		if r.contentType != "" {
			req.Header.Set("Content-Type", r.contentType)
		}
		res, err := t.http.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if retryable && attempt < t.retries {
				if err := t.sleep(ctx, (300*time.Millisecond)<<attempt); err != nil {
					return nil, err
				}
				continue
			}
			return nil, &Error{Status: 0, Code: "network_error", Message: "The request could not reach Superroute.",
				Headers: map[string]string{}, cause: err}
		}
		raw, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil && retryable && attempt < t.retries {
			continue
		}
		retryAfter := res.Header.Get("Retry-After")
		transient := res.StatusCode >= 502 && res.StatusCode <= 504
		throttled := res.StatusCode == 429 && retryAfter != ""
		if retryable && attempt < t.retries && (transient || throttled) {
			wait := (300 * time.Millisecond) << attempt
			if throttled {
				secs, err := strconv.Atoi(strings.TrimSpace(retryAfter))
				if err != nil || secs < 1 {
					secs = 1
				}
				if secs > 30 {
					secs = 30
				}
				wait = time.Duration(secs) * time.Second
			}
			if err := t.sleep(ctx, wait); err != nil {
				return nil, err
			}
			continue
		}
		var parsed any
		if len(raw) > 0 {
			if json.Unmarshal(raw, &parsed) != nil {
				parsed = string(raw)
			}
		}
		return &answer{status: res.StatusCode, body: parsed, requestID: res.Header.Get("X-Request-Id"),
			headers: lowerHeaders(res.Header)}, nil
	}
}

func (a *answer) orError() (any, error) {
	if refused(a.status, a.body) {
		return nil, errorFrom(a.status, a.body, a.requestID).withAnswer(a.headers)
	}
	return a.body, nil
}

// ------------------------------------------------------------------------------------------------ partner client

// PartnerOptions configure a PartnerClient. AppCode is required; ClientCode (the service provider's public client
// code) is required for sign-in and public calls; AppKey is the restricted key for calls before sign-in.
type PartnerOptions struct {
	AppCode     string
	ClientCode  string
	AppKey      string
	TokenStore  TokenStore
	BaseURL     string
	Locale      string
	DeviceID    string
	Retries     *int
	HTTPClient  *http.Client
	Now         func() time.Time
	Sleep       func(ctx context.Context, d time.Duration) error
	OnSignedOut func()
}

// PartnerClient is the partner profile: an app registered on the Open Platform console, acting for one person at one
// service provider.
//
//	sr, _ := sapp.NewPartnerClient(sapp.PartnerOptions{AppCode: "p_acme_parcels", ClientCode: code, AppKey: key})
//	_, _ = sr.SignInWithIDToken(ctx, googleIDToken, nil)
//	wallet, err := sr.API.PlatformAppWallet(ctx)
type PartnerClient struct {
	// API has one method per operation of the partner surface.
	API *PartnerAPI

	appCode     string
	appKey      string
	baseURL     string
	deviceID    string
	store       TokenStore
	now         func() time.Time
	onSignedOut func()
	t           *transport

	mu         sync.Mutex
	clientCode string
	locale     string
	refreshing *refreshCall
	listeners  map[int]func(bool)
	nextID     int
}

type refreshCall struct {
	done   chan struct{}
	tokens *Tokens
	err    error
}

// AuthorizationRequest is a started sign-in on the Superroute sign-in page.
type AuthorizationRequest struct {
	URL         string
	Verifier    string
	State       string
	RedirectURI string
}

// NewPartnerClient makes a partner client.
func NewPartnerClient(o PartnerOptions) (*PartnerClient, error) {
	if o.AppCode == "" {
		return nil, errors.New("sapp: AppCode is required")
	}
	c := &PartnerClient{
		appCode: o.AppCode, appKey: o.AppKey, deviceID: o.DeviceID, clientCode: o.ClientCode, locale: o.Locale,
		baseURL: strings.TrimRight(o.BaseURL, "/"), store: o.TokenStore, now: o.Now, onSignedOut: o.OnSignedOut,
		listeners: map[int]func(bool){},
	}
	if c.baseURL == "" {
		c.baseURL = DefaultBaseURL
	}
	if c.store == nil {
		c.store = NewMemoryTokenStore(nil)
	}
	if c.now == nil {
		c.now = time.Now
	}
	c.t = newTransport(o.HTTPClient, o.Retries, o.Sleep)
	c.API = &PartnerAPI{call: c.apiCall}
	return c, nil
}

func newTransport(h *http.Client, retries *int, sleep func(context.Context, time.Duration) error) *transport {
	t := &transport{http: h, retries: 2, sleep: sleep}
	if t.http == nil {
		t.http = http.DefaultClient
	}
	if retries != nil {
		t.retries = *retries
	}
	if t.sleep == nil {
		t.sleep = defaultSleep
	}
	return t
}

// SetClientCode changes the service provider, e.g. after the person picked one at sign-up. Sign in again afterwards.
func (c *PartnerClient) SetClientCode(code string) {
	c.mu.Lock()
	c.clientCode = code
	c.mu.Unlock()
}

// SetLocale changes the language sent as Accept-Language.
func (c *PartnerClient) SetLocale(locale string) {
	c.mu.Lock()
	c.locale = locale
	c.mu.Unlock()
}

func (c *PartnerClient) client() (code, locale string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.clientCode, c.locale
}

// OnSessionChange calls listener with true when a sign-in starts a session and false when it ends. It answers the
// function that stops it.
func (c *PartnerClient) OnSessionChange(listener func(bool)) func() {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.listeners[id] = listener
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		delete(c.listeners, id)
		c.mu.Unlock()
	}
}

func (c *PartnerClient) emit(signedIn bool) {
	c.mu.Lock()
	ls := make([]func(bool), 0, len(c.listeners))
	for _, l := range c.listeners {
		ls = append(ls, l)
	}
	c.mu.Unlock()
	for _, l := range ls {
		func() {
			// A listener's panic never breaks the session.
			defer func() { _ = recover() }()
			l(signedIn)
		}()
	}
}

func (c *PartnerClient) baseHeaders() map[string]string {
	h := map[string]string{"Accept": "application/json", "X-SApp": "go/" + Version}
	if _, locale := c.client(); locale != "" {
		h["Accept-Language"] = locale
	}
	return h
}

func (c *PartnerClient) requireClientCode() (string, error) {
	code, _ := c.client()
	if code == "" {
		return "", errors.New("sapp: ClientCode is required to sign in")
	}
	return code, nil
}

func formOf(fields [][2]string) []byte {
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		if f[1] != "" {
			parts = append(parts, url.QueryEscape(f[0])+"="+url.QueryEscape(f[1]))
		}
	}
	return []byte(strings.Join(parts, "&"))
}

func (c *PartnerClient) tokenRequest(ctx context.Context, fields ...[2]string) (*Tokens, error) {
	all := append([][2]string{{"client_id", c.appCode}}, fields...)
	a, err := c.t.send(ctx, request{method: "POST", url: c.baseURL + "/app/oauth/token", headers: c.baseHeaders(),
		body: formOf(all), contentType: "application/x-www-form-urlencoded"}, false)
	if err != nil {
		return nil, err
	}
	body, err := a.orError()
	if err != nil {
		return nil, err
	}
	tokens, err := tokensFrom(body, c.now(), a.headers)
	if err != nil {
		return nil, err
	}
	was, err := c.store.Get(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.store.Set(ctx, tokens); err != nil {
		return nil, err
	}
	if was == nil {
		c.emit(true)
	}
	return tokens, nil
}

func (c *PartnerClient) endSession(ctx context.Context) {
	_ = c.store.Clear(ctx)
	if c.onSignedOut != nil {
		c.onSignedOut()
	}
	c.emit(false)
}

func scopeText(scope []string) string { return strings.Join(scope, " ") }

// AuthorizationRequest starts sign-in on the Superroute sign-in page (method 1): open URL in the system browser and
// pass the URL it returns to CompleteSignIn.
func (c *PartnerClient) AuthorizationRequest(redirectURI string, scope []string, language string) (AuthorizationRequest, error) {
	code, err := c.requireClientCode()
	if err != nil {
		return AuthorizationRequest{}, err
	}
	pkce := NewPKCE()
	state := randomString(16)
	if language == "" {
		_, language = c.client()
	}
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", c.appCode)
	q.Set("client_code", code)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("code_challenge", pkce.Challenge)
	q.Set("code_challenge_method", "S256")
	if len(scope) > 0 {
		q.Set("scope", scopeText(scope))
	}
	if language != "" {
		q.Set("language", language)
	}
	return AuthorizationRequest{URL: c.baseURL + "/app/oauth/authorize?" + q.Encode(), Verifier: pkce.Verifier, State: state, RedirectURI: redirectURI}, nil
}

// CompleteSignIn finishes method 1 with the URL the sign-in page returned to. It fails with invalid_state when the
// answer does not belong to the request.
func (c *PartnerClient) CompleteSignIn(ctx context.Context, req AuthorizationRequest, returnedURL string) (*Tokens, error) {
	u, err := url.Parse(returnedURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	if q.Get("state") != req.State {
		return nil, &Error{Status: 400, Code: "invalid_state", Message: "The sign-in answer does not belong to this request."}
	}
	if e := q.Get("error"); e != "" {
		return nil, &Error{Status: 400, Code: e, Message: q.Get("error_description")}
	}
	return c.ExchangeCode(ctx, q.Get("code"), req.RedirectURI, req.Verifier)
}

// ExchangeCode finishes method 1 with the redirect's code and the verifier of the request.
func (c *PartnerClient) ExchangeCode(ctx context.Context, code, redirectURI, verifier string) (*Tokens, error) {
	return c.tokenRequest(ctx, [2]string{"grant_type", "authorization_code"}, [2]string{"code", code},
		[2]string{"redirect_uri", redirectURI}, [2]string{"code_verifier", verifier}, [2]string{"device_id", c.deviceID})
}

// SignInWithIDToken is method 2: a Google or Apple ID token issued for one of the app's registered audiences.
func (c *PartnerClient) SignInWithIDToken(ctx context.Context, idToken string, scope []string) (*Tokens, error) {
	code, err := c.requireClientCode()
	if err != nil {
		return nil, err
	}
	return c.tokenRequest(ctx, [2]string{"grant_type", tokenExchange}, [2]string{"client_code", code},
		[2]string{"subject_token", idToken}, [2]string{"subject_token_type", idTokenType},
		[2]string{"scope", scopeText(scope)}, [2]string{"device_id", c.deviceID})
}

// SignInWithPassword is method 3: email and password in the app's own screens, for apps approved for it.
func (c *PartnerClient) SignInWithPassword(ctx context.Context, username, password string, scope []string) (*Tokens, error) {
	code, err := c.requireClientCode()
	if err != nil {
		return nil, err
	}
	return c.tokenRequest(ctx, [2]string{"grant_type", "password"}, [2]string{"client_code", code},
		[2]string{"username", username}, [2]string{"password", password},
		[2]string{"scope", scopeText(scope)}, [2]string{"device_id", c.deviceID})
}

// Refresh gets new tokens. One refresh at a time: concurrent callers wait for the same one (a refresh token works
// once). A refused refresh ends the session.
func (c *PartnerClient) Refresh(ctx context.Context) (*Tokens, error) {
	c.mu.Lock()
	if r := c.refreshing; r != nil {
		c.mu.Unlock()
		select {
		case <-r.done:
			return r.tokens, r.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	r := &refreshCall{done: make(chan struct{})}
	c.refreshing = r
	c.mu.Unlock()

	r.tokens, r.err = c.doRefresh(ctx)
	c.mu.Lock()
	c.refreshing = nil
	c.mu.Unlock()
	close(r.done)
	return r.tokens, r.err
}

func (c *PartnerClient) doRefresh(ctx context.Context) (*Tokens, error) {
	tokens, err := c.store.Get(ctx)
	if err != nil {
		return nil, err
	}
	if tokens == nil || tokens.RefreshToken == "" {
		c.endSession(ctx)
		return nil, &Error{Status: 401, Code: "invalid_grant", Message: "Not signed in."}
	}
	fresh, err := c.tokenRequest(ctx, [2]string{"grant_type", "refresh_token"}, [2]string{"refresh_token", tokens.RefreshToken},
		[2]string{"device_id", c.deviceID})
	var e *Error
	if errors.As(err, &e) && (e.Code == "invalid_grant" || e.Status == 401) {
		c.endSession(ctx)
	}
	return fresh, err
}

func (c *PartnerClient) accessToken(ctx context.Context) (string, error) {
	tokens, err := c.store.Get(ctx)
	if err != nil || tokens == nil {
		return "", err
	}
	if tokens.RefreshToken != "" && !tokens.ExpiresAt.Add(-refreshMargin).After(c.now()) {
		fresh, err := c.Refresh(ctx)
		if err != nil {
			return "", err
		}
		return fresh.AccessToken, nil
	}
	return tokens.AccessToken, nil
}

// SignOut ends this sign-in on the server and forgets the tokens.
func (c *PartnerClient) SignOut(ctx context.Context) error {
	tokens, err := c.store.Get(ctx)
	if err != nil {
		return err
	}
	if tokens != nil && tokens.RefreshToken != "" {
		// Signing out locally never waits on a failed network.
		_, _ = c.t.send(ctx, request{method: "POST", url: c.baseURL + "/app/oauth/revoke", headers: c.baseHeaders(),
			body: formOf([][2]string{{"token", tokens.RefreshToken}, {"client_id", c.appCode}}), contentType: "application/x-www-form-urlencoded"}, false)
	}
	c.endSession(ctx)
	return nil
}

// IsSignedIn is true while the store holds tokens.
func (c *PartnerClient) IsSignedIn(ctx context.Context) (bool, error) {
	t, err := c.store.Get(ctx)
	return t != nil, err
}

func (c *PartnerClient) authorise(ctx context.Context, op Operation, headers map[string]string) error {
	if op.Auth == AuthToken || op.Auth == AuthTokenOrKey {
		token, err := c.accessToken(ctx)
		if err != nil {
			return err
		}
		if token != "" {
			headers["Authorization"] = "Bearer " + token
			return nil
		}
		if op.Auth == AuthToken {
			return &Error{Status: 401, Code: "invalid_token", Message: "Not signed in."}
		}
	}
	if (op.Auth == AuthKey || op.Auth == AuthTokenOrKey) && c.appKey != "" {
		headers["X-App-Key"] = c.appKey
	}
	return nil
}

func (c *PartnerClient) call(ctx context.Context, op Operation, params map[string]any, body map[string]any, files []File, opts ...CallOption) (any, error) {
	a, err := c.exchange(ctx, op, params, body, files, opts...)
	if err != nil {
		return nil, err
	}
	return a.orError()
}

func (c *PartnerClient) exchange(ctx context.Context, op Operation, params map[string]any, body map[string]any, files []File, opts ...CallOption) (*answer, error) {
	u, err := buildURL(c.baseURL, op, params, optionsOf(opts).extraQuery)
	if err != nil {
		return nil, err
	}
	payload, contentType, err := encodeBody(op.Body, body, files)
	if err != nil {
		return nil, err
	}
	retryable := op.Method == "GET"
	perCall := callHeaders(op, params)
	headers := c.baseHeaders()
	if err := c.authorise(ctx, op, headers); err != nil {
		return nil, err
	}
	withCallHeaders(headers, perCall)
	r := request{method: op.Method, url: u, headers: headers, body: payload, contentType: contentType}
	a, err := c.t.send(ctx, r, retryable)
	if err != nil {
		return nil, err
	}
	// An access token can end early (revoked, the person's access changed): refresh once and try again.
	if a.status == 401 && headers["Authorization"] != "" {
		if _, err := c.Refresh(ctx); err != nil {
			return nil, err
		}
		headers = c.baseHeaders()
		if err := c.authorise(ctx, op, headers); err != nil {
			return nil, err
		}
		r.headers = withCallHeaders(headers, perCall)
		if a, err = c.t.send(ctx, r, retryable); err != nil {
			return nil, err
		}
	}
	return a, nil
}

func (c *PartnerClient) apiCall(ctx context.Context, op Operation, params map[string]any, body map[string]any, files []File, opts ...CallOption) (any, error) {
	// Public calls name the service provider in the path; take it from the client unless the caller passes one.
	if strings.Contains(op.Path, "{clientCode}") && deref(params["clientCode"]) == nil {
		if code, _ := c.client(); code != "" {
			copied := make(map[string]any, len(params)+1)
			for k, v := range params {
				copied[k] = v
			}
			copied["clientCode"] = code
			params = copied
		}
	}
	return c.call(ctx, op, params, body, files, opts...)
}

// CallOperation is for building on the sApp core: it calls an operation described by hand exactly as the generated
// methods of API do — the same sign-in headers (the token, or the app key when the operation allows it), one refresh
// and retry after a 401, an Idempotency-Key made once per call (and kept across that retry) when the operation
// declares one and none is given, retries of GET calls, and *Error for a refusal (status 400 or more, or
// "result": false). Path and query values and header parameters (under the header's own name) travel in params, the
// extra query in WithExtraQuery; {clientCode} is filled from the client when not given. For packages such as sLocker
// that add their own operations on top of this client.
func (c *PartnerClient) CallOperation(ctx context.Context, op Operation, params map[string]any, body map[string]any, files []File, opts ...CallOption) (any, error) {
	return c.apiCall(ctx, op, params, body, files, opts...)
}

// GraphQL runs a partner GraphQL operation (open.superroute.ca/app/graphql), with the token when signed in, else the
// app key, and answers its data. GraphQL errors fail with code graphql_error.
func (c *PartnerClient) GraphQL(ctx context.Context, query string, variables map[string]any) (any, error) {
	if variables == nil {
		variables = map[string]any{}
	}
	a, err := c.exchange(ctx, Operation{Method: "POST", Path: "/app/graphql", Auth: AuthTokenOrKey, Body: BodyJSON}, nil,
		map[string]any{"query": query, "variables": variables}, nil)
	if err != nil {
		return nil, err
	}
	answer, err := a.orError()
	if err != nil {
		return nil, err
	}
	b, _ := answer.(map[string]any)
	if errs, ok := b["errors"].([]any); ok && len(errs) > 0 {
		parts := make([]string, 0, len(errs))
		for _, e := range errs {
			if m, ok := e.(map[string]any); ok {
				s, _ := m["message"].(string)
				parts = append(parts, s)
			}
		}
		e := &Error{Status: 200, Code: "graphql_error", Message: strings.Join(parts, " "), Body: answer}
		if id, _ := b["request_id"].(string); id != "" {
			e.RequestID = id
		}
		return nil, e.withAnswer(a.headers)
	}
	if b == nil {
		return nil, nil
	}
	return b["data"], nil
}

// ------------------------------------------------------------------------------------------------ open client

// OpenOptions configure an OpenClient. Give GetToken, answered by your backend with a short-lived token, inside an
// app; give APIKey only on a server.
type OpenOptions struct {
	GetToken   func(ctx context.Context) (string, error)
	APIKey     string
	BaseURL    string
	Locale     string
	Retries    *int
	HTTPClient *http.Client
	Sleep      func(ctx context.Context, d time.Duration) error
}

// OpenClient is the open profile: Open Platform capabilities (routing, geocoding, ETA, packing, …) at
// open.superroute.ca/v1.
type OpenClient struct {
	// API has one method per Open Platform operation.
	API  *OpenAPI
	call operationCall
}

// CallOperation is for building on the sApp core: it calls an operation described by hand exactly as the generated
// methods of API do (the token, header parameters and idempotency keys, extra query, retries of GET calls and *Error
// for a refusal).
func (c *OpenClient) CallOperation(ctx context.Context, op Operation, params map[string]any, body map[string]any, files []File, opts ...CallOption) (any, error) {
	return c.call(ctx, op, params, body, files, opts...)
}

// NewOpenClient makes an Open Platform client.
func NewOpenClient(o OpenOptions) (*OpenClient, error) {
	if o.GetToken == nil && o.APIKey == "" {
		return nil, errors.New("sapp: GetToken or APIKey is required")
	}
	base := strings.TrimRight(o.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	t := newTransport(o.HTTPClient, o.Retries, o.Sleep)
	call := func(ctx context.Context, op Operation, params map[string]any, body map[string]any, files []File, opts ...CallOption) (any, error) {
		token := o.APIKey
		if o.GetToken != nil {
			var err error
			if token, err = o.GetToken(ctx); err != nil {
				return nil, err
			}
		}
		u, err := buildURL(base, op, params, optionsOf(opts).extraQuery)
		if err != nil {
			return nil, err
		}
		payload, contentType, err := encodeBody(op.Body, body, files)
		if err != nil {
			return nil, err
		}
		headers := map[string]string{"Accept": "application/json", "Authorization": "Bearer " + token, "X-SApp": "go/" + Version}
		if o.Locale != "" {
			headers["Accept-Language"] = o.Locale
		}
		withCallHeaders(headers, callHeaders(op, params))
		a, err := t.send(ctx, request{method: op.Method, url: u, headers: headers, body: payload, contentType: contentType}, op.Method == "GET")
		if err != nil {
			return nil, err
		}
		return a.orError()
	}
	return &OpenClient{API: &OpenAPI{call: call}, call: call}, nil
}
