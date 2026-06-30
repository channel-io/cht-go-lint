package handler

import (
	apppub "example.com/svc/internal/domain/app/publicsvc"
	"example.com/svc/internal/domain/app/svc"
	orderpub "example.com/svc/internal/domain/order/publicsvc"
)

var _ = svc.Do
var _ = apppub.Of
var _ = orderpub.Find
