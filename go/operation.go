package sapp

import "context"

// Auth says how an operation authenticates.
type Auth int

const (
	// AuthNone sends no credentials.
	AuthNone Auth = iota
	// AuthToken needs the signed-in person's access token.
	AuthToken
	// AuthKey sends the app key (calls before sign-in).
	AuthKey
	// AuthTokenOrKey sends the token when signed in, else the app key.
	AuthTokenOrKey
)

// BodyKind says how an operation's body is sent.
type BodyKind int

const (
	// BodyNone sends no body.
	BodyNone BodyKind = iota
	// BodyJSON sends the body as JSON.
	BodyJSON
	// BodyForm sends the body as application/x-www-form-urlencoded.
	BodyForm
	// BodyMultipart sends the body and files as multipart/form-data.
	BodyMultipart
)

// Operation is one operation of a generated API. Headers names the header parameters it declares (Idempotency-Key,
// X-Guest-Order-Token, …); their values travel in the parameters under the header's own name.
type Operation struct {
	Method  string
	Path    string
	Auth    Auth
	Query   []string
	Body    BodyKind
	Headers []string
}

// CallOption changes one call of a generated method.
type CallOption func(*callOptions)

type callOptions struct {
	extraQuery map[string]any
}

// WithExtraQuery adds query parameters the spec does not declare (e.g. multiple=1). They are appended after the
// declared parameters and never replace one; nil values are left out, booleans are sent as 1 / 0.
func WithExtraQuery(q map[string]any) CallOption {
	return func(o *callOptions) {
		if o.extraQuery == nil {
			o.extraQuery = map[string]any{}
		}
		for k, v := range q {
			o.extraQuery[k] = v
		}
	}
}

func optionsOf(opts []CallOption) callOptions {
	var o callOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

// File is a file sent with a multipart operation (photos, signatures).
type File struct {
	Field       string
	Content     []byte
	Filename    string
	ContentType string
}

// operationCall is what a generated API calls for each operation: the path, query and header values, the body, files
// and the call's options.
type operationCall func(ctx context.Context, op Operation, params map[string]any, body map[string]any, files []File, opts ...CallOption) (any, error)

// String answers a pointer to v, for optional parameters.
func String(v string) *string { return &v }

// Int answers a pointer to v, for optional parameters.
func Int(v int64) *int64 { return &v }

// Float answers a pointer to v, for optional parameters.
func Float(v float64) *float64 { return &v }

// Bool answers a pointer to v, for optional parameters.
func Bool(v bool) *bool { return &v }
