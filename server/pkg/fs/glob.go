package fs

import (
	"path/filepath"
	"strings"
)

// globMatcher matches forward-slash normalized paths against a C3-style glob
// pattern. Segment matching is delegated to filepath.Match, which supports
// '*', '?', and character classes; a '**' segment additionally matches one or
// more whole path segments (the recursive semantics c3c uses for source and
// exclude patterns).
type globMatcher struct {
	segs []string
}

// compileGlob parses a forward-slash pattern. A leading "./" or "/" is
// stripped so callers can pass patterns like "src/**".
func compileGlob(pattern string) (globMatcher, error) {
	clean := filepath.ToSlash(pattern)
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "/")

	var segs []string
	for _, seg := range strings.Split(clean, "/") {
		if seg == "" {
			continue
		}
		if seg != "**" {
			// filepath.Match negates classes with '^' only; translate the
			// POSIX '!' form c3c accepts.
			seg = strings.ReplaceAll(seg, "[!", "[^")
			if _, err := filepath.Match(seg, ""); err != nil {
				return globMatcher{}, err
			}
		}
		segs = append(segs, seg)
	}
	return globMatcher{segs: segs}, nil
}

// matches reports whether the path matches the pattern.
func (m globMatcher) matches(path string) bool {
	if path == "" || path == "." {
		return false
	}
	return matchSegments(m.segs, strings.Split(path, "/"), 0, 0)
}

// matchesAnyUnder reports whether the pattern matches the path itself or, for
// a single-segment pattern, any segment of it. An exclude entry like "build"
// therefore covers the directory itself, everything beneath it, and nested
// directories of the same name anywhere in the tree.
func (m globMatcher) matchesAnyUnder(path string) bool {
	if m.matches(path) {
		return true
	}
	if len(m.segs) != 1 {
		return false
	}
	for _, seg := range strings.Split(path, "/") {
		if seg != "" && m.matches(seg) {
			return true
		}
	}
	return false
}

// matchSegments matches pattern segments against path segments, where a "**"
// segment consumes one or more whole path segments.
func matchSegments(pats []string, pathSegs []string, pi, si int) bool {
	for pi < len(pats) {
		if pats[pi] == "**" {
			if si >= len(pathSegs) {
				return false
			}
			for j := si; j < len(pathSegs); j++ {
				if matchSegments(pats, pathSegs, pi+1, j+1) {
					return true
				}
			}
			return false
		}
		if si >= len(pathSegs) {
			return false
		}
		ok, _ := filepath.Match(pats[pi], pathSegs[si])
		if !ok {
			return false
		}
		pi++
		si++
	}
	return si == len(pathSegs)
}
