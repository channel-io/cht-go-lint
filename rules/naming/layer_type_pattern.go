package naming

import (
	"fmt"
	"path/filepath"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&LayerTypePattern{})
}

// LayerTypePattern enforces type naming conventions per layer/tag.
// This is a generic rule that can replace layer-specific rules like
// public-service-v2 and saga-naming.
//
// Options:
//
//	patterns: list of pattern objects, each with:
//	  tag: string - location tag to filter on (e.g., "isPublicSvc")
//	  tag_value: string - expected tag value (default: "true")
//	  filename_contains: string - additional filename filter
//	  skip_tags: map[string]string - skip files matching these tags
//	  required_interface: string - exact interface name required
//	  required_interface_suffix: string - interface name must end with this
//	  required_struct: string - exact private struct name required
//	  required_struct_match: string - "exact" or "case_insensitive" match against interface name
//	  no_impl_suffix: bool - forbid "Impl" suffix on struct name
//	  constructor_returns_interface: bool - constructors must return the interface type
type LayerTypePattern struct{}

func (r *LayerTypePattern) Meta() lint.Meta {
	return lint.Meta{
		Name:        "naming/layer-type-pattern",
		Description: "Enforce type naming conventions per layer/tag",
		Category:    "naming",
		Tier:        lint.TierLayerAware,
	}
}

func (r *LayerTypePattern) Check(ctx *lint.Context) error {
	patterns := ctx.Options.MapSlice("patterns")
	if len(patterns) == 0 {
		return nil
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		for _, p := range patterns {
			if err := checkLayerPattern(ctx, file, p); err != nil {
				return err
			}
		}
		return nil
	})
}

func checkLayerPattern(ctx *lint.Context, file *lint.ParsedFile, pattern map[string]any) error {
	// Filter by tag
	tag, _ := pattern["tag"].(string)
	if tag == "" {
		return nil
	}
	tagValue, _ := pattern["tag_value"].(string)
	if tagValue == "" {
		tagValue = "true"
	}
	if file.Location.Tag(tag) != tagValue {
		return nil
	}

	// Skip files matching skip_tags
	if skipTags, ok := pattern["skip_tags"].(map[string]any); ok {
		for k, v := range skipTags {
			if sv, ok := v.(string); ok && file.Location.Tag(k) == sv {
				return nil
			}
		}
	}

	// Optional filename filter
	if fnContains, ok := pattern["filename_contains"].(string); ok && fnContains != "" {
		base := filepath.Base(file.RelPath)
		if !strings.Contains(base, fnContains) {
			return nil
		}
	}

	ruleName := "naming/layer-type-pattern"

	// Check required_interface (exact name)
	requiredIface, _ := pattern["required_interface"].(string)
	ifaceSuffix, _ := pattern["required_interface_suffix"].(string)

	var foundIfaceName string
	for _, td := range file.Types {
		if !td.IsInterface || !td.Exported {
			continue
		}
		if requiredIface != "" && td.Name == requiredIface {
			foundIfaceName = td.Name
			break
		}
		if ifaceSuffix != "" && strings.HasSuffix(td.Name, ifaceSuffix) {
			foundIfaceName = td.Name
			break
		}
	}

	// Check for old-pattern violations (e.g., Public as struct instead of interface)
	if requiredIface != "" {
		for _, td := range file.Types {
			if td.IsStruct && td.Exported && td.Name == requiredIface && foundIfaceName == "" {
				ctx.Report.Add(lint.Violation{
					Rule:     ruleName,
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     td.Pos.Line,
					Message:  fmt.Sprintf("%s is a struct, must be interface", requiredIface),
					Found:    fmt.Sprintf("type %s struct", requiredIface),
					Expected: fmt.Sprintf("type %s interface", requiredIface),
				})
				return nil
			}
		}
	}

	if requiredIface != "" && foundIfaceName == "" {
		ctx.Report.Add(lint.Violation{
			Rule:     ruleName,
			Severity: ctx.Severity,
			File:     file.RelPath,
			Line:     1,
			Message:  fmt.Sprintf("file must define 'type %s interface'", requiredIface),
			Expected: fmt.Sprintf("type %s interface", requiredIface),
		})
	}
	if ifaceSuffix != "" && foundIfaceName == "" {
		ctx.Report.Add(lint.Violation{
			Rule:     ruleName,
			Severity: ctx.Severity,
			File:     file.RelPath,
			Line:     1,
			Message:  fmt.Sprintf("file must define an interface ending with %q", ifaceSuffix),
		})
		return nil
	}

	// Check required_struct
	requiredStruct, _ := pattern["required_struct"].(string)
	structMatch, _ := pattern["required_struct_match"].(string)

	if requiredStruct != "" || structMatch != "" {
		hasStruct := false
		for _, td := range file.Types {
			if !td.IsStruct || td.Exported {
				continue
			}
			if requiredStruct != "" && td.Name == requiredStruct {
				hasStruct = true
				break
			}
			if structMatch == "case_insensitive" && foundIfaceName != "" {
				if strings.EqualFold(td.Name, foundIfaceName) {
					hasStruct = true
					break
				}
			}
		}
		if !hasStruct && foundIfaceName != "" {
			expected := requiredStruct
			if expected == "" && foundIfaceName != "" {
				expected = strings.ToLower(foundIfaceName[:1]) + foundIfaceName[1:]
			}
			ctx.Report.Add(lint.Violation{
				Rule:     ruleName,
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     1,
				Message:  fmt.Sprintf("missing private struct for interface %q", foundIfaceName),
				Expected: expected,
			})
		}
	}

	// Check no_impl_suffix
	noImplSuffix, _ := pattern["no_impl_suffix"].(bool)
	if noImplSuffix && requiredStruct != "" {
		implName := requiredStruct + "Impl"
		for _, td := range file.Types {
			if td.IsStruct && !td.Exported && td.Name == implName {
				ctx.Report.Add(lint.Violation{
					Rule:     ruleName,
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     td.Pos.Line,
					Message:  fmt.Sprintf("%s uses Impl suffix, use 'type %s struct'", implName, requiredStruct),
					Found:    fmt.Sprintf("type %s struct", implName),
					Expected: fmt.Sprintf("type %s struct", requiredStruct),
				})
			}
		}
	}

	// Check constructor_returns_interface
	ctorReturnsIface, _ := pattern["constructor_returns_interface"].(bool)
	if ctorReturnsIface && foundIfaceName != "" {
		for _, fd := range file.Funcs {
			if !fd.IsConstructor || fd.ReceiverType != "" {
				continue
			}
			for _, rt := range fd.ReturnTypes {
				if strings.HasPrefix(rt, "*") {
					ctx.Report.Add(lint.Violation{
						Rule:     ruleName,
						Severity: ctx.Severity,
						File:     file.RelPath,
						Line:     fd.Pos.Line,
						Message:  fmt.Sprintf("constructor %q must return the interface, not a pointer to struct", fd.Name),
						Found:    rt,
						Expected: foundIfaceName,
					})
				}
			}
		}
	}

	return nil
}
