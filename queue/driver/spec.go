package driver

// Spec is the uniform input to every queue driver constructor.
type Spec struct {
	Name string

	// DefaultQueue is used when Push/Pop receive an empty queue name.
	DefaultQueue string

	// Memory driver
	Workers   int
	QueueSize int

	// SQS driver
	AWSRegion string
	QueueURL  string

	// Kafka driver
	Brokers []string
	Topic   string
	GroupID string

	// NATS driver
	NATSURL     string
	Subject     string
	StreamName  string

	Extra map[string]string
}
