package inmemorygenerator

// InMemoryGenerator represents a secret generator that generates everything
// by itself, using no 3rd party tools
type InMemoryGenerator struct {
	Bits      int    // Key bits
	Expiry    int    // Expiration (days)
	Algorithm string // Algorithm type
}

// NewInMemoryGenerator creates a default InMemoryGenerator
func NewInMemoryGenerator() *InMemoryGenerator {
	return &InMemoryGenerator{Bits: 4096, Expiry: 365, Algorithm: "rsa"}
}
