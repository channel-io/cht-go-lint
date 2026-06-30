package appsvc

import (
	"example.com/svc/internal/domain/app/infra"
	"example.com/svc/internal/domain/app/repo"
	"example.com/svc/internal/domain/app/svc"
)

var X = 1
var (
	_ = svc.Do
	_ = repo.Load
	_ = infra.Touch
)
