package handler

import "example.com/app/pkg/kafka/consumer/decode" // sibling shared within consumer -> allowed

type H struct{ _ decode.Decoder }
