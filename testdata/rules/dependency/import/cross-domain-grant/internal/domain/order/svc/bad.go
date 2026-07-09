package svc

import appmodel "example.com/xdom/internal/domain/app/model" // WANT-VIOLATION: order is not granted app (reverse of the app->order edge)

var _ = appmodel.Account{}
