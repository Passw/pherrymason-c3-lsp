package parser

import (
	idx "github.com/pherrymason/c3-lsp/pkg/symbols"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

/*
interface_declaration: $ => seq(

	  'interface',
	  field('name', $.type_ident),
	  field('body', $.interface_body),
	),
*/
func (p *Parser) nodeToInterface(node *sitter.Node, currentModule *idx.Module, docId *string, sourceCode []byte) idx.Interface {
	// TODO parse attributes
	methods := []*idx.Function{}

	for i := 0; i < int(node.ChildCount()); i++ {
		n := node.Child(uint(i))
		switch n.Kind() {
		case "interface_body":
			for i := 0; i < int(n.ChildCount()); i++ {
				m := n.Child(uint(i))
				var funcDecl *sitter.Node
				if m.Kind() == "interface_func_declaration" {
					// 0.8: interface methods are wrapped in interface_func_declaration
					for j := 0; j < int(m.ChildCount()); j++ {
						if m.Child(uint(j)).Kind() == "func_declaration" {
							funcDecl = m.Child(uint(j))
							break
						}
					}
				} else if m.Kind() == "func_declaration" {
					funcDecl = m
				}
				if funcDecl != nil {
					fun, err := p.nodeToFunction(funcDecl, currentModule, docId, sourceCode)
					if err == nil {
						methods = append(methods, &fun)
					}
				}
			}
		}
	}

	nameNode := node.ChildByFieldName("name")
	_interface := idx.NewInterface(
		nameNode.Utf8Text(sourceCode),
		currentModule.GetModuleString(),
		*docId,
		idx.NewRangeFromTreeSitterPositions(nameNode.StartPosition(), nameNode.EndPosition()),
		idx.NewRangeFromTreeSitterPositions(declStart(node), node.EndPosition()),
	)

	_interface.AddMethods(methods)

	return _interface
}
