package search

import (
	"fmt"
	"testing"

	"github.com/pherrymason/c3-lsp/internal/lsp/context"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

const stdlibPathDiag = "/opt/homebrew/Cellar/c3c/0.8.0/lib/c3"

func TestStdlibDiag_imports(t *testing.T) {
	state := NewTestState()
	state.state.SetLanguageVersion("0.8.0", stdlibPathDiag)

	appSrc := `module app;
import std::io;

fn void test() {
	io::|||
}
`
	body, pos := parseBodyWithCursor(appSrc)
	state.registerDoc("app.c3", body)

	lspPos := protocol.Position{Line: uint32(pos.Line), Character: uint32(pos.Character)}
	ctx := context.BuildFromDocumentPosition(lspPos, "app.c3", &state.state)

	fmt.Printf("Cursor pos: line=%d char=%d\n", pos.Line, pos.Character)
	fmt.Printf("CursorContext: pos=%v docURI=%s\n", ctx.Position, ctx.DocURI)

	// Print what the doc text looks like
	doc := state.state.GetDocument("app.c3")
	if doc != nil {
		fmt.Printf("Doc text:\n%s\n", doc.SourceCode.Text)
	}

	// Check what module is at the doc
	um := state.state.GetUnitModulesByDoc("app.c3")
	if um != nil {
		for _, mod := range um.Modules() {
			fmt.Printf("App module: %q imports=%v\n", mod.GetModule().GetName(), mod.Imports)
		}
	}

	search := NewSearchWithoutLog()
	items := filterOutKeywordSuggestions(search.BuildCompletionList(ctx, &state.state))
	fmt.Printf("Completions: %d\n", len(items))
	for i, it := range items {
		if i < 20 {
			fmt.Printf("  %s\n", it.Label)
		}
	}
}
