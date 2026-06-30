package svc

import (
	"example.com/svc/internal/domain/app/model"
	"example.com/svc/internal/domain/app/repo"
)

func Do() model.Account { return repo.Load() }
