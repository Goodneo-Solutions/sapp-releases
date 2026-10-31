package sapp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

type recorded struct {
	Method  string
	Path    string
	Query   url.Values
	Headers http.Header
	Form    url.Values
	JSON    map[string]any
	URI     string // escaped path and query as sent
}

type reply struct {
	status  int
	body    any // nil: empty body
	headers map[string]string
	network bool
}

// fake answers from a list of handlers in order and records every request.
type fake struct {
	mu       sync.Mutex
	handlers []func(recorded) reply
	calls    []recorded
}

func (f *fake) RoundTrip(r *http.Request) (*http.Response, error) {
	raw := []byte{}
	if r.Body != nil {
		raw, _ = io.ReadAll(r.Body)
	}
	c := recorded{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Headers: r.Header.Clone(), URI: r.URL.RequestURI()}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		c.Form, _ = url.ParseQuery(string(raw))
	} else if len(raw) > 0 {
		_ = json.Unmarshal(raw, &c.JSON)
	}
	f.mu.Lock()
	f.calls = append(f.calls, c)
	if len(f.handlers) == 0 {
		f.mu.Unlock()
		return nil, errors.New("unexpected request " + c.Method + " " + c.Path)
	}
	h := f.handlers[0]
	f.handlers = f.handlers[1:]
	f.mu.Unlock()
	a := h(c)
	if a.network {
		return nil, errors.New("connection refused")
	}
	text := ""
	if a.body != nil {
		b, _ := json.Marshal(a.body)
		text = string(b)
	}
	res := &http.Response{StatusCode: a.status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(text)), Request: r}
	for k, v := range a.headers {
		res.Header.Set(k, v)
	}
	return res, nil
}

func (f *fake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func fixed(replies ...reply) *fake {
	f := &fake{}
	for _, r := range replies {
		r := r
		f.handlers = append(f.handlers, func(recorded) reply { return r })
	}
	return f
}

func tokenBody(n string) map[string]any {
	return map[string]any{"access_token": "sp_app_access" + n, "refresh_token": "sp_app_refresh" + n, "token_type": "Bearer", "expires_in": 3600, "scope": "tracking notifications.in_app"}
}

func noSleep(context.Context, time.Duration) error { return nil }

func partner(t *testing.T, f *fake, o PartnerOptions) *PartnerClient {
	t.Helper()
	if o.AppCode == "" {
		o.AppCode = "p_a"
	}
	if o.ClientCode == "" {
		o.ClientCode = "c"
	}
	o.HTTPClient = &http.Client{Transport: f}
	o.Sleep = noSleep
	c, err := NewPartnerClient(o)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func signedIn(expiresIn time.Duration) *MemoryTokenStore {
	return NewMemoryTokenStore(&Tokens{AccessToken: "old", RefreshToken: "r1", ExpiresAt: time.Now().Add(expiresIn)})
}

func formMap(v url.Values) map[string]string {
	out := map[string]string{}
	for k := range v {
		out[k] = v.Get(k)
	}
	return out
}

func equalMaps(t *testing.T, got, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func asError(t *testing.T, err error) *Error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("want *Error, got %v", err)
	}
	return e
}

var ctx = context.Background()

func TestPKCEChallengeIsS256OfTheVerifier(t *testing.T) {
	if got := CodeChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"); got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatal(got)
	}
	p := NewPKCE()
	if len(p.Verifier) < 43 || p.Challenge != CodeChallenge(p.Verifier) || p.Method != "S256" {
		t.Fatal(p)
	}
}

func TestSignInPageURLAndCodeExchange(t *testing.T) {
	f := fixed(reply{status: 200, body: tokenBody("1")})
	sr := partner(t, f, PartnerOptions{AppCode: "p_acme_parcels", ClientCode: "k3m9q2", Locale: "fr"})
	req, err := sr.AuthorizationRequest("com.acme.parcels:/oauth", []string{"wallet.top_up"}, "")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(req.URL)
	q := u.Query()
	if u.Scheme+"://"+u.Host+u.Path != "https://open.superroute.ca/app/oauth/authorize" || q.Get("client_id") != "p_acme_parcels" ||
		q.Get("client_code") != "k3m9q2" || q.Get("code_challenge") != CodeChallenge(req.Verifier) || q.Get("code_challenge_method") != "S256" ||
		q.Get("state") != req.State || q.Get("scope") != "wallet.top_up" || q.Get("language") != "fr" || q.Get("redirect_uri") != "com.acme.parcels:/oauth" {
		t.Fatal(req.URL)
	}
	tokens, err := sr.ExchangeCode(ctx, "abc", "com.acme.parcels:/oauth", req.Verifier)
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken != "sp_app_access1" || strings.Join(tokens.Scope, ",") != "tracking,notifications.in_app" {
		t.Fatal(tokens)
	}
	equalMaps(t, formMap(f.calls[0].Form), map[string]string{"client_id": "p_acme_parcels", "grant_type": "authorization_code", "code": "abc", "redirect_uri": "com.acme.parcels:/oauth", "code_verifier": req.Verifier})
}

func TestIDTokenAndPasswordGrants(t *testing.T) {
	f := fixed(reply{status: 200, body: tokenBody("1")}, reply{status: 200, body: tokenBody("2")})
	sr := partner(t, f, PartnerOptions{AppCode: "p_acme_parcels", ClientCode: "k3m9q2", DeviceID: "ios-1"})
	if _, err := sr.SignInWithIDToken(ctx, "eyJ.google.token", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := sr.SignInWithPassword(ctx, "a@example.com", "secret", []string{"wallet.top_up"}); err != nil {
		t.Fatal(err)
	}
	equalMaps(t, formMap(f.calls[0].Form), map[string]string{"client_id": "p_acme_parcels", "grant_type": "urn:ietf:params:oauth:grant-type:token-exchange", "client_code": "k3m9q2",
		"subject_token": "eyJ.google.token", "subject_token_type": "urn:ietf:params:oauth:token-type:id_token", "device_id": "ios-1"})
	equalMaps(t, formMap(f.calls[1].Form), map[string]string{"client_id": "p_acme_parcels", "grant_type": "password", "client_code": "k3m9q2", "username": "a@example.com", "password": "secret", "scope": "wallet.top_up", "device_id": "ios-1"})
}

func TestARefusedSignInCarriesTheServerCode(t *testing.T) {
	f := fixed(reply{status: 400, body: map[string]any{"error": "interaction_required", "error_description": "This account uses two-step verification."}})
	sr := partner(t, f, PartnerOptions{})
	_, err := sr.SignInWithPassword(ctx, "a", "b", nil)
	e := asError(t, err)
	if e.Code != "interaction_required" || e.Status != 400 || !strings.Contains(e.Error(), "two-step") {
		t.Fatal(e)
	}
}

func TestCallsCarryTokenPathAndQuery(t *testing.T) {
	f := fixed(reply{status: 200, body: map[string]any{"result": true}}, reply{status: 200, body: map[string]any{"result": true}})
	sr := partner(t, f, PartnerOptions{TokenStore: signedIn(time.Hour), Locale: "de"})
	if _, err := sr.API.PlatformAppMarkNotificationRead(ctx, PlatformAppMarkNotificationReadParams{NotificationId: "0f8fad5b-d9cb-469f-a165-70867728950e"}); err != nil {
		t.Fatal(err)
	}
	c := f.calls[0]
	if c.Method != "POST" || c.Path != "/app/v1/notifications/0f8fad5b-d9cb-469f-a165-70867728950e/read" || c.Headers.Get("Authorization") != "Bearer old" || c.Headers.Get("Accept-Language") != "de" || c.Headers.Get("X-SApp") != "go/"+Version {
		t.Fatal(c)
	}
	// A GET with a query and no path parameters, called through the operation table with a boolean.
	var id string
	for k, op := range partnerOperations {
		if op.Method == "GET" && op.Auth == AuthToken && len(op.Query) > 0 && !strings.Contains(op.Path, "{") {
			id = k
			break
		}
	}
	op := partnerOperations[id]
	if _, err := sr.API.call(ctx, op, map[string]any{op.Query[0]: Bool(true)}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if f.calls[1].Path != op.Path || f.calls[1].Query.Get(op.Query[0]) != "1" {
		t.Fatal(f.calls[1])
	}
}

func TestPublicCallsSendTheAppKeyAndTheClientCode(t *testing.T) {
	f := fixed(reply{status: 200, body: map[string]any{"result": true}})
	sr := partner(t, f, PartnerOptions{ClientCode: "k3m9q2", AppKey: "sp_appkey_abc"})
	if _, err := sr.API.PlatformAppBootstrap(ctx, PlatformAppBootstrapParams{Platform: String("ios")}); err != nil {
		t.Fatal(err)
	}
	c := f.calls[0]
	if c.Path != "/app/v1/clients/k3m9q2/bootstrap" || c.Query.Get("platform") != "ios" || c.Headers.Get("X-App-Key") != "sp_appkey_abc" || c.Headers.Get("Authorization") != "" {
		t.Fatal(c)
	}
}

func TestACallWithoutASessionIsRefusedBeforeAnyRequest(t *testing.T) {
	f := fixed()
	sr := partner(t, f, PartnerOptions{})
	_, err := sr.API.PlatformAppNotifications(ctx, PlatformAppNotificationsParams{})
	e := asError(t, err)
	if e.Code != "invalid_token" || !e.SignInRequired() || f.count() != 0 {
		t.Fatal(e)
	}
}

func TestA401RefreshesOnceAndConcurrentCallsShareIt(t *testing.T) {
	store := signedIn(time.Hour)
	var mu sync.Mutex
	refreshes := 0
	release := make(chan struct{})
	handler := func(c recorded) reply {
		if c.Path == "/app/oauth/token" {
			mu.Lock()
			refreshes++
			mu.Unlock()
			<-release
			return reply{status: 200, body: tokenBody("2")}
		}
		if c.Headers.Get("Authorization") == "Bearer sp_app_access2" {
			return reply{status: 200, body: map[string]any{"result": true}}
		}
		return reply{status: 401, body: map[string]any{"error": "invalid_token"}}
	}
	f := &fake{}
	for i := 0; i < 6; i++ {
		f.handlers = append(f.handlers, handler)
	}
	sr := partner(t, f, PartnerOptions{TokenStore: store})
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = sr.API.PlatformAppNotifications(ctx, PlatformAppNotificationsParams{})
		}(i)
	}
	// Let both calls reach their 401 and the refresh before it answers.
	deadline := time.Now().Add(2 * time.Second)
	for f.count() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if refreshes != 1 {
		t.Fatalf("refreshes = %d", refreshes)
	}
	tokens, _ := store.Get(ctx)
	if tokens.RefreshToken != "sp_app_refresh2" {
		t.Fatal(tokens)
	}
	for _, c := range f.calls {
		if c.Path == "/app/oauth/token" {
			equalMaps(t, formMap(c.Form), map[string]string{"client_id": "p_a", "grant_type": "refresh_token", "refresh_token": "r1"})
		}
	}
}

func TestATokenAboutToExpireIsRefreshedFirst(t *testing.T) {
	f := fixed(reply{status: 200, body: tokenBody("3")}, reply{status: 200, body: map[string]any{"result": true}})
	sr := partner(t, f, PartnerOptions{TokenStore: signedIn(10 * time.Second)})
	if _, err := sr.API.PlatformAppNotifications(ctx, PlatformAppNotificationsParams{}); err != nil {
		t.Fatal(err)
	}
	if f.calls[0].Path != "/app/oauth/token" || f.calls[1].Headers.Get("Authorization") != "Bearer sp_app_access3" {
		t.Fatal(f.calls)
	}
}

func TestARefusedRefreshEndsTheSessionAndTellsTheApp(t *testing.T) {
	store := signedIn(time.Hour)
	signedOut := 0
	f := fixed(reply{status: 401, body: map[string]any{"error": "invalid_token"}}, reply{status: 400, body: map[string]any{"error": "invalid_grant"}})
	sr := partner(t, f, PartnerOptions{TokenStore: store, OnSignedOut: func() { signedOut++ }})
	_, err := sr.API.PlatformAppNotifications(ctx, PlatformAppNotificationsParams{})
	e := asError(t, err)
	if e.Code != "invalid_grant" || !e.SignInRequired() {
		t.Fatal(e)
	}
	if tokens, _ := store.Get(ctx); tokens != nil || signedOut != 1 {
		t.Fatal(tokens, signedOut)
	}
}

func TestSignOutRevokesAndForgets(t *testing.T) {
	f := fixed(reply{status: 200, body: map[string]any{}})
	sr := partner(t, f, PartnerOptions{TokenStore: signedIn(time.Hour)})
	if err := sr.SignOut(ctx); err != nil {
		t.Fatal(err)
	}
	if f.calls[0].Path != "/app/oauth/revoke" {
		t.Fatal(f.calls[0])
	}
	equalMaps(t, formMap(f.calls[0].Form), map[string]string{"token": "r1", "client_id": "p_a"})
	if in, _ := sr.IsSignedIn(ctx); in {
		t.Fatal("still signed in")
	}
}

func TestRepeatableCallsAreRetriedAndWritesAreNot(t *testing.T) {
	f := fixed(reply{status: 503}, reply{network: true}, reply{status: 200, body: map[string]any{"result": true}}, reply{status: 503, body: map[string]any{"message": "Down"}})
	two := 2
	sr := partner(t, f, PartnerOptions{TokenStore: signedIn(time.Hour), Retries: &two})
	got, err := sr.API.PlatformAppNotifications(ctx, PlatformAppNotificationsParams{})
	if err != nil || got.(map[string]any)["result"] != true || f.count() != 3 {
		t.Fatal(got, err, f.count())
	}
	_, err = sr.API.PlatformAppMarkAllNotificationsRead(ctx)
	if asError(t, err).Status != 503 || f.count() != 4 {
		t.Fatal(err, f.count())
	}
}

func TestPlatformAppErrorCodesReachTheCaller(t *testing.T) {
	f := fixed(reply{status: 403, body: map[string]any{"result": false, "message": "APP_ACCESS_DENIED"}, headers: map[string]string{"X-Request-Id": "req-1"}})
	sr := partner(t, f, PartnerOptions{TokenStore: signedIn(time.Hour)})
	_, err := sr.API.PlatformAppNotifications(ctx, PlatformAppNotificationsParams{})
	e := asError(t, err)
	if e.Code != "APP_ACCESS_DENIED" || e.RequestID != "req-1" {
		t.Fatal(e)
	}
}

func TestGraphQLSendsTheQueryAndSurfacesErrors(t *testing.T) {
	f := fixed(reply{status: 200, body: map[string]any{"data": map[string]any{"partnerAppMe": map[string]any{"app_code": "p_a"}}}}, reply{status: 200, body: map[string]any{"errors": []any{map[string]any{"message": "Unauthorized"}}}})
	sr := partner(t, f, PartnerOptions{TokenStore: signedIn(time.Hour)})
	data, err := sr.GraphQL(ctx, "{ partnerAppMe { app_code } }", nil)
	if err != nil || data.(map[string]any)["partnerAppMe"].(map[string]any)["app_code"] != "p_a" {
		t.Fatal(data, err)
	}
	if f.calls[0].Path != "/app/graphql" || f.calls[0].JSON["query"] != "{ partnerAppMe { app_code } }" || len(f.calls[0].JSON["variables"].(map[string]any)) != 0 {
		t.Fatal(f.calls[0])
	}
	_, err = sr.GraphQL(ctx, "{ x }", nil)
	if e := asError(t, err); e.Code != "graphql_error" || e.Message != "Unauthorized" {
		t.Fatal(e)
	}
}

func TestTheOpenClientAsksGetTokenForEveryCall(t *testing.T) {
	asked := 0
	f := fixed(reply{status: 200, body: map[string]any{"ok": true}}, reply{status: 200, body: map[string]any{"ok": true}})
	open, err := NewOpenClient(OpenOptions{GetToken: func(context.Context) (string, error) { asked++; return "tok" + string(rune('0'+asked)), nil }, HTTPClient: &http.Client{Transport: f}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := open.API.CapabilitiesIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := open.API.CapabilitiesIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if f.calls[0].Path != openOperations["capabilitiesIndex"].Path || f.calls[0].Headers.Get("Authorization") != "Bearer tok1" || f.calls[1].Headers.Get("Authorization") != "Bearer tok2" {
		t.Fatal(f.calls)
	}
	if _, err := NewOpenClient(OpenOptions{}); err == nil {
		t.Fatal("no credentials accepted")
	}
}

func TestOpenClientWithAnAPIKeyAndAJSONBody(t *testing.T) {
	f := fixed(reply{status: 200, body: map[string]any{"ok": true}})
	open, _ := NewOpenClient(OpenOptions{APIKey: "sk_live", HTTPClient: &http.Client{Transport: f}})
	if _, err := open.API.GeoForward(ctx, GeoForwardParams{Body: map[string]any{"address": "1 Main St"}}); err != nil {
		t.Fatal(err)
	}
	if f.calls[0].Method != "POST" || f.calls[0].Headers.Get("Authorization") != "Bearer sk_live" || f.calls[0].JSON["address"] != "1 Main St" {
		t.Fatal(f.calls[0])
	}
}

func TestThemeTokensFollowTheBranding(t *testing.T) {
	dark := ThemeFromBranding(map[string]any{"primary_color": "#1a3d7c", "display_name": "Acme"})
	if dark.Color["primary"] != "#1A3D7C" || dark.Color["onPrimary"] != "#FFFFFF" || dark.DisplayName != "Acme" {
		t.Fatal(dark)
	}
	if ThemeFromBranding(map[string]any{"primary_color": "#FFE066"}).Color["onPrimary"] != "#111111" {
		t.Fatal("light")
	}
	if ThemeFromBranding(map[string]any{"primary_color": "red"}).Color["primary"] != "#E02020" {
		t.Fatal("fallback")
	}
}

func TestSubscribersHearTheSessionStartAndEnd(t *testing.T) {
	f := fixed(reply{status: 200, body: tokenBody("1")}, reply{status: 200, body: tokenBody("2")}, reply{status: 200, body: map[string]any{}})
	sr := partner(t, f, PartnerOptions{})
	var heard []bool
	off := sr.OnSessionChange(func(in bool) { heard = append(heard, in) })
	if _, err := sr.SignInWithIDToken(ctx, "t", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := sr.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	_ = sr.SignOut(ctx)
	off()
	_ = sr.SignOut(ctx)
	if len(heard) != 2 || heard[0] != true || heard[1] != false {
		t.Fatal(heard)
	}
}

func TestBrowserSignInChecksTheState(t *testing.T) {
	f := fixed(reply{status: 200, body: tokenBody("1")})
	sr := partner(t, f, PartnerOptions{})
	redirect := "com.acme.parcels:/oauth"
	req, _ := sr.AuthorizationRequest(redirect, nil, "")
	tokens, err := sr.CompleteSignIn(ctx, req, redirect+"?code=xyz&state="+url.QueryEscape(req.State))
	if err != nil || tokens.AccessToken != "sp_app_access1" || f.calls[0].Form.Get("code") != "xyz" {
		t.Fatal(tokens, err)
	}
	_, err = sr.CompleteSignIn(ctx, req, redirect+"?code=xyz&state=forged")
	if asError(t, err).Code != "invalid_state" {
		t.Fatal(err)
	}
	_, err = sr.CompleteSignIn(ctx, req, redirect+"?error=access_denied&state="+url.QueryEscape(req.State))
	if asError(t, err).Code != "access_denied" {
		t.Fatal(err)
	}
}

func TestURLBuilding(t *testing.T) {
	op := Operation{Method: "GET", Path: "/app/v1/x/{id}/{opt}", Query: []string{"ids", "n", "f", "missing"}}
	u, err := buildURL("https://h", op, map[string]any{"id": "a b/c", "opt": (*string)(nil), "ids": []int64{1, 2}, "n": Float(3), "f": 2.5})
	if err != nil {
		t.Fatal(err)
	}
	if u != "https://h/app/v1/x/a%20b%2Fc?f=2.5&ids=1&ids=2&n=3" {
		t.Fatal(u)
	}
}

func TestMultipartSendsFieldsListsAndFiles(t *testing.T) {
	body, ct, err := encodeBody(BodyMultipart, map[string]any{"note": "hi", "tags": []string{"a", "b"}, "skip": nil}, []File{{Field: "photo", Content: []byte("PNG"), Filename: "p.png", ContentType: "image/png"}})
	if err != nil || !strings.HasPrefix(ct, "multipart/form-data; boundary=") {
		t.Fatal(ct, err)
	}
	s := string(body)
	for _, want := range []string{`name="note"`, `name="tags[]"`, `filename="p.png"`, "image/png", "PNG"} {
		if !strings.Contains(s, want) {
			t.Fatal(want, s)
		}
	}
	if strings.Contains(s, `name="skip"`) {
		t.Fatal(s)
	}
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestHeaderParametersAreSentAndNeverReplaceAuthorizationOrTheAppKey(t *testing.T) {
	f := fixed(reply{status: 200, body: map[string]any{"result": true}}, reply{status: 200, body: map[string]any{"result": true}})
	sr := partner(t, f, PartnerOptions{ClientCode: "k3m9q2", AppKey: "sp_appkey_1", TokenStore: signedIn(time.Hour)})
	if _, err := sr.API.PlatformAppGuestStoreCart(ctx, PlatformAppGuestStoreCartParams{Slug: "shop", GuestCartToken: String("cart-1")}); err != nil {
		t.Fatal(err)
	}
	c := f.calls[0]
	if c.URI != "/app/v1/clients/k3m9q2/guest/stores/shop/cart" || c.Headers.Get("X-Guest-Cart-Token") != "cart-1" || c.Headers.Get("X-App-Key") != "sp_appkey_1" {
		t.Fatal(c)
	}
	if _, err := sr.API.PlatformAppWalletCreateTopUp(ctx, PlatformAppWalletCreateTopUpParams{Body: map[string]any{"amount": 10}, IdempotencyKey: String("key-1")}); err != nil {
		t.Fatal(err)
	}
	c = f.calls[1]
	if c.Headers.Get("Idempotency-Key") != "key-1" || c.Headers.Get("Authorization") != "Bearer old" || c.Headers.Get("X-SApp") != "go/"+Version {
		t.Fatal(c)
	}
	// A header the SDK sets is never replaced, even by an operation that names it.
	op := Operation{Method: "GET", Path: "/app/v1/wallet", Auth: AuthToken, Headers: []string{"Authorization", "X-Test"}}
	f.handlers = append(f.handlers, func(recorded) reply { return reply{status: 200, body: map[string]any{}} })
	if _, err := sr.API.call(ctx, op, map[string]any{"Authorization": String("Bearer forged"), "X-Test": "t"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if c := f.calls[2]; c.Headers.Get("Authorization") != "Bearer old" || c.Headers.Get("X-Test") != "t" {
		t.Fatal(c)
	}
}

func TestAnIdempotencyKeyIsMadePerCallAndKeptWhenTheCallIsSentAgain(t *testing.T) {
	f := fixed(
		reply{status: 401, body: map[string]any{"error": "invalid_token"}},
		reply{status: 200, body: tokenBody("2")},
		reply{status: 200, body: map[string]any{"result": true}},
		reply{status: 200, body: map[string]any{"result": true}},
	)
	sr := partner(t, f, PartnerOptions{TokenStore: signedIn(time.Hour)})
	if _, err := sr.API.PlatformAppWalletCreateTopUp(ctx, PlatformAppWalletCreateTopUpParams{Body: map[string]any{"amount": 10}}); err != nil {
		t.Fatal(err)
	}
	key := f.calls[0].Headers.Get("Idempotency-Key")
	if !uuidPattern.MatchString(key) || f.calls[2].Headers.Get("Idempotency-Key") != key || f.calls[2].Headers.Get("Authorization") != "Bearer sp_app_access2" {
		t.Fatal(key, f.calls)
	}
	if f.calls[1].Headers.Get("Idempotency-Key") != "" {
		t.Fatal(f.calls[1])
	}
	if _, err := sr.API.PlatformAppWalletCreateTopUp(ctx, PlatformAppWalletCreateTopUpParams{Body: map[string]any{"amount": 10}}); err != nil {
		t.Fatal(err)
	}
	if f.calls[3].Headers.Get("Idempotency-Key") == key {
		t.Fatal("a new call must get a new key")
	}
}

func TestTheExtraQueryIsAppendedAfterTheDeclaredParametersAndNeverReplacesOne(t *testing.T) {
	f := fixed(reply{status: 200, body: map[string]any{"result": true}}, reply{status: 200, body: map[string]any{"result": true}})
	sr := partner(t, f, PartnerOptions{TokenStore: signedIn(time.Hour)})
	if _, err := sr.API.PlatformAppWallet(ctx, PlatformAppWalletParams{Locale: String("fr")},
		WithExtraQuery(map[string]any{"locale": "de", "multiple": 1, "flag": true, "skip": nil})); err != nil {
		t.Fatal(err)
	}
	if c := f.calls[0]; c.URI != "/app/v1/wallet?locale=fr&flag=1&multiple=1" {
		t.Fatal(c.URI)
	}
	if _, err := sr.API.CustomerAddressByPostcodePostcodeGet(ctx, CustomerAddressByPostcodePostcodeGetParams{Postcode: "M5V 2T6"}, WithExtraQuery(map[string]any{"multiple": 1})); err != nil {
		t.Fatal(err)
	}
	if c := f.calls[1]; c.URI != "/app/v1/address-by-postcode/M5V%202T6?multiple=1" {
		t.Fatal(c.URI)
	}
}

func TestResultFalseIsRaisedEvenWithStatus200AndOtherAnswersAreReturned(t *testing.T) {
	refusal := map[string]any{
		"result": false, "code": "LABEL_CANCEL_FAILED", "message": "The label could not be cancelled.",
		"errors":  map[string]any{"label": []any{"The carrier refused the cancel."}, "amount": 5},
		"reasons": []any{"carrier_refused"}, "data": map[string]any{"order_id": 9},
	}
	f := fixed(
		reply{status: 200, body: refusal, headers: map[string]string{"X-Request-Id": "req-9"}},
		reply{status: 200, body: map[string]any{"result": true, "data": 1}},
		reply{status: 200, body: map[string]any{"items": []any{}}},
		reply{status: 200, body: []any{1, 2}},
	)
	sr := partner(t, f, PartnerOptions{TokenStore: signedIn(time.Hour)})
	_, err := sr.API.PlatformAppWallet(ctx, PlatformAppWalletParams{})
	e := asError(t, err)
	if e.Status != 200 || e.Code != "LABEL_CANCEL_FAILED" || e.Message != "The label could not be cancelled." || e.RequestID != "req-9" {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(e.Errors, map[string][]string{"label": {"The carrier refused the cancel."}, "amount": {"5"}}) {
		t.Fatal(e.Errors)
	}
	if !reflect.DeepEqual(e.Reasons, []any{"carrier_refused"}) || !reflect.DeepEqual(e.Data, map[string]any{"order_id": float64(9)}) {
		t.Fatal(e.Reasons, e.Data)
	}
	if b, _ := e.Body.(map[string]any); b["result"] != false {
		t.Fatal(e.Body)
	}
	for _, want := range []any{map[string]any{"result": true, "data": float64(1)}, map[string]any{"items": []any{}}, []any{float64(1), float64(2)}} {
		got, err := sr.API.PlatformAppWallet(ctx, PlatformAppWalletParams{})
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatal(got, err)
		}
	}
}

func TestAHandMadeOperationThroughCallOperationBehavesLikeAGeneratedOne(t *testing.T) {
	// A copy of the generated top-up operation, and an operation the generated API does not have.
	topUp := Operation{Method: "POST", Path: "/app/v1/wallet/top-ups", Auth: AuthToken, Body: BodyJSON, Headers: []string{"Idempotency-Key"}}
	handMade := Operation{Method: "GET", Path: "/app/v1/clients/{clientCode}/guest/things/{id}", Auth: AuthTokenOrKey, Query: []string{"status"}}
	f := fixed(
		reply{status: 200, body: map[string]any{"result": true}},
		reply{status: 401, body: map[string]any{"error": "invalid_token"}},
		reply{status: 200, body: tokenBody("2")},
		reply{status: 200, body: map[string]any{"result": true, "data": 7}},
		reply{status: 200, body: map[string]any{"result": true}},
		reply{status: 200, body: map[string]any{"result": false, "code": "NOPE", "message": "Refused."}},
	)
	sr := partner(t, f, PartnerOptions{TokenStore: signedIn(time.Hour), AppKey: "sp_appkey_1"})
	if _, err := sr.API.PlatformAppWalletCreateTopUp(ctx, PlatformAppWalletCreateTopUpParams{Body: map[string]any{"amount": 10}, IdempotencyKey: String("key-1")}); err != nil {
		t.Fatal(err)
	}
	answer, err := sr.CallOperation(ctx, topUp, nil, map[string]any{"amount": 10}, nil, WithExtraQuery(map[string]any{"x": 1}))
	if err != nil || answer.(map[string]any)["data"] != float64(7) {
		t.Fatal(answer, err)
	}
	generated, first, retried := f.calls[0], f.calls[1], f.calls[3]
	if first.Method != generated.Method || first.Path != generated.Path || !reflect.DeepEqual(first.JSON, generated.JSON) || first.URI != "/app/v1/wallet/top-ups?x=1" {
		t.Fatal(first, generated)
	}
	if first.Headers.Get("Authorization") != "Bearer old" || retried.Headers.Get("Authorization") != "Bearer sp_app_access2" {
		t.Fatal(first.Headers, retried.Headers)
	}
	key := first.Headers.Get("Idempotency-Key")
	if !uuidPattern.MatchString(key) || retried.Headers.Get("Idempotency-Key") != key {
		t.Fatal(key, retried.Headers)
	}
	if _, err := sr.CallOperation(ctx, handMade, map[string]any{"id": 5, "status": "open"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if f.calls[4].URI != "/app/v1/clients/c/guest/things/5?status=open" {
		t.Fatal(f.calls[4].URI)
	}
	_, err = sr.CallOperation(ctx, handMade, map[string]any{"id": 5}, nil, nil)
	if e := asError(t, err); e.Status != 200 || e.Code != "NOPE" {
		t.Fatal(e)
	}
}

func TestTheOpenClientCallOperationGoesThroughTheSameCall(t *testing.T) {
	f := fixed(reply{status: 200, body: map[string]any{"ok": true}})
	open, _ := NewOpenClient(OpenOptions{APIKey: "sk_open", HTTPClient: &http.Client{Transport: f}})
	op := Operation{Method: "GET", Path: "/v1/capabilities", Auth: AuthToken}
	if _, err := open.CallOperation(ctx, op, nil, nil, nil, WithExtraQuery(map[string]any{"a": "b"})); err != nil {
		t.Fatal(err)
	}
	if f.calls[0].URI != "/v1/capabilities?a=b" || f.calls[0].Headers.Get("Authorization") != "Bearer sk_open" {
		t.Fatal(f.calls[0])
	}
}
