package driver

type Spec struct {
	Name         string
	SubjectPrefix string
	NATSURL      string
	StreamName   string
	KafkaBrokers []string
	KafkaTopic   string
	Extra        map[string]string
}
