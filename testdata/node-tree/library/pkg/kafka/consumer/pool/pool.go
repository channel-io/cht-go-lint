package pool

import "example.com/app/pkg/kafka/consumer/decode" // pool is consumer's code; decode shared in consumer -> allowed

type Pool struct{ _ decode.Decoder }
