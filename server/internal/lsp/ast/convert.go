package ast

import "C"

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pherrymason/c3-lsp/internal/lsp/cst"
	"github.com/pherrymason/c3-lsp/pkg/option"
	"github.com/pherrymason/c3-lsp/pkg/symbols"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

func GetCST(sourceCode string) *sitter.Node {
	return cst.GetParsedTreeFromString(sourceCode).RootNode()
}

func ConvertToAST(cstNode *sitter.Node, sourceCode string, fileName string) File {
	source := []byte(sourceCode)

	var prg File

	if cstNode.Kind() == "source_file" {
		prg = File{
			Name:        fileName,
			ASTNodeBase: NewBaseNodeBuilder().WithSitterPos(cstNode).Build(),
		}
	}

	anonymousModule := false
	for i := 0; i < int(cstNode.ChildCount()); i++ {
		node := cstNode.Child(uint(i))
		parsedModules := len(prg.Modules)
		if parsedModules == 0 && node.Kind() != "module_declaration" {
			anonymousModule = true
			prg.Modules = append(prg.Modules,
				Module{
					ASTNodeBase: NewBaseNodeBuilder().WithStartEnd(uint(node.StartPosition().Row), uint(node.StartPosition().Column), 0, 0).Build(),
					Name:        symbols.NormalizeModuleName(fileName),
				},
			)
			parsedModules = len(prg.Modules)
		}

		var lastMod *Module
		if parsedModules > 0 {
			lastMod = &prg.Modules[len(prg.Modules)-1]
		}

		switch node.Kind() {
		case "module_declaration":
			if anonymousModule {
				anonymousModule = false
				lastMod.ASTNodeBase.EndPos = Position{uint(node.StartPosition().Row), uint(node.StartPosition().Column)}
			}

			prg.Modules = append(prg.Modules, convert_module(node, source))

		case "import_declaration":
			lastMod.Imports = append(lastMod.Imports, convert_imports(node, source)...)

		case "global_declaration":
			lastMod.Declarations = append(lastMod.Declarations, convert_global_declaration(node, source))

		case "enum_declaration":
			lastMod.Declarations = append(lastMod.Declarations, convert_enum_declaration(node, source))

		case "struct_declaration":
			lastMod.Declarations = append(lastMod.Declarations, convert_struct_declaration(node, source))

		case "bitstruct_declaration":
			lastMod.Declarations = append(lastMod.Declarations, convert_bitstruct_declaration(node, source))

		case "faultdef_declaration":
			lastMod.Declarations = append(lastMod.Declarations, convert_fault_declaration(node, source))

		case "const_declaration":
			lastMod.Declarations = append(lastMod.Declarations, convert_const_declaration(node, source))

		case "alias_declaration":
			lastMod.Declarations = append(lastMod.Declarations, convert_def_declaration(node, source))

		case "func_definition", "func_declaration":
			lastMod.Functions = append(lastMod.Functions, convert_function_declaration(node, source))

		case "interface_declaration":
			lastMod.Declarations = append(lastMod.Declarations, convert_interface_declaration(node, source))

		case "macro_declaration":
			lastMod.Macros = append(lastMod.Macros, convert_macro_declaration(node, source))
		}
	}

	return prg
}

func convertSourceFile(node *sitter.Node, source []byte) File {
	file := File{}
	file.SetPos(node.StartPosition(), node.EndPosition())

	return file
}

func convert_module(node *sitter.Node, source []byte) Module {
	module := Module{}
	module.Name = node.ChildByFieldName("path").Utf8Text(source)
	module.SetPos(node.StartPosition(), node.EndPosition())

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(uint(i))
		switch child.Kind() {
		case "generic_param_list":
			for g := 0; g < int(child.ChildCount()); g++ {
				gn := child.Child(uint(g))
				if gn.Kind() == "type_ident" {
					genericName := gn.Utf8Text(source)
					module.GenericParameters = append(module.GenericParameters, genericName)
				}
			}
		case "attributes":
			for a := 0; a < int(child.ChildCount()); a++ {
				gn := child.Child(uint(a))
				module.Attributes = append(module.Attributes, gn.Utf8Text(source))
			}
		}
	}

	return module
}

func convert_imports(node *sitter.Node, source []byte) []string {
	imports := []string{}

	for i := 0; i < int(node.ChildCount()); i++ {
		n := node.Child(uint(i))

		switch n.Kind() {
		case "path_ident":
			temp_mod := ""
			for m := 0; m < int(n.ChildCount()); m++ {
				sn := n.Child(uint(m))
				if sn.Kind() == "ident" || sn.Kind() == "module_resolution" {
					temp_mod += sn.Utf8Text(source)
				}
			}
			imports = append(imports, temp_mod)
		}
	}

	return imports
}

func convert_global_declaration(node *sitter.Node, source []byte) VariableDecl {
	variable := VariableDecl{
		Names: []Identifier{},
		ASTNodeBase: NewBaseNodeBuilder().
			WithSitterPosRange(node.StartPosition(), node.EndPosition()).
			Build(),
	}
	if node.ChildCount() == 0 && node.Child(0).Kind() == "declaration" {
		return variable
	}
	node = node.Child(0)

	for i := uint(0); i < node.ChildCount(); i++ {
		n := node.Child(uint(i))
		fmt.Println(i, ":", n.Kind(), ":: ", n.Utf8Text(source), ":: has errors: ", n.HasError())
		switch n.Kind() {
		case "type":
			variable.Type = typeNodeToType(n, source)

		case "ident":
			variable.Names = append(
				variable.Names,
				Identifier{
					Name: n.Utf8Text(source),
					ASTNodeBase: NewBaseNodeBuilder().
						WithSitterPosRange(n.StartPosition(), n.EndPosition()).
						Build(),
				},
			)

		case ";":

		case "identifier_list":
			for j := 0; j < int(n.ChildCount()); j++ {
				sub := n.Child(uint(j))
				if sub.Kind() == "ident" {
					variable.Names = append(
						variable.Names,
						Identifier{
							Name: sub.Utf8Text(source),
							ASTNodeBase: NewBaseNodeBuilder().
								WithSitterPosRange(sub.StartPosition(), sub.EndPosition()).
								Build(),
						},
					)
				}
			}
		}
	}

	// Check for initializer
	right := node.ChildByFieldName("right")
	if right != nil {
		if is_literal(right) {
			variable.Initializer = convert_literal(right, source)
		} else if right.Kind() == "ident" {
			variable.Initializer = NewIdentifierBuilder().WithName(right.Utf8Text(source)).WithSitterPos(right).Build()
		}
	}

	return variable
}

func convert_enum_declaration(node *sitter.Node, sourceCode []byte) EnumDecl {
	enumDecl := EnumDecl{
		Name: node.ChildByFieldName("name").Utf8Text(sourceCode),
		ASTNodeBase: NewBaseNodeBuilder().
			WithSitterPosRange(node.StartPosition(), node.EndPosition()).
			Build(),
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		n := node.Child(uint(i))
		switch n.Kind() {
		case "enum_spec":
			enumDecl.BaseType = typeNodeToType(n.Child(1), sourceCode)
			if n.ChildCount() >= 3 {
				param_list := n.Child(2)
				for p := 0; p < int(param_list.ChildCount()); p++ {
					paramNode := param_list.Child(uint(p))
					if paramNode.Kind() == "enum_param" {
						enumDecl.Properties = append(
							enumDecl.Properties,
							EnumProperty{
								ASTNodeBase: NewBaseNodeBuilder().
									WithSitterPosRange(paramNode.StartPosition(), paramNode.EndPosition()).
									Build(),
								Name: Identifier{
									Name: paramNode.Child(1).Utf8Text(sourceCode),
									ASTNodeBase: NewBaseNodeBuilder().
										WithSitterPosRange(paramNode.Child(1).StartPosition(), paramNode.Child(1).EndPosition()).
										Build(),
								},
								Type: typeNodeToType(paramNode.Child(0), sourceCode),
							},
						)
					}
				}
			}

		case "enum_body":
			for i := 0; i < int(n.ChildCount()); i++ {
				enumeratorNode := n.Child(uint(i))
				if enumeratorNode.Kind() != "enum_constant" {
					continue
				}

				compositeLiteral := CompositeLiteral{}
				args := enumeratorNode.ChildByFieldName("args")
				if args != nil && args.ChildCount() > 0 {
					args := args.Child(uint(int(args.ChildCount())) - 1)
					if is_literal(args) {
						compositeLiteral.Values = append(compositeLiteral.Values,
							convert_literal(args, sourceCode),
						)
					} else if args.Kind() == "initializer_list" {
						for a := 0; a < int(args.ChildCount()); a++ {
							arg := args.Child(uint(a))
							if arg.Kind() == "initializer_element" {
								if !is_literal(arg.Child(0)) {
									// Exit early to ensure correspondence between
									// index of each value and index of each predefined
									// enum parameter
									break
								}
								compositeLiteral.Values = append(compositeLiteral.Values,
									convert_literal(arg.Child(0), sourceCode),
								)
							}
						}
					}
				}

				name := enumeratorNode.ChildByFieldName("name")
				enumDecl.Members = append(enumDecl.Members,
					EnumMember{
						Name: Identifier{
							Name: name.Utf8Text(sourceCode),
							ASTNodeBase: NewBaseNodeBuilder().
								WithSitterPosRange(name.StartPosition(), name.EndPosition()).
								Build(),
						},
						Value: compositeLiteral,
						ASTNodeBase: NewBaseNodeBuilder().
							WithSitterPosRange(enumeratorNode.StartPosition(), enumeratorNode.EndPosition()).
							Build(),
					},
				)

			}
		}
	}

	return enumDecl
}

func convert_struct_declaration(node *sitter.Node, sourceCode []byte) StructDecl {
	structDecl := StructDecl{
		ASTNodeBase: NewBaseNodeBuilder().
			WithSitterPosRange(node.StartPosition(), node.EndPosition()).
			Build(),
		StructType: StructTypeNormal,
	}

	structDecl.Name = node.ChildByFieldName("name").Utf8Text(sourceCode)
	//membersNeedingSubtypingResolve := []string{}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(uint(i))
		switch child.Kind() {
		case "union":
			structDecl.StructType = StructTypeUnion
		case "interface_impl_list":
			for x := 0; x < int(child.ChildCount()); x++ {
				n := child.Child(uint(x))
				if n.IsNamed() {
					structDecl.Implements = append(structDecl.Implements, n.Utf8Text(sourceCode))
				}
			}
		case "attributes":
			// TODO attributes
		}
	}

	// TODO parse attributes
	bodyNode := node.ChildByFieldName("body")

	// Search Struct members
	for i := 0; i < int(bodyNode.ChildCount()); i++ {
		memberNode := bodyNode.Child(uint(i))
		isInline := false

		//fmt.Println("body child:", memberNode.Kind())
		if memberNode.Kind() != "struct_member_declaration" {
			continue
		}
		fmt.Printf("%d - %s\n", i, memberNode.Utf8Text(sourceCode))

		fieldType := TypeInfo{}
		member := StructMemberDecl{
			ASTNodeBase: NewBaseNodeBuilder().
				WithSitterPosRange(memberNode.StartPosition(), memberNode.EndPosition()).
				Build(),
		}

		for x := 0; x < int(memberNode.ChildCount()); x++ {
			n := memberNode.Child(uint(x))

			switch n.Kind() {
			case "type":
				fieldType = typeNodeToType(n, sourceCode)
				member.Type = fieldType
				//fmt.Println(fieldType, n.Utf8Text(sourceCode))

				//fieldType = n.Utf8Text(sourceCode)
				if isInline {
					//	identifier = "dummy-subtyping"
				}
			case "identifier_list":
				for j := 0; j < int(n.ChildCount()); j++ {
					member.Names = append(member.Names,
						Identifier{
							ASTNodeBase: NewBaseNodeBuilder().WithSitterPosRange(n.Child(uint(j)).StartPosition(), n.Child(uint(j)).EndPosition()).Build(),
							Name:        n.Child(uint(j)).Utf8Text(sourceCode),
						},
					) /*
						identifiers = append(identifiers, n.Child(uint(j)).Utf8Text(sourceCode))
						identifiersRange = append(identifiersRange,
							idx.NewRangeFromTreeSitterPositions(n.StartPosition(), n.EndPosition()),
						)*/
				}
			case "attributes":
				// TODO
			case "bitstruct_body":
				bitStructsMembers := convert_bitstruct_members(n, sourceCode)
				structDecl.Members = append(structDecl.Members, bitStructsMembers...)
				//structFields = append(structFields, bitStructsMembers...)

			case "inline":
				//isInline = true
				//fmt.Println("inline!: ", n.Utf8Text(sourceCode))
				//inlinedSubTyping = append(inlinedSubTyping, "1")
				member.IsInlined = true

			case "ident":
				member.Names = append(member.Names,
					Identifier{
						ASTNodeBase: NewBaseNodeBuilder().WithSitterPosRange(n.StartPosition(), n.EndPosition()).Build(),
						Name:        n.Utf8Text(sourceCode),
					},
				) /*
					identifier = n.Utf8Text(sourceCode)
					identifiersRange = append(identifiersRange,
						idx.NewRangeFromTreeSitterPositions(n.StartPosition(), n.EndPosition()),
					)*/
			}
		}

		/*
			if len(identifiers) > 0 {
				for y := 0; y < len(identifiers); y++ {
					structMember := idx.NewStructMember(
						identifiers[y],
						fieldType, // TODO <--- this type parsing is too simple
						option.None[[2]uint](),
						currentModule.GetModuleString(),
						docId,
						identifiersRange[y],
					)
					structFields = append(structFields, &structMember)
				}
			} else if isInline {
				var structMember idx.StructMember
				membersNeedingSubtypingResolve = append(membersNeedingSubtypingResolve, fieldType)
				structMember = idx.NewInlineSubtype(
					identifier,
					fieldType,
					currentModule.GetModuleString(),
					docId,
					identifiersRange[0],
				)
				structFields = append(structFields, &structMember)
			} else if len(identifier) > 0 {
				structMember := idx.NewStructMember(
					identifier,
					fieldType,
					option.None[[2]uint](),
					currentModule.GetModuleString(),
					docId,
					identifiersRange[0],
				)

				structFields = append(structFields, &structMember)
			}*/

		if len(member.Names) > 0 {
			structDecl.Members = append(structDecl.Members, member)
		}
	}

	return structDecl
}

func convert_bitstruct_declaration(node *sitter.Node, sourceCode []byte) StructDecl {
	structDecl := StructDecl{
		ASTNodeBase: NewBaseNodeBuilder().WithSitterPosRange(node.StartPosition(), node.EndPosition()).Build(),
		StructType:  StructTypeBitStruct,
	}

	membersNode := node.ChildByFieldName("body")
	structDecl.Members = convert_bitstruct_members(membersNode, sourceCode)

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(uint(i))
		//fmt.Println("type:", child.Kind(), child.Utf8Text(sourceCode))

		switch child.Kind() {
		case "interface_impl":
			// TODO
			for x := 0; x < int(child.ChildCount()); x++ {
				n := child.Child(uint(x))
				if n.Kind() == "interface" {
					structDecl.Implements = append(structDecl.Implements, n.Utf8Text(sourceCode))
				}
			}

		case "attributes":
			// TODO attributes

		case "type":
			structDecl.BackingType = option.Some(typeNodeToType(child, sourceCode))
		}
	}

	return structDecl
}

func convert_bitstruct_members(node *sitter.Node, sourceCode []byte) []StructMemberDecl {
	members := []StructMemberDecl{}
	for i := 0; i < int(node.ChildCount()); i++ {
		bdefnode := node.Child(uint(i))
		bType := bdefnode.Kind()
		member := StructMemberDecl{
			ASTNodeBase: NewBaseNodeBuilder().
				WithSitterPosRange(bdefnode.StartPosition(), bdefnode.EndPosition()).
				Build(),
		}

		if bType == "bitstruct_member_declaration" {
			for x := 0; x < int(bdefnode.ChildCount()); x++ {
				xNode := bdefnode.Child(uint(x))
				//fmt.Println(xNode.Kind())
				switch xNode.Kind() {
				case "base_type":
					// Note: here we consciously pass bdefnode because typeNodeToType expects a child node of base_type. If we send xNode it will not find it.
					member.Type = typeNodeToType(bdefnode, sourceCode)
				case "ident":
					member.Names = append(
						member.Names,
						NewIdentifierBuilder().
							WithName(xNode.Utf8Text(sourceCode)).
							WithSitterPos(xNode).
							Build(),
					)
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
			member.BitRange = option.Some(bitRanges)

			/*member := idx.NewStructMember(
				identity,
				memberType,
				option.Some(bitRanges),
				currentModule.GetModuleString(),
				docId,
				idx.NewRangeFromTreeSitterPositions(bdefnode.Child(1).StartPosition(), bdefnode.Child(1).EndPosition()),
			)*/
			members = append(members, member)
		}
	}

	return members
}

func convert_fault_declaration(node *sitter.Node, sourceCode []byte) Expression {
	// TODO parse attributes

	baseType := option.None[TypeInfo]() // TODO Parse type!
	var constants []FaultMember

	for i := 0; i < int(node.ChildCount()); i++ {
		n := node.Child(uint(i))
		switch n.Kind() {
		case "fault_body":
			for i := 0; i < int(n.ChildCount()); i++ {
				constantNode := n.Child(uint(i))

				if constantNode.Kind() == "const_ident" {
					constants = append(constants,
						FaultMember{
							Name: NewIdentifierBuilder().
								WithName(constantNode.Utf8Text(sourceCode)).
								WithSitterPos(constantNode).
								Build(),
							ASTNodeBase: NewBaseNodeBuilder().
								WithSitterPosRange(constantNode.StartPosition(), constantNode.EndPosition()).
								Build(),
						},
					)
				}
			}
		}
	}

	nameNode := node.ChildByFieldName("name")
	fault := FaultDecl{
		Name: NewIdentifierBuilder().
			WithName(nameNode.Utf8Text(sourceCode)).
			WithSitterPos(nameNode).
			Build(),
		BackingType: baseType,
		Members:     constants,
		ASTNodeBase: NewBaseNodeBuilder().
			WithSitterPosRange(node.StartPosition(), node.EndPosition()).
			Build(),
	}

	return fault
}

func convert_const_declaration(node *sitter.Node, sourceCode []byte) Expression {
	constant := ConstDecl{
		Names: []Identifier{},
		ASTNodeBase: NewBaseNodeBuilder().
			WithSitterPosRange(node.StartPosition(), node.EndPosition()).
			Build(),
	}

	var idNode *sitter.Node

	//fmt.Println(node.ChildCount())
	//fmt.Println(node)
	//fmt.Println(node.Utf8Text(sourceCode))

	for i := uint(0); i < node.ChildCount(); i++ {
		n := node.Child(uint(i))
		switch n.Kind() {
		case "type":
			constant.Type = typeNodeToType(n, sourceCode)

		case "const_ident":
			idNode = n
		}
	}

	constant.Names = append(constant.Names,
		NewIdentifierBuilder().
			WithName(idNode.Utf8Text(sourceCode)).
			WithSitterPos(idNode).
			Build(),
	)

	return constant
}

/*
define_declaration [13, 0] - [13, 15]

	type_ident [13, 4] - [13, 8]
	typedef_type [13, 11] - [13, 14]
	type [13, 11] - [13, 14]
		base_type [13, 11] - [13, 14]
		base_type_name [13, 11] - [13, 14]
*/
func convert_def_declaration(node *sitter.Node, sourceCode []byte) Expression {
	defBuilder := NewDefDeclBuilder().
		WithSitterPos(node)

	for i := 0; i < int(node.ChildCount()); i++ {
		n := node.Child(uint(i))
		switch n.Kind() {
		case "type_ident", "define_ident":
			defBuilder.WithName(n.Utf8Text(sourceCode)).
				WithIdentifierSitterPos(n)

		case "typedef_type":
			var _type TypeInfo
			if n.Child(0).Kind() == "type" {
				// Might contain module path
				_type = typeNodeToType(n.Child(0), sourceCode)
				defBuilder.WithResolvesToType(_type)
			} else if n.Child(0).Kind() == "func_typedef" {
				// TODO Parse full info of this func typedefinition
				defBuilder.WithResolvesTo(n.Utf8Text(sourceCode))
			}
		}
	}

	return defBuilder.Build()
}

func convert_function_declaration(node *sitter.Node, sourceCode []byte) Expression {
	var typeIdentifier option.Option[Identifier]
	funcHeader := node.Child(1)

	if funcHeader.ChildByFieldName("method_type") != nil {
		typeIdentifier = option.Some(NewIdentifierBuilder().
			WithName(funcHeader.ChildByFieldName("method_type").Utf8Text(sourceCode)).
			WithSitterPos(funcHeader.ChildByFieldName("method_type")).
			Build())
	}
	signature := convert_function_signature(node, sourceCode)

	funcDecl := FunctionDecl{
		ASTNodeBase:  NewBaseNodeBuilder().WithSitterPos(node).Build(),
		ParentTypeId: typeIdentifier,
		Signature:    signature,
	}

	/*
		var variables []*idx.Variable
		if node.ChildByFieldName("body") != nil {
			variables = p.FindVariableDeclarations(node, currentModule.GetModuleString(), currentModule, docId, sourceCode)
		}

		variables = append(variables, parameters...)

		funcDecl.AddVariables(variables)
	*/
	return funcDecl
}

func convert_function_signature(node *sitter.Node, sourceCode []byte) FunctionSignature {
	var typeIdentifier option.Option[Identifier]
	funcHeader := node.Child(1)
	nameNode := funcHeader.ChildByFieldName("name")

	if funcHeader.ChildByFieldName("method_type") != nil {
		typeIdentifier = option.Some(NewIdentifierBuilder().
			WithName(funcHeader.ChildByFieldName("method_type").Utf8Text(sourceCode)).
			WithSitterPos(funcHeader.ChildByFieldName("method_type")).
			Build())
	}

	parameters := []FunctionParameter{}
	nodeParameters := node.Child(2)
	if nodeParameters.ChildCount() > 2 {
		for i := uint(0); i < nodeParameters.ChildCount(); i++ {
			argNode := nodeParameters.Child(uint(i))
			if argNode.Kind() != "parameter" {
				continue
			}

			parameters = append(
				parameters,
				convert_function_parameter(argNode, typeIdentifier, sourceCode),
			)
		}
	}

	signatureDecl := FunctionSignature{
		Name: NewIdentifierBuilder().
			WithName(nameNode.Utf8Text(sourceCode)).
			WithSitterPos(nameNode).
			Build(),
		ReturnType: typeNodeToType(funcHeader.ChildByFieldName("return_type"), sourceCode),
		Parameters: parameters,
		ASTNodeBase: NewBaseNodeBuilder().
			WithSitterPosRange(node.StartPosition(), node.EndPosition()).
			Build(),
	}

	return signatureDecl
}

// nodeToArgument Very similar to nodeToVariable, but arguments have optional identifiers (for example when using `self` for struct methods)
/*
	_parameter: $ => choice(
      seq($.type, $.ident, optional($.attributes)),			// 3
      seq($.type, '...', $.ident, optional($.attributes)),	// 3/4
      seq($.type, '...', $.ct_ident),						// 3
      seq($.type, $.ct_ident),								// 2
      seq($.type, '...', optional($.attributes)),			// 2/3
      seq($.type, $.hash_ident, optional($.attributes)),	// 2/3
      seq($.type, '&', $.ident, optional($.attributes)),	// 3/4
      seq($.type, optional($.attributes)),					// 1/2
      seq('&', $.ident, optional($.attributes)),			// 2/3
      seq($.hash_ident, optional($.attributes)),			// 1/2
      '...',												// 1
      seq($.ident, optional($.attributes)),					// 1/2
      seq($.ident, '...', optional($.attributes)),			// 2/3
      $.ct_ident,											// 1
      seq($.ct_ident, '...'),								// 2
    ),
*/
func convert_function_parameter(argNode *sitter.Node, methodIdentifier option.Option[Identifier], sourceCode []byte) FunctionParameter {
	var identifier Identifier
	var argType TypeInfo
	ampersandFound := false

	for i := 0; i < int(argNode.ChildCount()); i++ {
		n := argNode.Child(uint(i))

		switch n.Kind() {
		case "&":
			ampersandFound = true

		case "type":
			argType = typeNodeToType(n, sourceCode)
		case "ident":
			identifier = NewIdentifierBuilder().
				WithName(n.Utf8Text(sourceCode)).
				WithSitterPos(n).
				Build()

			// When detecting a self, the type is the Struct type
			if identifier.Name == "self" && methodIdentifier.IsSome() {
				pointer := uint(0)
				if ampersandFound {
					pointer = 1
				}

				argType = TypeInfo{
					Identifier: NewIdentifierBuilder().
						WithName(methodIdentifier.Get().Name).
						WithSitterPos(n).
						Build(),
					Pointer:     pointer,
					ASTNodeBase: NewBaseNodeBuilder().WithSitterPos(argNode).Build(),
				}
			}
		}
	}

	if ampersandFound {
		fmt.Println("poiner")
	}

	variable := FunctionParameter{
		Name:        identifier,
		Type:        argType,
		ASTNodeBase: NewBaseNodeBuilder().WithSitterPos(argNode).Build(),
	}

	return variable
}

func convert_interface_declaration(node *sitter.Node, sourceCode []byte) Expression {
	// TODO parse attributes
	methods := []FunctionSignature{}
	for i := 0; i < int(node.ChildCount()); i++ {
		n := node.Child(uint(i))
		switch n.Kind() {
		case "interface_body":
			for i := 0; i < int(n.ChildCount()); i++ {
				m := n.Child(uint(i))
				if m.Kind() == "func_declaration" {
					fun := convert_function_signature(m, sourceCode)
					methods = append(methods, fun)
				}
			}
		}
	}

	nameNode := node.ChildByFieldName("name")
	_interface := InterfaceDecl{
		ASTNodeBase: NewBaseNodeBuilder().WithSitterPos(node).Build(),
		Name:        NewIdentifierBuilder().WithName(nameNode.Utf8Text(sourceCode)).WithSitterPos(nameNode).Build(),
		Methods:     methods,
	}

	return _interface
}

func convert_macro_declaration(node *sitter.Node, sourceCode []byte) Expression {
	var nameNode *sitter.Node

	parameters := []FunctionParameter{}
	nodeParameters := node.Child(2)
	if nodeParameters.ChildCount() > 2 {
		for i := uint(0); i < nodeParameters.ChildCount(); i++ {
			argNode := nodeParameters.Child(uint(i))
			if argNode.Kind() != "parameter" {
				continue
			}

			parameters = append(
				parameters,
				convert_function_parameter(argNode, option.None[Identifier](), sourceCode),
			)
		}
	}

	nameNode = node.Child(1).ChildByFieldName("name")
	macro := MacroDecl{
		ASTNodeBase: NewBaseNodeBuilder().WithSitterPos(node).Build(),
		Signature: MacroSignature{
			Name: NewIdentifierBuilder().
				WithName(nameNode.Utf8Text(sourceCode)).
				WithSitterPos(nameNode).
				Build(),
			Parameters: parameters,
		},
	}
	/*
		if node.ChildByFieldName("body") != nil {
			variables := p.FindVariableDeclarations(node, currentModule.GetModuleString(), currentModule, docId, sourceCode)
			variables = append(arguments, variables...)
			macro.AddVariables(variables)
		}
	*/
	return macro
}

func is_literal(node *sitter.Node) bool {
	literals := []string{
		"string_literal", "char_literal",
		"integer_literal", "real_literal",
		"true",
		"false",
	}

	value := node.Kind()
	for _, v := range literals {
		if v == value {
			return true
		}
	}
	return false
}

func convert_literal(node *sitter.Node, sourceCode []byte) Expression {
	var literal Expression
	//fmt.Printf("Converting literal %s\n", node.Kind())
	switch node.Kind() {
	case "string_literal", "char_literal":
		fmt.Printf("%s: %s\n", node.Kind(), node.Utf8Text(sourceCode))
		literal = Literal{Value: node.Utf8Text(sourceCode)}
	case "integer_literal", "real_literal":
		/*
			for i := 0; i < int(node.ChildCount()); i++ {
				fmt.Printf("Literal type not supported: %s\n", node.Child(uint(i)).Kind())
			}
			fmt.Printf("Literal value: %s\n", node.Utf8Text(sourceCode))*/
		literal = Literal{
			Value: node.Utf8Text(sourceCode),
		}

	case "false":
		literal = BoolLiteral{Value: false}

	case "true":
		literal = BoolLiteral{Value: true}
	default:
		panic(fmt.Sprintf("Literal type not supported: %s\n", node.Kind()))
	}

	return literal
}

func typeNodeToType(node *sitter.Node, sourceCode []byte) TypeInfo {
	/*
		baseTypeLanguage := false
		baseType := ""
		modulePath := ""
		generic_arguments := []TypeInfo{}
		pointerCount := 0*/

	tailChild := node.Child(uint(int(node.ChildCount())) - 1)
	isOptional := !tailChild.IsNamed() && tailChild.Utf8Text(sourceCode) == "?"

	typeInfo := TypeInfo{
		Optional: isOptional,
		ASTNodeBase: NewBaseNodeBuilder().
			WithSitterPosRange(node.StartPosition(), node.EndPosition()).
			Build(),
	}

	typeInfo.ASTNodeBase = NewBaseNodeBuilder().
		WithSitterPosRange(node.StartPosition(), node.EndPosition()).
		Build()
	for i := 0; i < int(node.ChildCount()); i++ {
		n := node.Child(uint(i))
		// fmt.Println(n.Kind(), n.Utf8Text(sourceCode))
		switch n.Kind() {
		case "base_type_name":
			typeInfo.Identifier = NewIdentifierBuilder().
				WithName(n.Utf8Text(sourceCode)).
				WithSitterPos(n).
				Build()
			typeInfo.BuiltIn = true
		case "type_ident":
			typeInfo.Identifier = NewIdentifierBuilder().
				WithName(n.Utf8Text(sourceCode)).
				WithSitterPos(n).
				Build()
		case "generic_arguments":
			for g := 0; g < int(n.ChildCount()); g++ {
				gn := n.Child(uint(g))
				if gn.Kind() == "type" {
					gType := typeNodeToType(gn, sourceCode)
					typeInfo.Generics = append(typeInfo.Generics, gType)
				}
			}

		case "path_type_ident":
			var path, name string
			if n.ChildCount() == 2 {
				path = strings.Trim(n.Child(0).Utf8Text(sourceCode), ":")
				name = n.Child(1).Utf8Text(sourceCode)
			} else {
				name = n.Child(0).Utf8Text(sourceCode)
			}

			//fmt.Println(n)
			typeInfo.Identifier = NewIdentifierBuilder().
				WithPath(path).
				WithName(name).
				WithSitterPos(n).
				Build()

		case "type_suffix":
			suffix := n.Utf8Text(sourceCode)
			if suffix == "*" {
				// TODO Only covers pointer to final value
				typeInfo.Pointer = 1
			}
		}

	}

	// Is baseType a module generic argument? Flag it.
	/*isGenericArgument := false
	for genericId, _ := range currentModule.GenericParameters {
		if genericId == baseType {
			isGenericArgument = true
		}
	}


	var parsedType symbols.Type
	if len(generic_arguments) == 0 {
		if isOptional {
			parsedType = symbols.NewOptionalType(baseTypeLanguage, baseType, pointerCount, isGenericArgument, modulePath)
		} else {
			parsedType = symbols.NewType(baseTypeLanguage, baseType, pointerCount, isGenericArgument, modulePath)
		}
	} else {
		// TODO Can a type with generic be itself a generic argument?
		parsedType = symbols.NewTypeWithGeneric(baseTypeLanguage, isOptional, baseType, pointerCount, generic_arguments, modulePath)
	}*/

	return typeInfo
}
