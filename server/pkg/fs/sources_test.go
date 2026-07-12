package fs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobMatcher(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"build", "build", true},
		{"build", "build/x.c3", false},
		{"build/**", "build", false},
		{"build/**", "build/x.c3", true},
		{"build/**", "build/sub/x.c3", true},
		{"build/**", "src/x.c3", false},
		{"src/**", "src/foo.c3", true},
		{"src/**", "src/sub/foo.c3", true},
		{"src/*", "src/foo.c3", true},
		{"src/*", "src/sub/foo.c3", false},
		{"**/*.c3", "src/foo.c3", true},
		{"**/*.c3", "a/b/foo.c3", true},
		{"**/*.c3", "src/foo.c3i", false},
		{".git", ".git", true},
		{".git/**", ".git/HEAD", true},
		{".git/**", ".git/sub/HEAD", true},
		{"[abc].c3", "a.c3", true},
		{"[abc].c3", "d.c3", false},
		{"[!abc].c3", "d.c3", true},
	}

	for _, tc := range cases {
		m, err := compileGlob(tc.pattern)
		require.NoError(t, err, "compile %q", tc.pattern)
		assert.Equalf(t, tc.want, m.matches(tc.path), "pattern=%q path=%q", tc.pattern, tc.path)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestResolveProjectSources_NoProjectConfig(t *testing.T) {
	tmp := t.TempDir()
	files, hasConfig, err := ResolveProjectSources(tmp, nil)
	assert.NoError(t, err)
	assert.False(t, hasConfig)
	assert.Nil(t, files)
}

func TestResolveProjectSources_ProjectJsonNoSources(t *testing.T) {
	tmp := t.TempDir()
	cfg := &C3ProjectConfig{Dependencies: []string{"foo"}}
	files, hasConfig, err := ResolveProjectSources(tmp, cfg)
	assert.NoError(t, err)
	assert.True(t, hasConfig)
	assert.Nil(t, files, "no sources declared -> nothing indexed")
}

func TestResolveProjectSources_TrailingStarStar(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, filepath.Join(tmp, "src/a.c3"), "module a;")
	writeFile(t, filepath.Join(tmp, "src/sub/b.c3"), "module b;")
	writeFile(t, filepath.Join(tmp, "src/c.c3i"), "module c;")
	writeFile(t, filepath.Join(tmp, "README.md"), "nope")

	cfg := &C3ProjectConfig{Sources: []string{"src/**"}}
	files, hasConfig, err := ResolveProjectSources(tmp, cfg)
	require.NoError(t, err)
	assert.True(t, hasConfig)
	assert.ElementsMatch(t,
		[]string{
			filepath.Join(tmp, "src/a.c3"),
			filepath.Join(tmp, "src/c.c3i"),
			filepath.Join(tmp, "src/sub/b.c3"),
		},
		files,
	)
}

func TestResolveProjectSources_DirectoryPattern(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, filepath.Join(tmp, "src/a.c3"), "module a;")
	writeFile(t, filepath.Join(tmp, "src/sub/b.c3"), "module b;")
	writeFile(t, filepath.Join(tmp, "src/c.c3i"), "module c;")

	cfg := &C3ProjectConfig{Sources: []string{"src"}}
	files, _, err := ResolveProjectSources(tmp, cfg)
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{
			filepath.Join(tmp, "src/a.c3"),
			filepath.Join(tmp, "src/c.c3i"),
		},
		files,
		"directory pattern should only pick up top-level entries",
	)
}

func TestResolveProjectSources_ExcludeBuiltDirs(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, filepath.Join(tmp, "src/a.c3"), "module a;")
	writeFile(t, filepath.Join(tmp, "build/a.c3"), "module b;")
	writeFile(t, filepath.Join(tmp, ".git/HEAD"), "ref: refs/heads/x")
	writeFile(t, filepath.Join(tmp, "dist/c.c3"), "module d;")

	cfg := &C3ProjectConfig{Sources: []string{"**"}}
	files, _, err := ResolveProjectSources(tmp, cfg)
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{filepath.Join(tmp, "src/a.c3")},
		files,
	)
}

func TestResolveProjectSources_ExcludesDeclared(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, filepath.Join(tmp, "src/a.c3"), "module a;")
	writeFile(t, filepath.Join(tmp, "src/excluded/x.c3"), "module y;")
	writeFile(t, filepath.Join(tmp, "src/kept/y.c3"), "module z;")

	cfg := &C3ProjectConfig{
		Sources:  []string{"src/**"},
		Excludes: []string{"src/excluded/**"},
	}
	files, _, err := ResolveProjectSources(tmp, cfg)
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{
			filepath.Join(tmp, "src/a.c3"),
			filepath.Join(tmp, "src/kept/y.c3"),
		},
		files,
	)
}

func TestResolveProjectSources_DuplicatesAcrossPatterns(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, filepath.Join(tmp, "src/a.c3"), "module a;")
	writeFile(t, filepath.Join(tmp, "src/sub/b.c3"), "module b;")

	cfg := &C3ProjectConfig{
		Sources: []string{"src/a.c3", "**/*.c3"},
	}
	files, _, err := ResolveProjectSources(tmp, cfg)
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestScanProjectFallback_RespectsDefaults(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, filepath.Join(tmp, "src/a.c3"), "module a;")
	writeFile(t, filepath.Join(tmp, "build/c.c3"), "module c;")
	writeFile(t, filepath.Join(tmp, "out/d.c3i"), "module d;")
	writeFile(t, filepath.Join(tmp, "README.md"), "nope")

	files, err := ScanProjectFallback(tmp, nil)
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{filepath.Join(tmp, "src/a.c3")},
		files,
	)
}

func TestScanProjectFallback_ExtraExcludes(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, filepath.Join(tmp, "src/a.c3"), "module a;")
	writeFile(t, filepath.Join(tmp, "scratch/x.c3"), "module x;")

	files, err := ScanProjectFallback(tmp, []string{"scratch/**"})
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{filepath.Join(tmp, "src/a.c3")},
		files,
	)
}
