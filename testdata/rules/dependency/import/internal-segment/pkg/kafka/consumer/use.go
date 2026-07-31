package consumer

import "example.com/hoist/pkg/kafka/internal/codec" // consumer granted internal/codec — allowed

var _ = codec.Codec{}
