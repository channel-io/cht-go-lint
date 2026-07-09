package svc

import appmodel "example.com/combo/internal/domain/app/model" // WANT-VIOLATION: order must not import sibling domain app (domain isolation)

var _ = appmodel.Account{}
