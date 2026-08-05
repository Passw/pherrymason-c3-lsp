package parser

import (
	"strconv"

	"github.com/pherrymason/c3-lsp/pkg/option"
	idx "github.com/pherrymason/c3-lsp/pkg/symbols"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

func (p *Parser) nodeToBitStruct(node *sitter.Node, currentModule *idx.Module, docId *string, sourceCode []byte) idx.Bitstruct {
	nameNode := node.ChildByFieldName("name")
	name := nameNode.Utf8Text(sourceCode)
	var interfaces []string
	var bakedType idx.Type

	fieldsNode := node.ChildByFieldName("body")
	structFields := p.nodeToBitStructMembers(fieldsNode, currentModule, docId, sourceCode)

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(uint(i))
		//fmt.Println("type:", child.Kind(), child.Utf8Text(sourceCode))

		switch child.Kind() {
		case "interface_impl":
			// TODO
			for x := 0; x < int(child.ChildCount()); x++ {
				n := child.Child(uint(x))
				if n.Kind() == "interface" {
					interfaces = append(interfaces, n.Utf8Text(sourceCode))
				}
			}
		case "attributes":
			// TODO attributes
		case "type":
			bakedType = p.typeNodeToType(child, currentModule, sourceCode)
		}
	}

	_struct := idx.NewBitstruct(
		name,
		bakedType,
		interfaces,
		structFields,
		currentModule.GetModuleString(),
		*docId,
		idx.NewRangeFromTreeSitterPositions(nameNode.StartPosition(), nameNode.EndPosition()),
		idx.NewRangeFromTreeSitterPositions(declStart(node), node.EndPosition()),
	)

	return _struct
}

func (p *Parser) nodeToBitStructMembers(node *sitter.Node, currentModule *idx.Module, docId *string, sourceCode []byte) []*idx.StructMember {

	structFields := []*idx.StructMember{}
	// node = bitstruct_body
	for i := 0; i < int(node.ChildCount()); i++ {
		bdefnode := node.Child(uint(i))
		bType := bdefnode.Kind()
		if bType == "bitstruct_member_declaration" {
			var memberType idx.Type
			var identity string
			if bdefnodeType := bdefnode.ChildByFieldName("type"); bdefnodeType != nil {
				memberType = p.typeNodeToType(bdefnodeType, currentModule, sourceCode)
			}
			for x := 0; x < int(bdefnode.ChildCount()); x++ {
				xNode := bdefnode.Child(uint(x))
				//fmt.Println(xNode.Kind())
				switch xNode.Kind() {
				case "ident":
					identity = xNode.Utf8Text(sourceCode)
				}
			}

			bitRanges := [2]uint{}

			if bdefnode.ChildCount() >= 4 {
				lowBit, _ := strconv.ParseInt(bdefnode.Child(3).Utf8Text(sourceCode), 10, 32)
				bitRanges[0] = uint(lowBit)
			}

			if bdefnode.ChildCount() >= 6 {
				highBit, _ := strconv.ParseInt(bdefnode.Child(5).Utf8Text(sourceCode), 10, 32)
				bitRanges[1] = uint(highBit)
			}

			member := idx.NewStructMember(
				identity,
				memberType,
				option.Some(bitRanges),
				currentModule.GetModuleString(),
				*docId,
				idx.NewRangeFromTreeSitterPositions(bdefnode.Child(1).StartPosition(), bdefnode.Child(1).EndPosition()),
			)
			structFields = append(structFields, &member)
		}
	}

	return structFields
}
