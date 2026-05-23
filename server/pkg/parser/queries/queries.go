package queries

import (
	_ "embed"
	"fmt"

	"github.com/pherrymason/c3-lsp/internal/lsp/cst"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

//go:embed symbols.scm
var symbolsQueryRaw []byte

//go:embed local-var-declaration.scm
var localVarDeclQueryRaw []byte

var SymbolsQuery, LocalVarDeclQuery *sitter.Query

func init() {
	var qErr *sitter.QueryError
	SymbolsQuery, qErr = sitter.NewQuery(cst.Language, string(symbolsQueryRaw))
	if qErr != nil {
		panic(fmt.Errorf("could not create query symbols: %v", qErr))
	}
	LocalVarDeclQuery, qErr = sitter.NewQuery(cst.Language, string(localVarDeclQueryRaw))
	if qErr != nil {
		panic(fmt.Errorf("could not create query local var declaration: %v", qErr))
	}
}
