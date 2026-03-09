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

// IsDomainLevel returns true if this location is at component level without a sub-component.
func (l Location) IsDomainLevel() bool {
	return l.HasComponent() && !l.HasSubComponent()
}

// IsPublicSvc returns true if this location is a Public Service file.
func (l Location) IsPublicSvc() bool {
	return l.Tag("isPublicSvc") == "true"
}

// IsAlias returns true if this location is an alias file (alias.go).
func (l Location) IsAlias() bool {
	return l.Tag("isAlias") == "true"
}

// IsFxCompanion returns true if this component is an FX companion directory.
func (l Location) IsFxCompanion() bool {
	return l.Tag("isFxCompanion") == "true"
}

// IsSaga returns true if this location is in a saga directory.
func (l Location) IsSaga() bool {
	return l.Tag("isSaga") == "true"
}

// --- ImportLocation helper methods ---

// IsDomainLevel returns true if the import targets a domain root (no layer, no sub-component).
func (il ImportLocation) IsDomainLevel() bool {
	return il.IsSameModule && il.Component != "" && il.SubComponent == "" && il.Layer == ""
}

// IsSubdomainLevel returns true if the import targets within a sub-component.
func (il ImportLocation) IsSubdomainLevel() bool {
	return il.IsSameModule && il.SubComponent != ""
}

// IsAppServiceImport returns true if the import targets a domain-level service (app service).
func (il ImportLocation) IsAppServiceImport() bool {
	return il.IsSameModule && il.Component != "" && il.SubComponent == "" &&
		(il.Layer == "service" || il.Layer == "svc" || il.Layer == "appsvc")
}

// IsSagaImport returns true if the import targets a saga package.
func (il ImportLocation) IsSagaImport() bool {
	return il.IsSameModule && il.Layer == "saga"
}

// IsFxCompanion returns true if the import targets an FX companion component.
func (il ImportLocation) IsFxCompanion() bool {
	return il.IsSameModule && strings.HasSuffix(il.Component, "fx")
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

	filename := normalized[strings.LastIndex(normalized, "/")+1:]

	// Check handler roots (api/http, api/jsonrpc)
	for _, hr := range s.HandlerRoots {
		if strings.HasPrefix(normalized, hr+"/") || strings.HasPrefix(normalized, hr+"\\") {
			rest := strings.TrimPrefix(normalized, hr+"/")
			parts := strings.SplitN(rest, "/", 2)
			loc.Layer = "handler"
			loc.Tags["handler_type"] = hr
			loc.Tags["handler_source"] = "api"
			if len(parts) > 0 && !strings.Contains(parts[0], ".") {
				loc.Component = parts[0]
			}
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
		loc.Tags["isSaga"] = "true"
		if strings.HasSuffix(loc.Component, "fx") {
			loc.Tags["isFxCompanion"] = "true"
		}
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
		if filename == "alias.go" {
			loc.Tags["isAlias"] = "true"
		}

		// Check for Public Service file (public.go in svc/service layer)
		if filename == "public.go" && (loc.Layer == "service" || loc.Layer == "svc") {
			loc.Tags["isPublicSvc"] = "true"
		}

		// Check for FX companion directory
		if strings.HasSuffix(loc.Component, "fx") {
			loc.Tags["isFxCompanion"] = "true"
		}

		// Check for internal handler
		if loc.Layer == "handler" {
			loc.Tags["handler_source"] = "internal"
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
