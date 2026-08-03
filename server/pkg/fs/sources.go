package fs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveProjectSources returns the list of project source files for the given
// project root. Resolution is driven entirely by project.json:
//
//   - If project.json is missing: returns (nil, false, nil) — caller should
//     decide whether to fall back to a directory walk. The boolean indicates
//     that no project definition was present.
//   - If project.json is present and declares "sources": those patterns are
//     expanded against the project root, files matching any default or
//     declared exclude pattern are filtered out, only .c3/.c3i files are
//     kept, and the result is returned sorted and de-duplicated.
//   - If project.json is present but has no "sources": returns (nil, true, nil)
//     — caller should treat this as "no project sources declared" and index
//     nothing as project code.
//
// The second return value (hasConfig) lets callers distinguish between "no
// project.json" and "project.json exists but no sources". Errors are returned
// only for I/O failures while walking the filesystem.
func ResolveProjectSources(projectDir string, config *C3ProjectConfig) (files []string, hasConfig bool, err error) {
	if config == nil {
		return nil, false, nil
	}

	hasConfig = true

	// Sources may be declared at the top level (applied to all targets) or
	// per-target under "targets". When no top-level "sources" is declared,
	// collect the per-target lists. Pattern order across targets is
	// non-deterministic, so de-duplicate them.
	patterns := config.Sources
	if len(patterns) == 0 {
		seen := map[string]struct{}{}
		for _, target := range config.Targets {
			for _, p := range target.Sources {
				if p == "" {
					continue
				}
				if _, ok := seen[p]; ok {
					continue
				}
				seen[p] = struct{}{}
				patterns = append(patterns, p)
			}
		}
	}
	if len(patterns) == 0 {
		return nil, true, nil
	}

	canonicalRoot, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, true, fmt.Errorf("resolving project root: %w", err)
	}

	// Compile excludes once. A pattern without any "/**" or "**" suffix is
	// promoted to "match the entry OR anything beneath it" so that patterns
	// like "build" or ".git" exclude their contents as well as themselves.
	excludePatterns := append([]string(nil), DefaultExcludePatterns...)
	excludePatterns = append(excludePatterns, config.Excludes...)
	excludeMatchers := make([]globMatcher, 0, len(excludePatterns))
	for _, p := range excludePatterns {
		m, err := compileGlob(p)
		if err != nil {
			return nil, true, fmt.Errorf("invalid exclude pattern %q: %w", p, err)
		}
		excludeMatchers = append(excludeMatchers, m)
	}

	seen := map[string]struct{}{}
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		matches, err := expandSourcePattern(canonicalRoot, pattern)
		if err != nil {
			return nil, true, fmt.Errorf("expanding source pattern %q: %w", pattern, err)
		}
		for _, relPath := range matches {
			abs := filepath.Join(canonicalRoot, relPath)
			if !isC3SourceFile(abs) {
				continue
			}
			if isExcluded(canonicalRoot, relPath, excludeMatchers) {
				continue
			}
			if _, ok := seen[abs]; ok {
				continue
			}
			seen[abs] = struct{}{}
			files = append(files, abs)
		}
	}

	sort.Strings(files)
	return files, true, nil
}

// ScanProjectFallback performs the original behavior: walk the workspace and
// return every .c3/.c3i file, minus anything matching the default or supplied
// exclude patterns. Used as a fallback when a project has no project.json.
func ScanProjectFallback(projectDir string, extraExcludes []string) ([]string, error) {
	canonicalRoot, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, fmt.Errorf("resolving project root: %w", err)
	}

	excludePatterns := append([]string(nil), DefaultExcludePatterns...)
	excludePatterns = append(excludePatterns, extraExcludes...)
	excludeMatchers := make([]globMatcher, 0, len(excludePatterns))
	for _, p := range excludePatterns {
		m, err := compileGlob(p)
		if err != nil {
			return nil, fmt.Errorf("invalid exclude pattern %q: %w", p, err)
		}
		excludeMatchers = append(excludeMatchers, m)
	}

	var files []string
	walkErr := filepath.WalkDir(canonicalRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Don't abort the whole walk on a permission error etc.; skip
			// the offending path and continue.
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			// Exclude applies to directories too so we can short-circuit.
			rel, relErr := filepath.Rel(canonicalRoot, path)
			if relErr == nil && rel != "." {
				if isExcluded(canonicalRoot, rel, excludeMatchers) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if !isC3SourceFile(path) {
			return nil
		}

		rel, relErr := filepath.Rel(canonicalRoot, path)
		if relErr != nil {
			return nil
		}
		if isExcluded(canonicalRoot, rel, excludeMatchers) {
			return nil
		}

		files = append(files, path)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Strings(files)
	return files, nil
}

func isC3SourceFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".c3" || ext == ".c3i"
}

func isExcluded(root, relPath string, matchers []globMatcher) bool {
	if relPath == "" || relPath == "." {
		return false
	}
	normalized := filepath.ToSlash(relPath)
	pathSegs := strings.Split(normalized, "/")
	for _, m := range matchers {
		if m.matchesAnyUnder(pathSegs) {
			return true
		}
	}
	return false
}

// expandSourcePattern walks the matched paths for a single "sources" entry.
// Each pattern is interpreted relative to the project root. A trailing "/**"
// is treated as "match everything beneath".
func expandSourcePattern(root, pattern string) ([]string, error) {
	cleanPattern := filepath.ToSlash(filepath.Clean(pattern))
	cleanPattern = strings.TrimPrefix(cleanPattern, "/")

	// Special handling: trailing /** -> walk the directory matching everything
	// underneath (and the dir itself, for completeness).
	if strings.HasSuffix(cleanPattern, "/**") {
		prefix := strings.TrimSuffix(cleanPattern, "/**")
		base := filepath.Join(root, filepath.FromSlash(prefix))
		info, err := os.Stat(base)
		if err != nil {
			if os.IsNotExist(err) {
				// Pattern references a missing path; warn the caller via
				// error so they can log it, but don't fail the whole index.
				return nil, nil
			}
			return nil, err
		}
		if !info.IsDir() {
			// Not a directory: nothing to expand.
			return nil, nil
		}
		var matches []string
		walkErr := filepath.WalkDir(base, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if path == base {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			matches = append(matches, filepath.ToSlash(rel))
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
		return matches, nil
	}

	// Special handling: a "directory" pattern (no glob chars) -> expand to
	// every .c3/.c3i file directly inside it. This matches the common C3
	// convention of "sources": ["src"] meaning "every C3 file directly
	// under src", without recursing into nested build directories. The
	// exclude stage handles unwanted nested dirs.
	if !containsGlobChar(cleanPattern) {
		base := filepath.Join(root, filepath.FromSlash(cleanPattern))
		info, err := os.Stat(base)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		if !info.IsDir() {
			// Plain file reference: include if it's a source file, else
			// ignore (handled by caller via isC3SourceFile).
			rel, relErr := filepath.Rel(root, base)
			if relErr != nil {
				return nil, nil
			}
			return []string{filepath.ToSlash(rel)}, nil
		}

		var matches []string
		entries, err := os.ReadDir(base)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			rel, relErr := filepath.Rel(root, filepath.Join(base, e.Name()))
			if relErr != nil {
				continue
			}
			matches = append(matches, filepath.ToSlash(rel))
		}
		return matches, nil
	}

	// General glob: walk the root and collect everything that matches.
	matcher, err := compileGlob(cleanPattern)
	if err != nil {
		return nil, err
	}
	var matches []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			// Prune: don't descend into dirs that obviously can't match
			// (cheap directory-name check against the first path segment
			// of the glob).
			if !matcher.couldMatchDir(filepath.Base(rel)) {
				return filepath.SkipDir
			}
			return nil
		}
		normalized := filepath.ToSlash(rel)
		if matcher.matches(normalized) {
			matches = append(matches, normalized)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return matches, nil
}

func containsGlobChar(p string) bool {
	return strings.ContainsAny(p, "*?[")
}
