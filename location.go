package lint

import "strings"

// LocationStrategy maps file paths and import paths to architectural locations.
// This is the primary extension point for adapting the linter to different project structures.
type LocationStrategy interface {
	// Identify returns the architectural location for a file path relative to the project root.
	Identify(relPath string) Location
	// ParseImport returns the architectural location for an import path.
	ParseImport(importPath, modulePath string) ImportLocation
}

// Location represents the architectural position of a source file.
type Location struct {
	Component    string            // top-level module/domain (e.g., "app", "user", "order")
	SubComponent string            // sub-module/subdomain (e.g., "channel", "membership")
	Layer        string            // architectural layer (e.g., "model", "repo", "svc")
	Tags         map[string]string // extensible metadata (e.g., "isAlias": "true")
}

// ImportLocation represents the architectural position derived from an import path.
type ImportLocation struct {
	Component    string
	SubComponent string
	Layer        string
	IsInternal   bool // within the same module
	IsSameModule bool // same Go module
}

// HasComponent returns true if a component is set.
func (l Location) HasComponent() bool {
	return l.Component != ""
}

// HasSubComponent returns true if a sub-component is set.
func (l Location) HasSubComponent() bool {
	return l.SubComponent != ""
}

// HasLayer returns true if a layer is set.
func (l Location) HasLayer() bool {
	return l.Layer != ""
}

// Tag returns a tag value, or empty string if not set.
func (l Location) Tag(key string) string {
	if l.Tags == nil {
		return ""
	}
	return l.Tags[key]
}

// SameComponent returns true if two locations share the same component.
func (l Location) SameComponent(other Location) bool {
	return l.Component != "" && l.Component == other.Component
}

// SameSubComponent returns true if two locations share the same component and sub-component.
func (l Location) SameSubComponent(other Location) bool {
	return l.SameComponent(other) && l.SubComponent != "" && l.SubComponent == other.SubComponent
}

// --- Built-in: Nested Domain Strategy ---
// Pattern: internal/domain/{component}/subdomain/{subcomponent}/{layer}/
// Also handles: internal/saga/{name}/, api/{handler_type}/

// NestedDomainStrategy handles the ch-app-store-like directory structure.
type NestedDomainStrategy struct {
	DomainRoot   string            // default: "internal/domain"
	SubdomainDir string            // default: "subdomain"
	SagaRoot     string            // default: "internal/saga"
	HandlerRoots []string          // default: ["api/http", "api/jsonrpc"]
	LayerDirs    map[string]string // dir name → layer name (e.g., "svc" → "service")
}

func NewNestedDomainStrategy(cfg *Config) *NestedDomainStrategy {
	s := &NestedDomainStrategy{
		DomainRoot:   "internal/domain",
		SubdomainDir: "subdomain",
		SagaRoot:     "internal/saga",
		HandlerRoots: []string{"api/http", "api/jsonrpc"},
		LayerDirs:    make(map[string]string),
	}
	// Map layer names and aliases to canonical layer names
	for _, l := range cfg.Layers {
		s.LayerDirs[l.Name] = l.Name
		for _, a := range l.Aliases {
			s.LayerDirs[a] = l.Name
		}
	}
	// Apply location options if provided
	if cfg.Location != nil && cfg.Location.Options != nil {
		if v, ok := cfg.Location.Options["domain_root"].(string); ok {
			s.DomainRoot = v
		}
		if v, ok := cfg.Location.Options["subdomain_dir"].(string); ok {
			s.SubdomainDir = v
		}
		if v, ok := cfg.Location.Options["saga_root"].(string); ok {
			s.SagaRoot = v
		}
	}
	return s
}

func (s *NestedDomainStrategy) Identify(relPath string) Location {
	loc := Location{Tags: make(map[string]string)}
	normalized := strings.ReplaceAll(relPath, "\\", "/")

	// Check handler roots
	for _, hr := range s.HandlerRoots {
		if strings.HasPrefix(normalized, hr+"/") || strings.HasPrefix(normalized, hr+"\\") {
			loc.Layer = "handler"
			loc.Tags["handler_type"] = hr
			return loc
		}
	}

	// Check saga root
	if strings.HasPrefix(normalized, s.SagaRoot+"/") {
		rest := strings.TrimPrefix(normalized, s.SagaRoot+"/")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) > 0 {
			loc.Component = parts[0]
		}
		loc.Layer = "saga"
		return loc
	}

	// Check domain root
	if strings.HasPrefix(normalized, s.DomainRoot+"/") {
		rest := strings.TrimPrefix(normalized, s.DomainRoot+"/")
		parts := strings.Split(rest, "/")

		// Extract component (first directory under domain root)
		if len(parts) > 0 {
			loc.Component = parts[0]
		}

		// Look for subdomain directory
		for i, p := range parts {
			if p == s.SubdomainDir && i+1 < len(parts) {
				loc.SubComponent = parts[i+1]
				// Look for layer after subdomain name
				for _, remaining := range parts[i+2:] {
					if layer, ok := s.LayerDirs[remaining]; ok {
						loc.Layer = layer
						break
					}
				}
				break
			}
		}

		// If no subdomain found, look for layer directly under component
		if loc.Layer == "" {
			for _, p := range parts[1:] {
				if layer, ok := s.LayerDirs[p]; ok {
					loc.Layer = layer
					break
				}
			}
		}

		// Check for alias file
		if strings.HasSuffix(normalized, "/alias.go") {
			loc.Tags["isAlias"] = "true"
		}

		return loc
	}

	return loc
}

func (s *NestedDomainStrategy) ParseImport(importPath, modulePath string) ImportLocation {
	iloc := ImportLocation{}
	if !strings.HasPrefix(importPath, modulePath) {
		return iloc
	}
	iloc.IsSameModule = true

	// Get the path relative to module
	relPath := strings.TrimPrefix(importPath, modulePath+"/")
	loc := s.Identify(relPath)

	iloc.Component = loc.Component
	iloc.SubComponent = loc.SubComponent
	iloc.Layer = loc.Layer
	iloc.IsInternal = strings.HasPrefix(relPath, "internal/")
	return iloc
}

// --- Built-in: Flat Package Strategy ---
// Pattern: {root}/{component}/{layer}/ or {root}/{layer}/

// FlatPkgStrategy handles simple project structures.
type FlatPkgStrategy struct {
	Roots     []string          // directories to scan (default: ["internal", "pkg"])
	LayerDirs map[string]string // dir name → layer name
}

func NewFlatPkgStrategy(cfg *Config) *FlatPkgStrategy {
	s := &FlatPkgStrategy{
		Roots:     []string{"internal", "pkg"},
		LayerDirs: make(map[string]string),
	}
	for _, l := range cfg.Layers {
		s.LayerDirs[l.Name] = l.Name
		for _, a := range l.Aliases {
			s.LayerDirs[a] = l.Name
		}
	}
	if cfg.Location != nil && cfg.Location.Options != nil {
		if v, ok := cfg.Location.Options["roots"]; ok {
			if roots, ok := v.([]any); ok {
				s.Roots = make([]string, 0, len(roots))
				for _, r := range roots {
					if str, ok := r.(string); ok {
						s.Roots = append(s.Roots, str)
					}
				}
			}
		}
	}
	return s
}

func (s *FlatPkgStrategy) Identify(relPath string) Location {
	loc := Location{Tags: make(map[string]string)}
	normalized := strings.ReplaceAll(relPath, "\\", "/")

	for _, root := range s.Roots {
		prefix := root + "/"
		if !strings.HasPrefix(normalized, prefix) {
			continue
		}
		rest := strings.TrimPrefix(normalized, prefix)
		parts := strings.Split(rest, "/")

		if len(parts) == 0 {
			continue
		}

		// Check if first part is a layer
		if layer, ok := s.LayerDirs[parts[0]]; ok {
			loc.Layer = layer
			return loc
		}

		// First part is component
		loc.Component = parts[0]

		// Check remaining parts for layer
		for _, p := range parts[1:] {
			if layer, ok := s.LayerDirs[p]; ok {
				loc.Layer = layer
				break
			}
		}
		return loc
	}
	return loc
}

func (s *FlatPkgStrategy) ParseImport(importPath, modulePath string) ImportLocation {
	iloc := ImportLocation{}
	if !strings.HasPrefix(importPath, modulePath) {
		return iloc
	}
	iloc.IsSameModule = true
	relPath := strings.TrimPrefix(importPath, modulePath+"/")
	loc := s.Identify(relPath)
	iloc.Component = loc.Component
	iloc.SubComponent = loc.SubComponent
	iloc.Layer = loc.Layer
	iloc.IsInternal = strings.HasPrefix(relPath, "internal/")
	return iloc
}
