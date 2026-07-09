package consumer

import (
	"example.com/shared/pkg/errs"       // root shared — allowed
	"example.com/shared/pkg/kafka/core"  // kafka-local shared — allowed
)

type Subscriber struct {
	_ errs.Error
	_ core.Record
}
