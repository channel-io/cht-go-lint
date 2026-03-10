package lint

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// CodebaseAnalyzer walks and parses Go source files, caching results.
type CodebaseAnalyzer struct {
	root         string
	modulePath   string
	strategy     LocationStrategy
	excludePaths []string
	fset         *token.FileSet

	mu    sync.RWMutex
	cache map[string]*ParsedFile
}

// NewAnalyzer creates a new codebase analyzer.
func NewAnalyzer(root, modulePath string, strategy LocationStrategy, excludePaths []string) *CodebaseAnalyzer {
	return &CodebaseAnalyzer{
		root:         root,
		modulePath:   modulePath,
		strategy:     strategy,
		excludePaths: excludePaths,
		fset:         token.NewFileSet(),
		cache:        make(map[string]*ParsedFile),
	}
}

// ResetCache clears all cached parsed files and resets the file set.
// This is used after auto-fix to re-parse modified files.
func (a *CodebaseAnalyzer) ResetCache() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cache = make(map[string]*ParsedFile)
	a.fset = token.NewFileSet()
}

// IsExcluded checks whether a relative path should be excluded from analysis.
func (a *CodebaseAnalyzer) IsExcluded(relPath string) bool {
	for _, prefix := range a.excludePaths {
		p := filepath.ToSlash(strings.TrimSuffix(prefix, "/"))
		if relPath == p || strings.HasPrefix(relPath, p+"/") {
			return true
		}
	}
	return false
}

// Root returns the project root directory.
func (a *CodebaseAnalyzer) Root() string { return a.root }

// ModulePath returns the Go module path.
func (a *CodebaseAnalyzer) ModulePath() string { return a.modulePath }

// FileSet returns the shared token file set.
func (a *CodebaseAnalyzer) FileSet() *token.FileSet { return a.fset }

// Strategy returns the location strategy, or nil if not set.
func (a *CodebaseAnalyzer) Strategy() LocationStrategy { return a.strategy }

// ParseFile parses a Go file and caches the result.
func (a *CodebaseAnalyzer) ParseFile(path string) (*ParsedFile, error) {
	a.mu.RLock()
	if pf, ok := a.cache[path]; ok {
		a.mu.RUnlock()
		return pf, nil
	}
	a.mu.RUnlock()

	a.mu.Lock()
	defer a.mu.Unlock()

	// Double-check after acquiring write lock
	if pf, ok := a.cache[path]; ok {
		return pf, nil
	}

	f, err := parser.ParseFile(a.fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	relPath, _ := filepath.Rel(a.root, path)
	relPath = filepath.ToSlash(relPath)

	pf := &ParsedFile{
		Path:    path,
		RelPath: relPath,
		Package: f.Name.Name,
		AST:     f,
		FileSet: a.fset,
	}

	pf.Imports = extractImports(f, a.fset)
	pf.Types = extractTypes(f, a.fset)
	pf.Funcs = extractFuncs(f, a.fset)

	if a.strategy != nil {
		pf.Location = a.strategy.Identify(relPath)
	}

	a.cache[path] = pf
	return pf, nil
}

// skipDirs contains directory names that should be skipped during walking.
var skipDirs = map[string]bool{
	"vendor":    true,
	"testdata":  true,
	".git":      true,
	"generated": true,
	"node_modules": true,
}

// WalkGoFiles walks all Go source files (excluding test files and skipped dirs).
func (a *CodebaseAnalyzer) WalkGoFiles(fn func(path string, file *ParsedFile) error) error {
	return filepath.WalkDir(a.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			if len(a.excludePaths) > 0 {
				rel, _ := filepath.Rel(a.root, path)
				if rel != "." && a.IsExcluded(filepath.ToSlash(rel)) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !isGoSourceFile(d.Name()) {
			return nil
		}
		pf, err := a.ParseFile(path)
		if err != nil {
			return nil // skip unparseable files
		}
		return fn(path, pf)
	})
}

// WalkDir walks Go source files under a specific directory relative to root.
func (a *CodebaseAnalyzer) WalkDir(dir string, fn func(path string, file *ParsedFile) error) error {
	fullDir := filepath.Join(a.root, dir)
	if _, err := os.Stat(fullDir); os.IsNotExist(err) {
		return nil
	}
	return filepath.WalkDir(fullDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			if len(a.excludePaths) > 0 {
				rel, _ := filepath.Rel(a.root, path)
				if rel != "." && a.IsExcluded(filepath.ToSlash(rel)) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !isGoSourceFile(d.Name()) {
			return nil
		}
		pf, err := a.ParseFile(path)
		if err != nil {
			return nil
		}
		return fn(path, pf)
	})
}

// WalkDirs walks Go source files under multiple directories.
func (a *CodebaseAnalyzer) WalkDirs(dirs []string, fn func(path string, file *ParsedFile) error) error {
	for _, dir := range dirs {
		if err := a.WalkDir(dir, fn); err != nil {
			return err
		}
	}
	return nil
}

// IsInternalImport checks if an import path belongs to this module.
func (a *CodebaseAnalyzer) IsInternalImport(importPath string) bool {
	return strings.HasPrefix(importPath, a.modulePath+"/") || importPath == a.modulePath
}

// ImportLocation returns the architectural location for an import path.
func (a *CodebaseAnalyzer) ImportLocation(importPath string) ImportLocation {
	if a.strategy == nil {
		return ImportLocation{IsSameModule: a.IsInternalImport(importPath)}
	}
	return a.strategy.ParseImport(importPath, a.modulePath)
}

// ListDirs returns subdirectories under a given relative path.
func (a *CodebaseAnalyzer) ListDirs(relDir string) ([]string, error) {
	fullDir := filepath.Join(a.root, relDir)
	entries, err := os.ReadDir(fullDir)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !skipDirs[e.Name()] {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs, nil
}

func isGoSourceFile(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}
