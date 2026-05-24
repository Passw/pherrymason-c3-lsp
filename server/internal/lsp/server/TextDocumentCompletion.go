package server

import (
	"strings"

	ctx "github.com/pherrymason/c3-lsp/internal/lsp/context"
	"github.com/pherrymason/c3-lsp/pkg/utils"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// Support "Completion"
// Returns: []CompletionItem | CompletionList | nil
func (h *Server) TextDocumentCompletion(context *glsp.Context, params *protocol.CompletionParams) (any, error) {
	docId := utils.NormalizePath(params.TextDocument.URI)
	h.server.Log.Infof("completion request: doc=%s pos=(%d,%d)", docId, params.Position.Line, params.Position.Character)

	doc := h.state.GetDocument(docId)
	if doc != nil {
		// Log the text around the cursor for diagnosis
		lines := strings.Split(doc.SourceCode.Text, "\n")
		if int(params.Position.Line) < len(lines) {
			line := lines[params.Position.Line]
			col := int(params.Position.Character)
			end := col
			if end > len(line) {
				end = len(line)
			}
			h.server.Log.Infof("  cursor line text: %q (cursor at col %d)", line[:end], col)
		}
	}

	cursorContext := ctx.BuildFromDocumentPosition(
		params.Position,
		docId,
		h.state,
	)

	suggestions := h.search.BuildCompletionList(
		cursorContext,
		h.state,
	)
	h.server.Log.Infof("  returning %d completion items", len(suggestions))
	return suggestions, nil
}
