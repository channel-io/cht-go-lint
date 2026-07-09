package svc

import (
	"example.com/layer/internal/domain/order/model" // svc -> model — allowed
	"example.com/layer/internal/domain/order/repo"   // svc -> repo — allowed
)

func Do() model.T { return repo.Load() }
