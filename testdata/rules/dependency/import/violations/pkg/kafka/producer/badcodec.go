package producer

import "example.com/bad/pkg/kafka/internal/codec" // WANT-VIOLATION: producer must not import hoisted internal/codec (only consumer is granted it)

var _ = codec.Codec{}
