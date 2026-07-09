package consumer

import "example.com/shared/pkg/kafka/producer" // WANT-VIOLATION: producer is not shared and consumer has no edge

var _ = producer.Publisher{}
