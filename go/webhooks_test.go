package sapp

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"
)

type vector struct {
	Name      string            `json:"name"`
	Scheme    string            `json:"scheme"`
	Secrets   []string          `json:"secrets"`
	Body      string            `json:"body"`
	Headers   map[string]string `json:"headers"`
	Now       int64             `json:"now"`
	Tolerance *int64            `json:"tolerance"`
	Expect    struct {
		OK    bool           `json:"ok"`
		Error string         `json:"error"`
		Event map[string]any `json:"event"`
	} `json:"expect"`
}

func loadVectors(t *testing.T) []vector {
	t.Helper()
	raw, err := os.ReadFile("testdata/webhooks/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Vectors []vector `json:"vectors"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Vectors
}

func optional[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

func eventMap(e *WebhookEvent) map[string]any {
	num := func(p *int64) any {
		if p == nil {
			return nil
		}
		return float64(*p)
	}
	return map[string]any{
		"id": optional(e.ID), "type": optional(e.Type), "time": optional(e.Time), "subject": optional(e.Subject),
		"source": optional(e.Source), "business": num(e.Business), "attempt": num(e.Attempt), "format": e.Format, "data": e.Data,
	}
}

func run(v vector, viaHeader bool) (*WebhookEvent, error) {
	opts := WebhookOptions{Scheme: v.Scheme, Now: time.Unix(v.Now, 0)}
	if v.Tolerance != nil {
		opts.Tolerance = time.Duration(*v.Tolerance) * time.Second
	}
	if viaHeader {
		h := http.Header{}
		for k, x := range v.Headers {
			h.Set(k, x)
		}
		return VerifyWebhook([]byte(v.Body), h, v.Secrets, opts)
	}
	return VerifyWebhookMap([]byte(v.Body), v.Headers, v.Secrets, opts)
}

func TestWebhookVectors(t *testing.T) {
	for _, v := range loadVectors(t) {
		for _, viaHeader := range []bool{false, true} {
			v, viaHeader := v, viaHeader
			t.Run(v.Name, func(t *testing.T) {
				event, err := run(v, viaHeader)
				if v.Expect.OK {
					if err != nil {
						t.Fatal(err)
					}
					if got := eventMap(event); !reflect.DeepEqual(got, v.Expect.Event) {
						t.Fatalf("got %#v\nwant %#v", got, v.Expect.Event)
					}
					if event.Payload == nil {
						t.Fatal("no payload")
					}
					return
				}
				var e *WebhookVerificationError
				if !errors.As(err, &e) || e.Code != v.Expect.Error {
					t.Fatalf("want %s, got %v", v.Expect.Error, err)
				}
			})
		}
	}
}

// The module carries its own copy of the shared vectors so it can be published on its own; inside the sApp
// repository the copy must match the shared file.
func TestWebhookVectorsMatchTheSharedCopy(t *testing.T) {
	shared, err := os.ReadFile("../testdata/webhooks/vectors.json")
	if err != nil {
		t.Skip("not inside the sApp repository")
	}
	local, err := os.ReadFile("testdata/webhooks/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(shared, local) {
		t.Fatal("go/testdata/webhooks/vectors.json differs from testdata/webhooks/vectors.json: run python3 -I tools/webhook_vectors.py")
	}
}
