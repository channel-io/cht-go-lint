package model

import "example.com/layer/internal/domain/order/svc" // WANT-VIOLATION: model must not import up to svc

var _ = svc.Do
