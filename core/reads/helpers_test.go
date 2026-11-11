package reads

import (
	"encoding/json"
	"os"
	"testing"

	"flowdeck/core/boot"
)

// setSource loads the fixture source (source only — no boot: the specs
// are code, the helper reads need no launch).
func setSource(t *testing.T) {
	t.Helper()
	if err := boot.SetWorkflowsSource(os.DirFS("../testworkflows")); err != nil {
		t.Fatalf("set source: %v", err)
	}
}

// inputValues of a workflow the source does not spec: no display values
// (the stored value is not guessed onto an unknown input).
func TestInputValuesWithoutASpec(t *testing.T) {
	setSource(t)
	if in := inputValues(42, "ghost"); in != nil {
		t.Fatalf("inputValues without spec: got %+v, want none", in)
	}
}

// A raw stored input that is not JSON text renders with fmt.Sprint (the
// last resort — the value is shown as stored, not decoded).
func TestInputValuesFallsBackToSprint(t *testing.T) {
	setSource(t)
	in := inputValues(42, "live-test")
	if len(in) != 1 || in[0].Name != "mode" || in[0].Value != "42" {
		t.Fatalf("inputValues raw: got %+v, want [{mode 42}]", in)
	}
}

// A simple input stored as unquoted JSON (a number) renders bare.
func TestInputValuesOfUnquotedJSON(t *testing.T) {
	setSource(t)
	in := inputValues("42", "live-test")
	if len(in) != 1 || in[0].Name != "mode" || in[0].Value != "42" {
		t.Fatalf("inputValues unquoted: got %+v, want [{mode 42}]", in)
	}
}

func TestInputOfAnUnknownWorkflow(t *testing.T) {
	setSource(t)
	if _, ok := inputOf("ghost"); ok {
		t.Fatal("inputOf unknown: got ok, want not-ok")
	}
}

func TestStoredMapRejectsBadJSON(t *testing.T) {
	if _, ok := storedMap(`{bad`); ok {
		t.Fatal("storedMap bad json: got ok, want not-ok")
	}
}

// structInputValues without a spec: the stored keys, sorted by name
// (the spec is unavailable — the values are still shown).
func TestStructInputValuesWithoutASpec(t *testing.T) {
	setSource(t)
	got := structInputValues(map[string]any{"b": "2", "a": "1"}, "ghost")
	want := []InputValue{{Name: "a", Value: "1"}, {Name: "b", Value: "2"}}
	if len(got) != len(want) {
		t.Fatalf("structInputValues: got %+v, want %v", got, want)
	}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("structInputValues[%d]: got %+v, want %+v", i, got[i], v)
		}
	}
}

// structFields of a simple-input workflow: no struct fields (a struct
// input has several fields — the spec is not a struct).
func TestStructFieldsOfASimpleInputWorkflow(t *testing.T) {
	setSource(t)
	if _, ok := structFields("live-test"); ok {
		t.Fatal("structFields simple input: got ok, want not-ok")
	}
}

func TestZeroOf(t *testing.T) {
	if s := zeroOf("string"); s != "" {
		t.Fatalf("zeroOf string: got %q, want empty", s)
	}
	if s := zeroOf("int"); s != "0" {
		t.Fatalf("zeroOf int: got %q, want 0", s)
	}
}

func TestQuotedStringRejectsANonString(t *testing.T) {
	if _, ok := quotedString("abc"); ok {
		t.Fatal("quotedString non-string: got ok, want not-ok")
	}
}

func TestQuotedStringRejectsBadJSON(t *testing.T) {
	if _, ok := quotedString(`"bad`); ok {
		t.Fatal("quotedString bad json: got ok, want not-ok")
	}
}

func TestJSONText(t *testing.T) {
	if s, ok := jsonText([]byte("x")); !ok || s != "x" {
		t.Fatalf("jsonText bytes: got %q,%v, want x,true", s, ok)
	}
	if s, ok := jsonText(json.RawMessage("x")); !ok || s != "x" {
		t.Fatalf("jsonText raw: got %q,%v, want x,true", s, ok)
	}
	if s, ok := jsonText("x"); !ok || s != "x" {
		t.Fatalf("jsonText string: got %q,%v, want x,true", s, ok)
	}
	if _, ok := jsonText(42); ok {
		t.Fatal("jsonText number: got ok, want not-ok")
	}
}
