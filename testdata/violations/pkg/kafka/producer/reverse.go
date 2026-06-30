package producer

import "example.com/bad/pkg/kafka/consumer" // WANT-VIOLATION: producer must not import sibling consumer

var _ = consumer.X
