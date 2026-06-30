package producer

import "example.com/app/pkg/kafka/core" // sibling, shared -> allowed

type Publisher struct{ _ core.Record }
