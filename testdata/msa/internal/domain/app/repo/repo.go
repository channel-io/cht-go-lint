package repo

import "example.com/svc/internal/domain/app/model" // shared layer -> allowed

func Load() model.Account { return model.Account{} }
