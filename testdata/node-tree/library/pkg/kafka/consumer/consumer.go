package consumer

import (
	"example.com/app/pkg/errs"              // root shared -> allowed
	"example.com/app/pkg/kafka/core"        // sibling shared -> allowed
	"example.com/app/pkg/kafka/producer"    // sibling, may_import -> allowed
)

type Subscriber struct {
	_ errs.Error
	_ core.Record
	_ producer.Publisher
}
