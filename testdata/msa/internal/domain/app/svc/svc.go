package svc

import (
	"example.com/svc/internal/domain/app/model"     // shared -> allowed
	"example.com/svc/internal/domain/app/repo"       // may_import -> allowed
	"example.com/svc/internal/domain/order/publicsvc" // app->order allowed
)

func Do() model.Account { _ = publicsvc.Find; return repo.Load() }
