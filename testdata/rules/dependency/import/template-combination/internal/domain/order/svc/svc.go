package svc

import "example.com/combo/internal/domain/order/model" // same-domain svc -> model — allowed

func Do() model.Order { return model.Order{} }
