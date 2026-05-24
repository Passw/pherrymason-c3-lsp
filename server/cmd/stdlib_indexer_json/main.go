// stdlib_indexer_json is a helper tool that tests stdlib indexing by
// parsing C3 source files and reporting how many modules/functions were found.
// It is not used by the LSP at runtime; the LSP indexes stdlib directly.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pherrymason/c3-lsp/pkg/document"
	"github.com/pherrymason/c3-lsp/pkg/fs"
	p "github.com/pherrymason/c3-lsp/pkg/parser"
	"github.com/tliron/commonlog"
)

func main() {
	// Allow custom path via argument
	var c3cPath string
	if len(os.Args) > 1 {
		c3cPath = os.Args[1]
	} else {
		c3cPath = filepath.Join("..", "..", "..", "assets", "c3c")
	}

	c3cVersion := getC3Version(c3cPath)
	fmt.Printf("C3 Version detected: %s\n", c3cVersion)

	baseLibPath := fs.GetCanonicalPath(filepath.Join(c3cPath, "lib"))
	files, err := fs.ScanForC3(filepath.Join(baseLibPath, "std"))
	if err != nil {
		panic(fmt.Errorf("failed to scan for C3 files: %v", err))
	}
	fmt.Printf("Found %d C3 files to parse\n", len(files))

	commonlog.Configure(2, nil)
	logger := commonlog.GetLogger("")
	parser := p.NewParser(logger)

	totalModules := 0
	totalFunctions := 0

	for i, filePath := range files {
		relPath, _ := filepath.Rel(baseLibPath, filePath)
		fmt.Printf("Parsing (%03d / %03d): %s\n", i+1, len(files), relPath)

		content, err := os.ReadFile(filePath)
		if err != nil {
			panic(fmt.Errorf("could not read file %s: %v", filePath, err))
		}

		doc := document.NewDocumentFromString(filePath, string(content))
		modules, _ := parser.ParseSymbols(&doc)

		for _, mod := range modules.Modules() {
			totalModules++
			totalFunctions += len(mod.ChildrenFunctions)
		}
	}

	fmt.Printf("\nTotal modules indexed: %d\n", totalModules)
	fmt.Printf("Total functions indexed: %d\n", totalFunctions)
	fmt.Printf("\nNote: stdlib is indexed per-file at LSP startup and cached to disk for subsequent sessions.\n")
}

func getC3Version(path string) string {
	versionFile := filepath.Join(path, "src", "version.h")
	content, err := os.ReadFile(versionFile)
	if err != nil {
		panic(fmt.Sprintf("Could not find c3c version: Could not open %s file: %s", versionFile, err))
	}

	text := string(content)
	versionRegex := regexp.MustCompile(`#define\s+COMPILER_VERSION\s+"([^"]+)"`)
	versionMatch := versionRegex.FindStringSubmatch(text)
	if len(versionMatch) > 1 {
		return versionMatch[1]
	}

	panic("Could not find c3c version: Did not find COMPILER_VERSION in version.h")
}
