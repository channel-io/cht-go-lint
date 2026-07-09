package core

import "example.com/shared/pkg/errs" // core -> errs (root shared) — allowed

type Record struct{ _ errs.Error }
