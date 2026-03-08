package naming

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&ImplNaming{})
}

// ImplNaming checks that implementation structs follow naming convention relative to their interface.
type ImplNaming struct{}

func (r *ImplNaming) Meta() lint.Meta {
	return lint.Meta{
		Name:        "naming/impl-naming",
		Description: "Implementation structs should follow naming convention relative to their interface",
		Category:    "naming",
		Tier:        lint.TierUniversal,
	}
}

func (r *ImplNaming) Check(ctx *lint.Context) error {
	pattern := ctx.Options.String("pattern", "lowercase")

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		var interfaces []string
		var structs []lint.TypeDecl

		for _, td := range file.Types {
			if td.IsInterface && td.Exported {
				interfaces = append(interfaces, td.Name)
			}
			if td.IsStruct && !td.Exported {
				structs = append(structs, td)
			}
		}

		if len(interfaces) == 0 || len(structs) == 0 {
			return nil
		}

		for _, ifaceName := range interfaces {
			expectedName := expectedImplName(ifaceName, pattern)

			for _, st := range structs {
				if couldBeImpl(st.Name, ifaceName, pattern) && st.Name != expectedName {
					ctx.Report.Add(lint.Violation{
						Rule:     "naming/impl-naming",
						Severity: ctx.Severity,
						File:     file.RelPath,
						Line:     st.Pos.Line,
						Message:  fmt.Sprintf("implementation struct %q does not follow %s convention for interface %q", st.Name, pattern, ifaceName),
						Found:    st.Name,
						Expected: expectedName,
					})
				}
			}
		}

		return nil
	})
}

func expectedImplName(ifaceName, pattern string) string {
	switch pattern {
	case "Impl":
		return ifaceName + "Impl"
	default: // "lowercase"
		return strings.ToLower(ifaceName[:1]) + ifaceName[1:]
	}
}

func couldBeImpl(structName, ifaceName, pattern string) bool {
	lower := strings.ToLower(ifaceName[:1]) + ifaceName[1:]
	switch pattern {
	case "Impl":
		return structName == lower || structName == lower+"Impl" ||
			structName == ifaceName+"Impl"
	default: // "lowercase"
		return structName == lower || structName == lower+"Impl" ||
			structName == ifaceName+"Impl"
	}
}
