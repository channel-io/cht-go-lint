package structure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&DomainAliasRequired{})
}

// DomainAliasRequired enforces that domain directories containing subdomains
// must have an alias.go file at the domain root level.
//
// Options:
//
//	domain_root: string - root directory for domains (default: "internal/domain")
type DomainAliasRequired struct{}

func (r *DomainAliasRequired) Meta() lint.Meta {
	return lint.Meta{
		Name:        "structure/domain-alias-required",
		Description: "Domain directories with subdomains must have an alias.go file",
		Category:    "structure",
		Tier:        lint.TierComponentAware,
	}
}

// layerDirs is the set of directory names considered architectural layer dirs.
var layerDirs = map[string]bool{
	"model":    true,
	"repo":     true,
	"svc":      true,
	"service":  true,
	"infra":    true,
	"client":   true,
	"event":    true,
	"handler":  true,
	"consumer": true,
}

func (r *DomainAliasRequired) Check(ctx *lint.Context) error {
	domainRoot := ctx.Options.String("domain_root", "internal/domain")

	domains, err := ctx.Analyzer.ListDirs(domainRoot)
	if err != nil {
		return nil
	}

	root := ctx.Analyzer.Root()

	for _, domain := range domains {
		// Skip FX companion directories
		if strings.HasSuffix(domain, "fx") {
			continue
		}

		children, err := ctx.Analyzer.ListDirs(domainRoot + "/" + domain)
		if err != nil {
			continue
		}

		// Check if any child is a subdomain (not a layer dir and not ending with "fx")
		hasSubdomain := false
		for _, child := range children {
			if strings.HasSuffix(child, "fx") {
				continue
			}
			if layerDirs[child] {
				continue
			}
			// This child is a subdomain
			hasSubdomain = true
			break
		}

		if !hasSubdomain {
			continue
		}

		// Check if alias.go exists
		aliasPath := filepath.Join(root, domainRoot, domain, "alias.go")
		if _, err := os.Stat(aliasPath); os.IsNotExist(err) {
			relDir := filepath.ToSlash(filepath.Join(domainRoot, domain))
			ctx.Report.Add(lint.Violation{
				Rule:     "structure/domain-alias-required",
				Severity: ctx.Severity,
				File:     relDir,
				Message:  fmt.Sprintf("domain %q has subdomains but is missing alias.go", domain),
				Expected: "alias.go",
			})
		}
	}
	return nil
}
