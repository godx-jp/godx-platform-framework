package driver

// Spec is the uniform input to every cipher constructor. Most
// drivers carry no configuration beyond Name today; Extra is the
// escape hatch for future tunables.
type Spec struct {
	Name  string
	Extra map[string]string
}
