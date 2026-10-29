package sapp

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var methods = []string{"get", "post", "put", "patch", "delete"}

// specIDs answers the operationIds of a spec, the way tools/generate.py takes them.
func specIDs(t *testing.T, file string, skip func(string) bool) []string {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Skip("the specs are not beside the module")
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for path, ms := range spec.Paths {
		if skip(path) {
			continue
		}
		for _, m := range methods {
			if op, ok := ms[m]; ok {
				var o struct {
					OperationID string `json:"operationId"`
				}
				_ = json.Unmarshal(op, &o)
				ids = append(ids, o.OperationID)
			}
		}
	}
	sort.Strings(ids)
	return ids
}

func keys(m map[string]Operation) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestGeneratedTablesCoverEverySpecOperation(t *testing.T) {
	raw, err := os.ReadFile("../specs/operation-counts.json")
	if err != nil {
		t.Skip("the specs are not beside the module")
	}
	var counts map[string]int
	if err := json.Unmarshal(raw, &counts); err != nil {
		t.Fatal(err)
	}
	if len(PartnerOperations) != counts["partner"] || len(OpenOperations) != counts["open"] {
		t.Fatalf("partner %d/%d open %d/%d", len(PartnerOperations), counts["partner"], len(OpenOperations), counts["open"])
	}
	partner := specIDs(t, "../specs/partner-api.json", func(p string) bool { return strings.HasPrefix(p, "/app/oauth") || p == "/app/graphql" })
	if !reflect.DeepEqual(partner, keys(PartnerOperations)) {
		t.Fatal("partner operations differ from the spec")
	}
	open := specIDs(t, "../specs/open-api.json", func(string) bool { return false })
	if !reflect.DeepEqual(open, keys(OpenOperations)) {
		t.Fatal("open operations differ from the spec")
	}
	if PartnerOperations["platformAppBootstrap"].Auth != AuthKey || PartnerOperations["driverEarnings"].Auth != AuthToken {
		t.Fatal("auth kinds")
	}
}

func TestEveryOperationHasAMethod(t *testing.T) {
	for name, ops := range map[string]map[string]Operation{"PartnerAPI": PartnerOperations, "OpenAPI": OpenOperations} {
		var api reflect.Type
		if name == "PartnerAPI" {
			api = reflect.TypeOf(&PartnerAPI{})
		} else {
			api = reflect.TypeOf(&OpenAPI{})
		}
		if api.NumMethod() != len(ops) {
			t.Fatalf("%s has %d methods for %d operations", name, api.NumMethod(), len(ops))
		}
		for id := range ops {
			if _, ok := api.MethodByName(goName(id)); !ok {
				t.Fatalf("%s lacks %s", name, id)
			}
		}
	}
}

// goName is the generator's Pascal case: parts split on anything but letters and digits, each part capitalised.
func goName(id string) string {
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	})
	out := ""
	for _, p := range parts {
		out += strings.ToUpper(p[:1]) + p[1:]
	}
	return out
}
