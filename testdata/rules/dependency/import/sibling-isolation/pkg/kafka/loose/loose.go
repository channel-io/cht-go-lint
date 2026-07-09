package loose

import "example.com/sib/pkg/kafka/producer" // WANT-VIOLATION: undeclared sibling loose has no edge to producer

var _ = producer.Publisher{}
