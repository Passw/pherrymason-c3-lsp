package cst

//#include "tree_sitter/parser.h"
//TSLanguage *tree_sitter_c3();
import "C"
import (
	"context"
	"fmt"
	"unsafe"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

var Language *sitter.Language

func init() {
	languagePtr := unsafe.Pointer(C.tree_sitter_c3())
	if languagePtr == nil {
		panic("Couldnt get c3 tree sitter language")
	}
	Language = sitter.NewLanguage(languagePtr)
}

func NewSitterParser() *sitter.Parser {
	parser := sitter.NewParser()
	if err := parser.SetLanguage(Language); err != nil {
		panic(fmt.Errorf("failed setting language: %v", err))
	}

	return parser
}

func GetParsedTreeFromString(source string) *sitter.Tree {
	sourceCode := []byte(source)
	parser := NewSitterParser()
	n := parser.ParseCtx(context.Background(), sourceCode, nil)

	return n
}

func RunQuery(query *sitter.Query, node *sitter.Node) *sitter.QueryMatches {
	qc := sitter.NewQueryCursor()
	matches := qc.Matches(query, node, nil)

	return &matches
}
