package handler

import (
	"example.com/svc/internal/domain/app/model" // shared -> allowed
	"example.com/svc/internal/domain/app/svc"    // may_import -> allowed
)

func H() model.Account { return svc.Do() }
