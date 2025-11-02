package ledp

import (
	"fmt"
	"sync"

	"github.com/mundrapranay/silhouette-db/algorithms/common"
)

var (
	algorithmsMu sync.RWMutex
	algorithms   = make(map[string]func() common.GraphAlgorithm)
)

// Register registers an LEDP algorithm implementation
func Register(name string, constructor func() common.GraphAlgorithm) {
	algorithmsMu.Lock()
	defer algorithmsMu.Unlock()

	if constructor == nil {
		panic(fmt.Sprintf("algorithm constructor for %s is nil", name))
	}

	if _, exists := algorithms[name]; exists {
		panic(fmt.Sprintf("algorithm %s is already registered", name))
	}

	algorithms[name] = constructor
}

// Get returns a registered algorithm by name
func Get(name string) (common.GraphAlgorithm, error) {
	algorithmsMu.RLock()
	defer algorithmsMu.RUnlock()

	constructor, exists := algorithms[name]
	if !exists {
		return nil, fmt.Errorf("algorithm %s not found", name)
	}

	return constructor(), nil
}

// List returns all registered algorithm names
func List() []string {
	algorithmsMu.RLock()
	defer algorithmsMu.RUnlock()

	names := make([]string, 0, len(algorithms))
	for name := range algorithms {
		names = append(names, name)
	}

	return names
}

