package server

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pherrymason/c3-lsp/internal/c3c"
	"github.com/pherrymason/c3-lsp/pkg/option"
)

type DiagnosticsOpts struct {
	Enabled bool          `json:"enabled"`
	Delay   time.Duration `json:"delay"`
}

// ServerOpts holds the options to create a new Server.
type ServerOpts struct {
	C3          c3c.C3Opts      `json:"C3Opts"`
	Diagnostics DiagnosticsOpts `json:"Diagnostics"`

	LogFilepath      option.Option[string]
	SendCrashReports bool
	Debug            bool
}

type ServerOptsJson struct {
	C3 struct {
		Version     *string  `json:"version,omitempty"`
		Path        *string  `json:"path,omitempty"`
		StdlibPath  *string  `json:"stdlib-path,omitempty"`
		CompileArgs []string `json:"compile-args"`
	}

	Diagnostics struct {
		Enabled bool          `json:"enabled"`
		Delay   time.Duration `json:"delay"`
	}
}

func (s *Server) loadServerConfigurationForWorkspace(path string) {
	file, err := os.Open(path + "/c3lsp.json")
	if err != nil {
		// No configuration project file found - use defaults
		s.server.Log.Infof("No configuration " + path + "/c3lsp.json found")
		// Still need to initialize stdlib with default version
		s.applyVersionAndLoadStdlib(option.None[string]())
		return
	}
	defer file.Close()

	s.server.Log.Infof("Reading configuration " + path + "/c3lsp.json")

	// Lee el archivo
	data, err := io.ReadAll(file)
	if err != nil {
		s.server.Log.Errorf("Error reading config json: %v", err)
	}
	s.server.Log.Infof("%s", data)

	var options ServerOptsJson
	err = json.Unmarshal(data, &options)
	if err != nil {
		s.server.Log.Errorf("Error deserializing config json: %v", err)
	}

	if options.C3.StdlibPath != nil {
		s.options.C3.StdlibPath = option.Some(*options.C3.StdlibPath)
		s.server.Log.Infof("Stdlib:%s", *options.C3.StdlibPath)
		s.server.Log.Infof("Setted Stdlib:%s", s.options.C3.StdlibPath.Get())
	}

	// Store user-configured version (from c3lsp.json)
	var userConfiguredVersion option.Option[string]
	if options.C3.Version != nil {
		userConfiguredVersion = option.Some(*options.C3.Version)
	}

	if options.C3.Path != nil {
		s.options.C3.Path = option.Some(*options.C3.Path)
	}

	if len(options.C3.CompileArgs) > 0 {
		s.options.C3.CompileArgs = options.C3.CompileArgs
	}

	// Apply version and load stdlib
	s.applyVersionAndLoadStdlib(userConfiguredVersion)

	// Change log filepath?
	// Should be able to do that from c3lsp.json?

	// Enable/disable sendCrashReports
	// Should be able to do that from c3lsp.json?
}

// applyVersionAndLoadStdlib determines the C3 version and loads stdlib
func (s *Server) applyVersionAndLoadStdlib(userConfiguredVersion option.Option[string]) {
	// Detect version from binary (either from configured path or system PATH)
	detectedBinaryVersion := c3c.GetC3Version(s.options.C3.Path)

	// Validate and set the final version to use
	var finalVersion option.Option[string]
	if userConfiguredVersion.IsSome() && detectedBinaryVersion.IsSome() {
		// Both configured and detected - check for mismatch
		if userConfiguredVersion.Get() != detectedBinaryVersion.Get() {
			s.server.Log.Warningf("Version mismatch detected!")
			s.server.Log.Warningf("c3lsp.json specifies version: %s", userConfiguredVersion.Get())
			s.server.Log.Warningf("c3c binary reports version: %s", detectedBinaryVersion.Get())
			s.server.Log.Warningf("Using detected binary version: %s", detectedBinaryVersion.Get())
		}
		// Use detected version (the actual binary version)
		finalVersion = detectedBinaryVersion
	} else if detectedBinaryVersion.IsSome() {
		// Only detected version available
		finalVersion = detectedBinaryVersion
	} else if userConfiguredVersion.IsSome() {
		// Only user configured version available (binary not found or version not detected)
		s.server.Log.Warningf("Could not detect c3c binary version. Using configured version: %s", userConfiguredVersion.Get())
		finalVersion = userConfiguredVersion
	}
	// else: no version at all, will default to latest supported

	s.options.C3.Version = finalVersion

	requestedLanguageVersion := checkRequestedLanguageVersion(s.server.Log, s.options.C3.Version)

	// Determine c3cLibPath to pass to SetLanguageVersion
	// Priority: explicit stdlib-path > configured c3c path > "c3c" on system PATH
	c3cLibPath := ""
	if s.options.C3.StdlibPath.IsSome() {
		c3cLibPath = s.options.C3.StdlibPath.Get()
	} else if s.options.C3.Path.IsSome() {
		c3cLibPath = resolveStdlibFromBinary(s.options.C3.Path.Get())
	} else {
		// No explicit path configured: try "c3c" on the system PATH
		c3cLibPath = resolveStdlibFromBinary("c3c")
	}

	s.state.SetLanguageVersion(requestedLanguageVersion, c3cLibPath)
}

// resolveStdlibFromBinary takes a c3c binary name or path (e.g. "c3c" or
// "/usr/bin/c3c") and returns the path to the C3 standard library directory
// (the "lib/c3" directory alongside the binary's install prefix).
//
// Typical layouts:
//
//	/usr/bin/c3c        → /usr/lib/c3
//	/opt/homebrew/bin/c3c → /opt/homebrew/lib/c3
//	/path/to/c3c        → /path/to/lib          (fallback: binary-dir/lib)
func resolveStdlibFromBinary(binaryPath string) string {
	// Resolve the binary to an absolute path (handles bare "c3c" via PATH)
	absPath, err := exec.LookPath(binaryPath)
	if err != nil {
		// Not on PATH — treat as a literal path
		absPath = binaryPath
	}

	// Resolve symlinks so we get the real install location
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		resolved = absPath
	}

	binDir := filepath.Dir(resolved)
	installPrefix := filepath.Dir(binDir) // parent of bin/

	// Standard layout: <prefix>/lib/c3/std
	candidate := filepath.Join(installPrefix, "lib", "c3")
	if info, err := os.Stat(filepath.Join(candidate, "std")); err == nil && info.IsDir() {
		return candidate
	}

	// Fallback: some distributions put everything flat: <prefix>/lib/std
	candidate2 := filepath.Join(installPrefix, "lib")
	if info, err := os.Stat(filepath.Join(candidate2, "std")); err == nil && info.IsDir() {
		return candidate2
	}

	// Last resort: binary directory itself (dev/source builds where c3c lives next to lib/)
	candidate3 := filepath.Join(binDir, "lib")
	if info, err := os.Stat(filepath.Join(candidate3, "std")); err == nil && info.IsDir() {
		return candidate3
	}

	return ""
}
