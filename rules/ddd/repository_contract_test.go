package ddd

import (
	"testing"

	lint "github.com/channel-io/cht-go-lint"
	"github.com/channel-io/cht-go-lint/testutil"
)

func TestRepositoryMethodContract(t *testing.T) {
	cfg := repositoryFixture(t, map[string]string{
		"internal/domain/catalog/repo/widget.go": `package repo

import "context"

type Widget interface {
	Get(ctx context.Context, id string) (*widgetModel, error)
	Save(ctx context.Context, widget *widgetModel) (*widgetModel, error)
	Find(ctx context.Context, id string) (*widgetModel, error)
}

type widget struct { db database }
type widgetModel struct{}
type database interface {
	Fetch(context.Context, string) (*widgetModel, error)
}

func (w *widget) Find(ctx context.Context, id string) (*widgetModel, error) {
	return w.db.Fetch(ctx, id)
}
`,
	})
	cfg.Rules[repositoryMethodContractRule] = lint.RuleConfig{
		Severity: lint.Error,
		Options: map[string]any{
			"discouraged_mutation_prefixes": []any{"Save", "Set", "Mark"},
		},
	}

	report := testutil.RunRule(t, &RepositoryMethodContract{}, cfg)
	if got := report.ErrorCount(); got != 3 {
		t.Fatalf("errors: got %d, want 3\n%s", got, report.String())
	}
}

func TestRepositoryMethodContractExactExcludesAndRejectsStaleEntries(t *testing.T) {
	cfg := repositoryFixture(t, map[string]string{
		"internal/domain/catalog/repo/widget.go": `package repo

import "context"

type Widget interface {
	Get(ctx context.Context, id string) error
	GetNew(ctx context.Context, id string) error
}
`,
	})
	cfg.Rules[repositoryMethodContractRule] = lint.RuleConfig{
		Severity: lint.Error,
		Options: map[string]any{
			"reject_unused_excludes": true,
			"exclude_methods": []any{
				map[string]any{
					"path":    "internal/domain/catalog/repo/widget.go",
					"symbols": []any{"Widget.Get", "Widget.List"},
					"reason":  "legacy contract; List is intentionally stale",
				},
			},
		},
	}

	report := testutil.RunRule(t, &RepositoryMethodContract{}, cfg)
	if got := report.ErrorCount(); got != 2 {
		t.Fatalf("errors: got %d, want one unexcluded method and one stale exclusion\n%s", got, report.String())
	}
}

func TestRepositoryMethodContractFlagsSpecializedUpdateButAllowsUpdate(t *testing.T) {
	cfg := repositoryFixture(t, map[string]string{
		"internal/domain/catalog/repo/widget.go": `package repo

type Widget interface {
	Update() error
	UpdateTitleIfEmpty() error
}
`,
	})
	cfg.Rules[repositoryMethodContractRule] = lint.RuleConfig{
		Severity: lint.Error,
		Options: map[string]any{
			"discouraged_mutation_prefixes": []any{"Update"},
		},
	}

	report := testutil.RunRule(t, &RepositoryMethodContract{}, cfg)
	if got := report.ErrorCount(); got != 1 {
		t.Fatalf("errors: got %d, want 1 specialized update\n%s", got, report.String())
	}
}

func TestRepositoryReadModifyWrite(t *testing.T) {
	cfg := repositoryFixture(t, map[string]string{
		"internal/domain/catalog/repo/widget.go": `package repo

import "context"

type widget struct { db database }
type widgetWithSplitStorage struct { reader database; writer database }
type widgetModel struct{}
type database interface {
	Find(context.Context, string) (*widgetModel, error)
	FetchForUpdate(context.Context, string) (*widgetModel, error)
	Update(context.Context, *widgetModel) (*widgetModel, error)
	XLock(context.Context) error
}

func (w *widget) Unsafe(ctx context.Context, id string) (*widgetModel, error) {
	item, err := w.db.Find(ctx, id)
	if err != nil { return nil, err }
	return w.db.Update(ctx, item)
}

func (w *widget) Locked(ctx context.Context, id string) (*widgetModel, error) {
	item, err := w.db.FetchForUpdate(ctx, id)
	if err != nil { return nil, err }
	return w.db.Update(ctx, item)
}

func (w *widget) GuardAfterWrite(ctx context.Context, id string) (*widgetModel, error) {
	item, err := w.db.Find(ctx, id)
	if err != nil { return nil, err }
	updated, err := w.db.Update(ctx, item)
	if err != nil { return nil, err }
	if err := w.db.XLock(ctx); err != nil { return nil, err }
	return updated, nil
}

func (w *widgetWithSplitStorage) DifferentRows(ctx context.Context, id string) (*widgetModel, error) {
	item, err := w.reader.Find(ctx, id)
	if err != nil { return nil, err }
	return w.writer.Update(ctx, item)
}
`,
	})
	cfg.Rules[repositoryReadModifyWriteRule] = lint.RuleConfig{Severity: lint.Error}

	report := testutil.RunRule(t, &RepositoryReadModifyWrite{}, cfg)
	if got := report.ErrorCount(); got != 2 {
		t.Fatalf("errors: got %d, want unsafe and too-late guard\n%s", got, report.String())
	}
}

func TestRepositoryErrorContract(t *testing.T) {
	cfg := repositoryFixture(t, map[string]string{
		"internal/domain/catalog/repo/widget.go": `package repo

import (
	"context"
	"github.com/pkg/errors"
)

type widget struct{}

func (w *widget) Find(context.Context, string) (*widget, error) {
	var err error
	if err != nil { return nil, err }
	return nil, errors.Wrap(err, "find widget")
}
`,
	})
	cfg.Rules[repositoryErrorContractRule] = lint.RuleConfig{Severity: lint.Error}

	report := testutil.RunRule(t, &RepositoryErrorContract{}, cfg)
	if got := report.ErrorCount(); got != 2 {
		t.Fatalf("errors: got %d, want 2\n%s", got, report.String())
	}
}

func TestRepositoryErrorContractDetectsSentinelsAndUnwrappedAPIErrorCauses(t *testing.T) {
	cfg := repositoryFixture(t, map[string]string{
		"internal/domain/catalog/repo/widget.go": `package repo

import (
	"context"
	stderrors "errors"
	"example.com/apierr"
	liberrors "github.com/channel-io/go-lib/pkg/errors"
)

var ErrWidgetNotFound = stderrors.New("widget not found")

type widget struct{}

func (w *widget) Unwrapped(context.Context) error {
	var err error
	return apierr.NotFound(err)
}

func (w *widget) Wrapped(context.Context) error {
	var err error
	return apierr.NotFound(liberrors.Wrap(err, "find widget"))
}

func (w *widget) Unclassified(context.Context) error {
	return stderrors.New("widget not found")
}
`,
	})
	cfg.Rules[repositoryErrorContractRule] = lint.RuleConfig{Severity: lint.Error}

	report := testutil.RunRule(t, &RepositoryErrorContract{}, cfg)
	if got := report.ErrorCount(); got != 3 {
		t.Fatalf("errors: got %d, want sentinel, unwrapped apierr cause, and unclassified error\n%s", got, report.String())
	}
}

func TestRepositoryErrorContractExactImportExcludesAndRejectsStaleEntries(t *testing.T) {
	cfg := repositoryFixture(t, map[string]string{
		"internal/domain/catalog/repo/widget.go": `package repo

import "github.com/pkg/errors"

var _ = errors.New
`,
	})
	cfg.Rules[repositoryErrorContractRule] = lint.RuleConfig{
		Severity: lint.Error,
		Options: map[string]any{
			"reject_unused_excludes": true,
			"exclude_imports": []any{
				map[string]any{
					"path":    "internal/domain/catalog/repo/widget.go",
					"imports": []any{"github.com/pkg/errors", "github.com/friendsofgo/errors"},
					"reason":  "legacy package; friendsofgo entry is intentionally stale",
				},
			},
		},
	}

	report := testutil.RunRule(t, &RepositoryErrorContract{}, cfg)
	if got := report.ErrorCount(); got != 1 {
		t.Fatalf("errors: got %d, want 1 stale import exclusion\n%s", got, report.String())
	}
}

func repositoryFixture(t *testing.T, files map[string]string) *lint.Config {
	t.Helper()
	cfg := testutil.CreateFixture(t, "example.com/repository-contract", files)
	cfg.Layers = []lint.LayerConfig{
		{Name: "model"},
		{Name: "repo", MayImport: []string{"model"}},
		{Name: "service", Aliases: []string{"svc"}, MayImport: []string{"model", "repo"}},
	}
	cfg.Location = &lint.LocationConfig{
		Strategy: "nested-domain",
		Options: map[string]any{
			"domain_root":   "internal/domain",
			"subdomain_dir": "subdomain",
		},
	}
	return cfg
}
