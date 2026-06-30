package core

import "example.com/app/pkg/errs" // root shared foundation -> allowed

type Record struct{ _ errs.Error }
