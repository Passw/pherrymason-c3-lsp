package parser

import (
	"github.com/pherrymason/c3-lsp/pkg/document"
	"github.com/pherrymason/c3-lsp/pkg/symbols"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

/*
	module: $ => seq(
	'module',
	field('path', $.path_ident),
	optional(alias($.generic_module_parameters, $.generic_parameters)),
	optional($.attributes),
	';'

	attributes:
		@private

),
*/
func (p *Parser) nodeToModule(doc *document.Document, node *sitter.Node, sourceCode []byte) (*symbols.Module, string, map[string]*symbols.GenericParameter) {

	moduleName := node.ChildByFieldName("path").Utf8Text(sourceCode)

	generic_parameters := make(map[string]*symbols.GenericParameter)
	attributes := []string{}

	for i := 0; i < int(node.ChildCount()); i++ {
		n := node.Child(uint(i))
		//fmt.Println("Node type:", n.Kind(), ":: ", n.Utf8Text(sourceCode))
		switch n.Kind() {
		case "generic_param_list":
			for g := 0; g < int(n.ChildCount()); g++ {
				gn := n.Child(uint(g))
				//fmt.Println("G Node type:", gn.Kind(), ":: ", gn.Utf8Text(sourceCode))
				if gn.Kind() == "type_ident" {
					genericName := gn.Utf8Text(sourceCode)
					param := symbols.NewGenericParameter(
						genericName,
						moduleName,
						doc.URI,
						symbols.NewRangeFromTreeSitterPositions(gn.StartPosition(), gn.EndPosition()),
						symbols.NewRangeFromTreeSitterPositions(gn.StartPosition(), gn.EndPosition()),
					)
					generic_parameters[genericName] = param
				}
			}
		case "attributes":
			for a := 0; a < int(n.ChildCount()); a++ {
				gn := n.Child(uint(a))
				//fmt.Println("Attr Node type:", gn.Kind(), ":: ", gn.Utf8Text(sourceCode))
				attributes = append(attributes, gn.Utf8Text(sourceCode))
			}
		}
	}

	name := node.ChildByFieldName("path")
	module := symbols.NewModule(
		moduleName,
		doc.URI,
		symbols.NewRangeFromTreeSitterPositions(name.StartPosition(), name.EndPosition()),
		symbols.NewRangeFromTreeSitterPositions(name.StartPosition(), name.EndPosition()),
	)
	module.SetAttributes(attributes)
	module.SetGenericParameters(generic_parameters)

	return module, moduleName, generic_parameters
}

/*
		import_declaration: $ => seq(
	      'import',
	      field('path', commaSep1($.path_ident)),
	      optional($.attributes),
	      ';'
	    ),
*/
func (p *Parser) nodeToImport(doc *document.Document, node *sitter.Node, sourceCode []byte) []string {
	imports := []string{}

	for i := 0; i < int(node.ChildCount()); i++ {
		n := node.Child(uint(i))

		var pathIdent *sitter.Node
		switch n.Kind() {
		case "import_path":
			// 0.8: imports are wrapped in import_path nodes
			for j := 0; j < int(n.ChildCount()); j++ {
				if n.Child(uint(j)).Kind() == "path_ident" {
					pathIdent = n.Child(uint(j))
					break
				}
			}
		case "path_ident":
			pathIdent = n
		}

		if pathIdent != nil {
			temp_mod := ""
			for m := 0; m < int(pathIdent.ChildCount()); m++ {
				sn := pathIdent.Child(uint(m))
				if sn.Kind() == "ident" || sn.Kind() == "module_resolution" {
					temp_mod += sn.Utf8Text(sourceCode)
				}
			}
			imports = append(imports, temp_mod)
		}
	}

	return imports
}
