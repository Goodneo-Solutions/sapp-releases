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

// Operation is one operation of a generated API.
type Operation struct {
	Method string
	Path   string
	Auth   Auth
	Query  []string
	Body   BodyKind
}

// File is a file sent with a multipart operation (photos, signatures).
type File struct {
	Field       string
	Content     []byte
	Filename    string
	ContentType string
}

// operationCall is what a generated API calls for each operation: the path and query values, the body and files.
type operationCall func(ctx context.Context, op Operation, params map[string]any, body map[string]any, files []File) (any, error)

// String answers a pointer to v, for optional parameters.
func String(v string) *string { return &v }

// Int answers a pointer to v, for optional parameters.
func Int(v int64) *int64 { return &v }

// Float answers a pointer to v, for optional parameters.
func Float(v float64) *float64 { return &v }

// Bool answers a pointer to v, for optional parameters.
func Bool(v bool) *bool { return &v }
