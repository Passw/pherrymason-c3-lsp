package search

// Tests for multi-segment import completions (e.g. "import std::core::mem").
//
// Two separate problems are covered here:
//
//  1. Module-path extraction from a partial separator sequence.
//     Editors trigger completion on the first ':' of '::', so the source text
//     may only contain "module:" (single colon) when the request arrives.
//     extractExplicitModulePath must handle both "module:" and "module::".
//
//  2. Cross-module contamination.
//     Stdlib modules are indexed one UnitModules per source file.
//     Typing "dstring::" must NOT return symbols from unrelated modules
//     (e.g. std::time constants).

import (
	"testing"

	"github.com/pherrymason/c3-lsp/internal/lsp/context"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// ---------------------------------------------------------------------------
// extractExplicitModulePath unit tests
// ---------------------------------------------------------------------------

func TestExtractExplicitModulePath_double_colon(t *testing.T) {
	cases := []struct {
		input    string
		wantSome bool
		wantName string
	}{
		{"mem::", true, "mem"},
		{"dstring::", true, "dstring"},
		{"std::core::mem::", true, "std::core::mem"},
		{"", false, ""},
		{"mem", false, ""},
		{".", false, ""},
		{"a.b::", false, ""},  // chain + module — no module path before the dot
	}

	for _, tc := range cases {
		got := extractExplicitModulePath(tc.input)
		if got.IsSome() != tc.wantSome {
			t.Errorf("extractExplicitModulePath(%q): IsSome=%v want %v", tc.input, got.IsSome(), tc.wantSome)
			continue
		}
		if tc.wantSome && got.Get().GetName() != tc.wantName {
			t.Errorf("extractExplicitModulePath(%q): name=%q want %q", tc.input, got.Get().GetName(), tc.wantName)
		}
	}
}

func TestExtractExplicitModulePath_single_colon_trigger(t *testing.T) {
	// Editors fire on the first ':' so source only has one colon.
	cases := []struct {
		input    string
		wantSome bool
		wantName string
	}{
		{"mem:", true, "mem"},
		{"dstring:", true, "dstring"},
		{"std::core::mem:", true, "std::core::mem"},
		{":", false, ""},      // nothing before the colon
		{"a.b:", false, ""},   // chain — no pure module path
	}

	for _, tc := range cases {
		got := extractExplicitModulePath(tc.input)
		if got.IsSome() != tc.wantSome {
			t.Errorf("extractExplicitModulePath(%q): IsSome=%v want %v", tc.input, got.IsSome(), tc.wantSome)
			continue
		}
		if tc.wantSome && got.Get().GetName() != tc.wantName {
			t.Errorf("extractExplicitModulePath(%q): name=%q want %q", tc.input, got.Get().GetName(), tc.wantName)
		}
	}
}

// ---------------------------------------------------------------------------
// Completion integration tests (uses inline mock modules, no real stdlib)
// ---------------------------------------------------------------------------

// completionsForSource registers two docs (a library and an app), places the
// cursor at the ||| marker, and returns the non-keyword completion labels.
func completionsForSource(libDoc, libSrc, appSrc string) []string {
	state := NewTestState()
	state.registerDoc(libDoc, libSrc)

	body, pos := parseBodyWithCursor(appSrc)
	state.registerDoc("app.c3", body)

	search := NewSearchWithoutLog()
	items := filterOutKeywordSuggestions(search.BuildCompletionList(
		context.CursorContext{Position: pos, DocURI: "app.c3"},
		&state.state))

	labels := make([]string, 0, len(items))
	for _, it := range items {
		labels = append(labels, it.Label)
	}
	return labels
}

func TestCompletion_multiSegmentImport_doubleColon(t *testing.T) {
	labels := completionsForSource(
		"mem.c3",
		`module std::core::mem;
fn void* malloc(usz size) {}
fn void free(void* ptr) {}
`,
		`module app;
import std::core::mem;

fn void test() {
	mem::|||
}
`)
	assertContains(t, labels, "malloc")
	assertContains(t, labels, "free")
}

func TestCompletion_multiSegmentImport_singleColonTrigger(t *testing.T) {
	// Source has only one colon — simulates editor triggering on first ':'
	labels := completionsForSource(
		"mem.c3",
		`module std::core::mem;
fn void* malloc(usz size) {}
fn void free(void* ptr) {}
`,
		`module app;
import std::core::mem;

fn void test() {
	mem:|||
}
`)
	assertContains(t, labels, "malloc")
	assertContains(t, labels, "free")
}

func TestCompletion_multiSegmentImport_noContamination(t *testing.T) {
	// dstring:: must NOT return symbols from unrelated modules
	state := NewTestState()
	state.registerDoc("dstring.c3", `module std::core::dstring;
fn void dstring_fn() {}
`)
	state.registerDoc("time.c3", `module std::time;
const long NANO_DURATION_ZERO = 0;
fn void time_fn() {}
`)

	body, pos := parseBodyWithCursor(`module app;
import std::core::dstring;

fn void test() {
	dstring::|||
}
`)
	state.registerDoc("app.c3", body)

	search := NewSearchWithoutLog()
	items := filterOutKeywordSuggestions(search.BuildCompletionList(
		context.CursorContext{Position: pos, DocURI: "app.c3"},
		&state.state))

	labels := make([]string, 0, len(items))
	for _, it := range items {
		labels = append(labels, it.Label)
	}

	assertContains(t, labels, "dstring_fn")
	assertNotContains(t, labels, "NANO_DURATION_ZERO")
	assertNotContains(t, labels, "time_fn")
}

func TestCompletion_multiSegmentImport_singleColonNoContamination(t *testing.T) {
	state := NewTestState()
	state.registerDoc("dstring.c3", `module std::core::dstring;
fn void dstring_fn() {}
`)
	state.registerDoc("time.c3", `module std::time;
const long NANO_DURATION_ZERO = 0;
`)

	body, pos := parseBodyWithCursor(`module app;
import std::core::dstring;

fn void test() {
	dstring:|||
}
`)
	state.registerDoc("app.c3", body)

	search := NewSearchWithoutLog()
	items := filterOutKeywordSuggestions(search.BuildCompletionList(
		context.CursorContext{Position: pos, DocURI: "app.c3"},
		&state.state))

	labels := make([]string, 0, len(items))
	for _, it := range items {
		labels = append(labels, it.Label)
	}

	assertContains(t, labels, "dstring_fn")
	assertNotContains(t, labels, "NANO_DURATION_ZERO")
}

// ---------------------------------------------------------------------------
// Real stdlib integration tests (requires c3c installed via Homebrew)
// ---------------------------------------------------------------------------

const stdlibPath = "/opt/homebrew/Cellar/c3c/0.8.0/lib/c3"

func newStateWithStdlib(t *testing.T) TestState {
	t.Helper()
	state := NewTestState()
	state.state.SetLanguageVersion("0.8.0", stdlibPath)
	return state
}

func stdlibCompletions(t *testing.T, importPath, trigger, appSrc string) []string {
	t.Helper()
	state := newStateWithStdlib(t)

	body, pos := parseBodyWithCursor(appSrc)
	state.registerDoc("app.c3", body)

	lspPos := protocol.Position{Line: uint32(pos.Line), Character: uint32(pos.Character)}
	ctx := context.BuildFromDocumentPosition(lspPos, "app.c3", &state.state)

	search := NewSearchWithoutLog()
	items := filterOutKeywordSuggestions(search.BuildCompletionList(ctx, &state.state))

	labels := make([]string, 0, len(items))
	for _, it := range items {
		labels = append(labels, it.Label)
	}
	return labels
}

func TestStdlib_mem_doubleColon(t *testing.T) {
	labels := stdlibCompletions(t, "std::core::mem", "::", `module app;
import std::core::mem;

fn void test() {
	mem::|||
}
`)
	assertContains(t, labels, "malloc")
	assertNotContains(t, labels, "NANO_DURATION_ZERO") // from std::time
}

func TestStdlib_mem_singleColonTrigger(t *testing.T) {
	labels := stdlibCompletions(t, "std::core::mem", ":", `module app;
import std::core::mem;

fn void test() {
	mem:|||
}
`)
	assertContains(t, labels, "malloc")
	assertNotContains(t, labels, "NANO_DURATION_ZERO")
}

func TestStdlib_dstring_doubleColon(t *testing.T) {
	labels := stdlibCompletions(t, "std::core::dstring", "::", `module app;
import std::core::dstring;

fn void test() {
	dstring::|||
}
`)
	assertContains(t, labels, "temp")
	assertContains(t, labels, "new")
	assertNotContains(t, labels, "NANO_DURATION_ZERO")
}

func TestStdlib_dstring_singleColonTrigger(t *testing.T) {
	labels := stdlibCompletions(t, "std::core::dstring", ":", `module app;
import std::core::dstring;

fn void test() {
	dstring:|||
}
`)
	assertContains(t, labels, "temp")
	assertContains(t, labels, "new")
	assertNotContains(t, labels, "NANO_DURATION_ZERO")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func assertContains(t *testing.T, labels []string, want string) {
	t.Helper()
	for _, l := range labels {
		if l == want {
			return
		}
	}
	t.Errorf("expected %q in completion list; got: %v", want, labels)
}

func assertNotContains(t *testing.T, labels []string, unwanted string) {
	t.Helper()
	for _, l := range labels {
		if l == unwanted {
			t.Errorf("unexpected %q found in completion list", unwanted)
			return
		}
	}
}
