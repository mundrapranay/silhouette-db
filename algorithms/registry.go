package algorithms

import (
	"fmt"

	"github.com/mundrapranay/silhouette-db/algorithms/common"
	"github.com/mundrapranay/silhouette-db/algorithms/exact"
	"github.com/mundrapranay/silhouette-db/algorithms/ledp"
)

// GetAlgorithm retrieves an algorithm by type and name
func GetAlgorithm(algorithmType common.AlgorithmType, name string) (common.GraphAlgorithm, error) {
	switch algorithmType {
	case common.AlgorithmTypeExact:
		return exact.Get(name)
	case common.AlgorithmTypeLEDP:
		return ledp.Get(name)
	default:
		return nil, fmt.Errorf("unknown algorithm type: %s", algorithmType)
	}
}

// ListAlgorithms returns all registered algorithms by type
func ListAlgorithms(algorithmType common.AlgorithmType) []string {
	switch algorithmType {
	case common.AlgorithmTypeExact:
		return exact.List()
	case common.AlgorithmTypeLEDP:
		return ledp.List()
	default:
		return []string{}
	}
}

// ListAllAlgorithms returns all registered algorithms for both types
func ListAllAlgorithms() (exactAlgs []string, ledpAlgs []string) {
	return exact.List(), ledp.List()
}
