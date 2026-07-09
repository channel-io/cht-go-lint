package consumer

import "example.com/sib/pkg/kafka/producer" // consumer -> producer (may_import) — allowed

var _ = producer.Publisher{}
