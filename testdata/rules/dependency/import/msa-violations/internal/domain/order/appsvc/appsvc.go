package appsvc

import (
	"example.com/badsvc/internal/domain/order/svc"   // same domain, appsvc -> svc (may_import) — allowed
	"example.com/badsvc/internal/domain/order/model" // same domain, model shared — allowed

	appmodel "example.com/badsvc/internal/domain/app/model" // WANT-VIOLATION: order must not import sibling domain app (domain isolation)
)

var (
	_ = svc.Do
	_ = model.Order{}
	_ = appmodel.Account{}
)
