package parser

import (
	idx "github.com/pherrymason/c3-lsp/pkg/symbols"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

/*
enum_arg: $ => seq('=', $._expr),
enum_constant: $ => seq(

	field('name', $.const_ident),
	optional($.attributes),
	field('args', optional($.enum_arg)),

),
enum_param_declaration: $ => seq(

	field('type', $.type),
	field('name', $.ident),

),
enum_param_list: $ => seq('(', commaSep($.enum_param_declaration), ')'),
enum_spec: $ => prec.right(seq(

	  ':',
	  field('type', optional($.type)),
	  optional($.enum_param_list),
	)),
	enum_body: $ => seq(
	  '{',
	  commaSepTrailing($.enum_constant),
	  '}'
	),

enum_declaration: $ => seq(

	  'enum',
	  field('name', $.type_ident),
	  optional($.interface_impl),
	  optional($.enum_spec),
	  optional($.attributes),
	  field('body', $.enum_body),
	),
*/
func (p *Parser) nodeToEnum(node *sitter.Node, currentModule *idx.Module, docId *string, sourceCode []byte) idx.Enum {
	// TODO parse attributes

	baseType := ""
	var enumerators []*idx.Enumerator
	var associatedParameters []idx.Variable

	module := currentModule.GetModuleString()
	enumRange := idx.NewRangeFromTreeSitterPositions(declStart(node), node.EndPosition())

	nameNode := node.ChildByFieldName("name")
	name := ""
	idRange := idx.NewRange(0, 0, 0, 0)
	if nameNode != nil {
		name = nameNode.Utf8Text(sourceCode)
		idRange = idx.NewRangeFromTreeSitterPositions(nameNode.StartPosition(), nameNode.EndPosition())
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		n := node.Child(uint(i))
		switch n.Kind() {
		case "enum_spec":
			typeNode := n.ChildByFieldName("type")
			paramListIndex := 1
			if typeNode != nil {
				// Custom enum backing type is optional
				baseType = typeNode.Utf8Text(sourceCode)
				paramListIndex = 2
			}

			paramList := n.Child(uint(paramListIndex))
			// Check if has enum_param_list
			if paramList != nil {
				// Try to get enum_param_list
				for p := 0; p < int(paramList.ChildCount()); p++ {
					paramNode := paramList.Child(uint(p))
					if paramNode.Kind() == "enum_param" {
						paramTypeNode := paramNode.ChildByFieldName("type")
						paramNameNode := paramNode.ChildByFieldName("name")
						if paramTypeNode == nil || paramNameNode == nil {
							continue
						}

						//fmt.Println(paramNode.Kind(), paramNode.Utf8Text(sourceCode))
						associatedParameters = append(
							associatedParameters,
							idx.NewVariable(
								paramNameNode.Utf8Text(sourceCode),
								idx.NewTypeFromString(paramTypeNode.Utf8Text(sourceCode), module),
								module,
								*docId,
								idx.NewRangeFromTreeSitterPositions(paramNameNode.StartPosition(), paramNameNode.EndPosition()),
								idx.NewRangeFromTreeSitterPositions(paramNode.StartPosition(), paramNode.EndPosition()),
							),
						)
					}
				}
			}

		case "enum_body":
			for i := 0; i < int(n.ChildCount()); i++ {
				enumeratorNode := n.Child(uint(i))

				if enumeratorNode.Kind() == "enum_constant" {
					enumeratorName := enumeratorNode.ChildByFieldName("name")
					if enumeratorName == nil {
						// Invalid node
						continue
					}
					enumerator := idx.NewEnumerator(
						enumeratorName.Utf8Text(sourceCode),
						"",
						associatedParameters,
						name,
						module,
						idx.NewRangeFromTreeSitterPositions(enumeratorName.StartPosition(), enumeratorName.EndPosition()),
						*docId,
					)
					enumerators = append(enumerators, enumerator)
				}
			}
		}
	}

	enum := idx.NewEnum(
		name,
		baseType,
		[]*idx.Enumerator{},
		module,
		*docId,
		idRange,
		enumRange,
	)

	enum.AddEnumerators(enumerators)

	return enum
}
