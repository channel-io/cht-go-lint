package sqlrepo

import "example.com/shared/pkg/errs" // sqlrepo -> errs (root shared) — allowed

var _ = errs.Error{}
