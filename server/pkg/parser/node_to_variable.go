package parser

import (
	idx "github.com/pherrymason/c3-lsp/pkg/symbols"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

func (p *Parser) variableDeclarationNodeToVariable(declarationNode *sitter.Node, currentModule *idx.Module, docId *string, sourceCode []byte) []*idx.Variable {
	var variables []*idx.Variable
	//var typeNodeContent string
	var vType idx.Type

	//fmt.Println(declarationNode.ChildCount())
	//fmt.Println(declarationNode)
	//fmt.Println(declarationNode.Utf8Text(sourceCode))
	//fmt.Println("----")

	for i := uint(0); i < declarationNode.ChildCount(); i++ {
		n := declarationNode.Child(uint(i))
		//fmt.Println(i, ":", n.Kind(), ":: ", n.Utf8Text(sourceCode), ":: has errors: ", n.HasError())
		switch n.Kind() {
		case "type":
			//typeNodeContent = n.Utf8Text(sourceCode)
			vType = p.typeNodeToType(n, currentModule, sourceCode)
		case "ident":
			variable := idx.NewVariable(
				n.Utf8Text(sourceCode),
				vType,
				//idx.NewTypeFromString(typeNodeContent, moduleName), // <-- moduleName is potentially wrong
				currentModule.GetModuleString(),
				*docId,
				idx.NewRangeFromTreeSitterPositions(
					n.StartPosition(),
					n.EndPosition(),
				),
				idx.NewRangeFromTreeSitterPositions(
					declarationNode.StartPosition(),
					declarationNode.EndPosition()),
			)
			variables = append(variables, &variable)
		case "identifier_list":
			for j := 0; j < int(n.ChildCount()); j++ {

				bn := n.Child(uint(j))
				if bn.Kind() != "ident" {
					continue
				}
				variable := idx.NewVariable(
					bn.Utf8Text(sourceCode),
					vType,
					//idx.NewTypeFromString(typeNodeContent, moduleName), // <-- moduleName is potentially wrong
					currentModule.GetModuleString(),
					*docId,
					idx.NewRangeFromTreeSitterPositions(
						bn.StartPosition(),
						bn.EndPosition(),
					),
					idx.NewRangeFromTreeSitterPositions(
						declarationNode.StartPosition(),
						declarationNode.EndPosition()),
				)
				variables = append(variables, &variable)
			}
		case ";":
			if n.HasError() && len(variables) > 0 {
				// Last variable is incomplete, remove it
				variables = variables[:len(variables)-1]
			}

		}

	}

	return variables
}

/*
		const_declaration: $ => seq(
	      'const',
	      field('type', optional($.type)),
	      $.const_ident,
	      optional($.attributes),
	      optional($._assign_right_expr),
	      ';'
	    )
*/
func (p *Parser) nodeToConstant(node *sitter.Node, currentModule *idx.Module, docId *string, sourceCode []byte) idx.Variable {
	var constant idx.Variable
	var typeNodeContent string
	var idNode *sitter.Node

	//fmt.Println(node.ChildCount())
	//fmt.Println(node)
	//fmt.Println(node.Utf8Text(sourceCode))

	for i := uint(0); i < node.ChildCount(); i++ {
		n := node.Child(uint(i))
		switch n.Kind() {
		case "type":
			typeNodeContent = n.Utf8Text(sourceCode)

		case "const_ident":
			idNode = n
		}
	}

	constant = idx.NewConstant(
		idNode.Utf8Text(sourceCode),
		idx.NewTypeFromString(typeNodeContent, currentModule.GetModuleString()), // <-- moduleName is potentially wrong
		currentModule.GetModuleString(),
		*docId,
		idx.NewRangeFromTreeSitterPositions(
			idNode.StartPosition(),
			idNode.EndPosition(),
		),
		idx.NewRangeFromTreeSitterPositions(
			node.StartPosition(),
			node.EndPosition()),
	)

	return constant
}
