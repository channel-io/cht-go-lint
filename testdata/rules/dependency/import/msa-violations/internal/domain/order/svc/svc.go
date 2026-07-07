package svc

import "example.com/badsvc/internal/domain/order/model" // svc -> model (same domain, allowed)

func Do() model.Order { return model.Order{} }
