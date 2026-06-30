package publicsvc

import "example.com/svc/internal/domain/order/model" // shared -> allowed

func Find() model.Order { return model.Order{} }
