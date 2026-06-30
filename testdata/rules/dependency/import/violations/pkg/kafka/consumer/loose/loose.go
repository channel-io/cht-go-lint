package loose

import "example.com/bad/pkg/kafka/consumer/handler" // WANT-VIOLATION: undeclared sibling loose must not import handler

var _ = handler
