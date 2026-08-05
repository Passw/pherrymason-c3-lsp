package parser

import (
	idx "github.com/pherrymason/c3-lsp/pkg/symbols"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

/*
distinct_declaration: $ => seq(

	  'distinct',
	  field('name', $.type_ident),
	  optional($.interface_impl),  // TODO
	  optional($.attributes),      // TODO
	  '=',
	  optional('inline'),
	  $.type,
	  ';'
	),
*/
func (p *Parser) nodeToDistinct(node *sitter.Node, currentModule *idx.Module, docId *string, sourceCode []byte) idx.Distinct {

	ds := declStart(node)
	distinctBuilder := idx.NewDistinctBuilder("", currentModule.GetModuleString(), *docId).
		WithDocumentRange(
			uint(ds.Row),
			uint(ds.Column),
			uint(node.EndPosition().Row),
			uint(node.EndPosition().Column),
		)

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		distinctBuilder.
			WithName(nameNode.Utf8Text(sourceCode)).
			WithIdentifierRange(
				uint(nameNode.StartPosition().Row),
				uint(nameNode.StartPosition().Column),
				uint(nameNode.EndPosition().Row),
				uint(nameNode.EndPosition().Column),
			)
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		n := node.Child(uint(i))
		switch n.Kind() {
		case "inline":
			distinctBuilder.WithInline(true)
		case "type":
			// Might contain module path
			_type := p.typeNodeToType(n, currentModule, sourceCode)
			distinctBuilder.WithBaseType(_type)
		}
	}

	return *distinctBuilder.Build()
}
