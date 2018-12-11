package inmemorygenerator

// InMemoryGenerator represents a secret generator that generates everything
// by itself, using no 3rd party tools
type InMemoryGenerator struct{}

// NewInMemoryGenerator creates an InMemoryGenerator
func NewInMemoryGenerator() *InMemoryGenerator {
	return &InMemoryGenerator{}
}
