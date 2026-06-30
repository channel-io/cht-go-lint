package engine

import "example.com/bad/pkg/kafka/consumer/handler" // WANT-VIOLATION: engine must not import sibling handler

var _ = handler
