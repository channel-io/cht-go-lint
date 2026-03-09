package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
	"github.com/channel-io/cht-go-lint/formatter"
	_ "github.com/channel-io/cht-go-lint/preset"
	_ "github.com/channel-io/cht-go-lint/preset/channeltalk"
	_ "github.com/channel-io/cht-go-lint/rules"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "check":
		cmdCheck(os.Args[2:])
	case "list-rules":
		cmdListRules()
	case "init":
		cmdInit()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdCheck(args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	configPath := fs.String("config", "", "config file path (default: auto-detect)")
	formatFlag := fs.String("format", "text", "output format: text, json, github")
	ruleFilter := fs.String("rule", "", "run specific rule(s) (comma-separated)")
	_ = fs.Parse(args)

	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	root, _ = filepath.Abs(root)

	var cfg *lint.Config
	var err error
	if *configPath != "" {
		cfg, err = lint.LoadConfigFrom(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
			os.Exit(1)
		}
		cfg.Root = root
	} else {
		cfg, err = lint.LoadConfig(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
			os.Exit(1)
		}
	}

	// Filter rules if specified
	if *ruleFilter != "" {
		names := strings.Split(*ruleFilter, ",")
		filtered := make(map[string]lint.RuleConfig)
		for _, name := range names {
			name = strings.TrimSpace(name)
			if rc, ok := cfg.Rules[name]; ok {
				filtered[name] = rc
			} else {
				filtered[name] = lint.RuleConfig{Severity: lint.Error}
			}
		}
		cfg.Rules = filtered
	}

	report := lint.Check(cfg)

	// Format output
	var f formatter.Formatter
	switch *formatFlag {
	case "json":
		f = formatter.JSON{Pretty: true}
	case "github":
		f = formatter.GitHub{}
	default:
		f = formatter.Text{}
	}

	fmt.Print(f.Format(report.Violations()))

	if report.HasErrors() {
		os.Exit(1)
	}
}

func cmdListRules() {
	fmt.Println("Available rules:")
	fmt.Println()

	rules := lint.All()
	category := ""
	for _, r := range rules {
		meta := r.Meta()
		if meta.Category != category {
			category = meta.Category
			fmt.Printf("  %s/\n", category)
		}
		tierLabel := ""
		switch meta.Tier {
		case lint.TierUniversal:
			tierLabel = "universal"
		case lint.TierLayerAware:
			tierLabel = "layer-aware"
		case lint.TierComponentAware:
			tierLabel = "component-aware"
		case lint.TierDomainSpecific:
			tierLabel = "domain-specific"
		}
		fmt.Printf("    %-40s [%s] %s\n", meta.Name, tierLabel, meta.Description)
	}
}

func cmdInit() {
	configContent := `# cht-go-lint configuration
# See: https://github.com/channel-io/cht-go-lint

module: github.com/your-org/your-project

# Location strategy: "nested-domain" or "flat-pkg"
# location:
#   strategy: flat-pkg

# Define architectural layers and their allowed imports
# layers:
#   - name: model
#     may_import: []
#   - name: repo
#     may_import: [model]
#   - name: service
#     aliases: [svc]
#     may_import: [model, repo]
#   - name: handler
#     may_import: [model, service]

# Enable rules (all rules are off by default)
rules:
  # Tier 0: Universal (no config needed)
  naming/file-naming: warn
  naming/no-stutter: warn
  structure/forbidden-dirs: warn

  # Tier 1: Layer-aware (requires layers config)
  # dependency/layer-direction: error

  # Tier 2: Component-aware (requires components config)
  # dependency/module-isolation: error

  # Tier 3: Domain-specific
  # ddd/aggregate-boundary:
  #   severity: error
  #   options:
  #     root_marker: "Aggregate"
`

	path := ".cht-go-lint.yaml"
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "%s already exists\n", path)
		os.Exit(1)
	}

	if err := os.WriteFile(path, []byte(configContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created %s\n", path)
	fmt.Println("Edit the file to configure rules for your project.")
}

func printUsage() {
	fmt.Println(`Usage: cht-go-lint <command> [options]

Commands:
  check       Run architecture lint checks
  list-rules  List all available rules
  init        Create a default configuration file

Options for 'check':
  --config <path>    Config file path (default: auto-detect .cht-go-lint.yaml)
  --format <fmt>     Output format: text, json, github (default: text)
  --rule <names>     Run specific rules (comma-separated)`)
}
