package context

import (
	"github.com/pherrymason/c3-lsp/internal/lsp/project_state"
	"github.com/pherrymason/c3-lsp/pkg/symbols"
	sitter "github.com/tree-sitter/go-tree-sitter"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

type CursorContext struct {
	Position symbols.Position
	DocURI   protocol.DocumentUri

	IsLiteral          bool
	IsIdentifier       bool
	IsModuleIdentifier bool
}

func BuildFromDocumentPosition(
	position protocol.Position,
	docURI protocol.DocumentUri,
	state *project_state.ProjectState,
) CursorContext {
	context := CursorContext{
		Position: symbols.NewPositionFromLSPPosition(position),
		DocURI:   docURI,
	}

	doc := state.GetDocument(docURI)
	tree := doc.ContextSyntaxTree
	root := tree.RootNode()

	// Search sitter.Node where cursor is currently
	node := root.NamedDescendantForPointRange(
		sitter.Point{Row: uint(position.Line), Column: uint(position.Character)},
		sitter.Point{Row: uint(position.Line), Column: uint(position.Character + 1)},
	)

	if node == nil {
		// Could not find node in document.
		return context
	}

	//s := fmt.Sprintf("Node found. Type: %s. Content: %s", node.Kind(), node.Utf8Text([]byte(doc.SourceCode.Text)))
	//fmt.Printf(s)

	switch node.Kind() {
	case "integer_literal":
		context.IsLiteral = true
	case "real_literal":
		context.IsLiteral = true
	case "char_literal":
		context.IsLiteral = true
	case "string_literal":
		context.IsLiteral = true
	case "raw_string_literal":
		context.IsLiteral = true
	case "string_expr":
		context.IsLiteral = true
	case "bytes_expr":
		context.IsLiteral = true

	case "ident":
		context.IsIdentifier = true

		if node.Parent().Kind() == "module_resolution" {
			context.IsModuleIdentifier = true
		}
	}

	return context
}
