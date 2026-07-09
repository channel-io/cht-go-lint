package svc

import "example.com/xdom/internal/domain/order/publicsvc" // app -> order (granted via may_import) — allowed

var _ = publicsvc.Find
