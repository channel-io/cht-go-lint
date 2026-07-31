package producer

import "example.com/hoist/pkg/kafka/internal/codec" // WANT-VIOLATION: producer is not granted internal/codec

var _ = codec.Codec{}
