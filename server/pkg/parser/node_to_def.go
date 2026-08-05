package parser

import (
	idx "github.com/pherrymason/c3-lsp/pkg/symbols"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

/*
alias_declaration: $ => seq(

	'alias',
	choice(
	  // Variable/function/macro/method/module
	  seq(
	    field('name', $._func_macro_ident),
	    optional($.attributes),
	    choice(
	      seq('=', 'module', $.path_ident),
	      $._assign_right_expr,
	    )
	  ),
	  // Constant
	  seq(
	    field('name', $.const_ident),
	    optional($.attributes),
	    $._assign_right_expr,
	  ),
	  // Type/function
	  seq(
	    field('name', $.type_ident),
	    optional($.attributes),
	    '=',
	    choice($._type_expr, $.func_signature)
	  ),
	),
	';'

),
*/
func (p *Parser) nodeToDef(node *sitter.Node, currentModule *idx.Module, docId *string, sourceCode []byte) idx.Def {
	//fmt.Println(node)
	// TODO: attributes
	ds := declStart(node)
	defBuilder := idx.NewDefBuilder("", currentModule.GetModuleString(), *docId).
		WithDocumentRange(
			uint(ds.Row),
			uint(ds.Column),
			uint(node.EndPosition().Row),
			uint(node.EndPosition().Column),
		)
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		defBuilder.WithName(nameNode.Utf8Text(sourceCode)).
			WithIdentifierRange(
				uint(nameNode.StartPosition().Row),
				uint(nameNode.StartPosition().Column),
				uint(nameNode.EndPosition().Row),
				uint(nameNode.EndPosition().Column),
			)
	}
	var bodyNode *sitter.Node
	for i := 0; i < int(node.ChildCount()-1); i++ {
		if node.Child(uint(i)).Kind() == "=" {
			bodyNode = node.Child(uint(i + 1))
			break
		}
	}
	if bodyNode != nil && bodyNode.Kind() == "type" {
		// Might contain module path
		type_ := p.typeNodeToType(bodyNode, currentModule, sourceCode)
		defBuilder.WithResolvesToType(type_)
	} else if bodyNode != nil {
		defBuilder.WithResolvesTo(bodyNode.Utf8Text(sourceCode))
	}

	return *defBuilder.Build()
}
