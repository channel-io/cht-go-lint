package model

import "example.com/combo/internal/domain/order/svc" // WANT-VIOLATION: model must not import up to svc (layer direction)

var _ = svc.Do
