package codec

// Codec is a kafka-internal helper. Go hides it outside pkg/kafka; the node tree
// hoists it to a deny-default sibling of consumer/producer, so only the nodes
// kafka grants (consumer) may import it.
type Codec struct{}
