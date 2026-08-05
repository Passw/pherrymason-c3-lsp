package project_state

import (
	"github.com/pherrymason/c3-lsp/internal/lsp/stdlib"
	"github.com/pherrymason/c3-lsp/pkg/symbols_table"
	"github.com/tliron/commonlog"
)

// SupportedC3Version: The C3 language version supported by the LSP.
const SupportedC3Version = "0.8.2"

// LoadStdLibByFile loads the standard library, returning one UnitModules per
// source file so that modules defined across multiple files are kept separate
// (as they are for workspace files). This avoids the overwrite problem that
// occurs when all modules share a single UnitModules entry.
//
// Uses disk and in-memory caching to avoid re-indexing on every startup.
func LoadStdLibByFile(logger commonlog.Logger, version string, c3cLibPath string) []symbols_table.UnitModules {
	detectedVersion := stdlib.GetDetectedC3Version()
	if detectedVersion != "" && detectedVersion != version {
		logger.Warningf("Requested C3 version %s does not match detected c3c binary version %s", version, detectedVersion)
	}
	logger.Infof("Loading stdlib for C3 version %s...", version)
	return stdlib.LoadOrBuildStdlibByFile(logger, version, c3cLibPath)
}
