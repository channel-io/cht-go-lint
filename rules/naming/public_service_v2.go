package naming

import (
	"fmt"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&PublicServiceV2{})
}

// PublicServiceV2 checks that public.go files in the svc layer follow the v2 pattern:
// type Public interface + type public struct (not publicImpl).
type PublicServiceV2 struct{}

func (r *PublicServiceV2) Meta() lint.Meta {
	return lint.Meta{
		Name:        "naming/public-service-v2",
		Description: "Public service files must define 'type Public interface' and 'type public struct'",
		Category:    "naming",
		Tier:        lint.TierLayerAware,
	}
}

func (r *PublicServiceV2) Check(ctx *lint.Context) error {
	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		if file.Location.Tag("isPublicSvc") != "true" {
			return nil
		}

		var hasPublicInterface bool
		var hasPublicStruct bool
		var hasPublicImplStruct bool
		var publicStructIsExported bool

		for _, td := range file.Types {
			if td.IsInterface && td.Exported && td.Name == "Public" {
				hasPublicInterface = true
			}
			if td.IsStruct && td.Name == "public" && !td.Exported {
				hasPublicStruct = true
			}
			if td.IsStruct && td.Name == "publicImpl" && !td.Exported {
				hasPublicImplStruct = true
			}
			if td.IsStruct && td.Name == "Public" && td.Exported {
				publicStructIsExported = true
			}
		}

		if !hasPublicInterface && publicStructIsExported {
			ctx.Report.Add(lint.Violation{
				Rule:     "naming/public-service-v2",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     1,
				Message:  "Public is a struct (v1 pattern), must be interface",
				Found:    "type Public struct",
				Expected: "type Public interface",
			})
			return nil
		}

		if !hasPublicInterface {
			ctx.Report.Add(lint.Violation{
				Rule:     "naming/public-service-v2",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     1,
				Message:  "public.go must define 'type Public interface'",
				Expected: "type Public interface",
			})
		}

		if hasPublicImplStruct {
			ctx.Report.Add(lint.Violation{
				Rule:     "naming/public-service-v2",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     1,
				Message:  fmt.Sprintf("publicImpl uses Impl suffix, use 'type public struct'"),
				Found:    "type publicImpl struct",
				Expected: "type public struct",
			})
		}

		if hasPublicInterface && !hasPublicStruct {
			ctx.Report.Add(lint.Violation{
				Rule:     "naming/public-service-v2",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     1,
				Message:  "Public interface found but no private 'public' struct",
				Found:    "type Public interface",
				Expected: "type Public interface + type public struct",
			})
		}

		return nil
	})
}
