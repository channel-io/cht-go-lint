package consumer

import "example.com/app/pkg/kafka/internal/codec"

// consumer is granted the hoisted internal/codec node via may_import, so this is
// clean. producer, which is not granted, would be a violation (see violations/).
var _ = codec.Codec{}
