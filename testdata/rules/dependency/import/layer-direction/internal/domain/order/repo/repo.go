package repo

import "example.com/layer/internal/domain/order/model" // repo -> model — allowed

func Load() model.T { return model.T{} }
