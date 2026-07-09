package consumer

import "example.com/hoist/pkg/kafka/internal/codec" // consumer granted codec — allowed

var _ = codec.Codec{}
