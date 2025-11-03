package common

import (
	"fmt"
	"math"
	"sync"
)

func GroupDegree(group int, phi float64) float64 {
	return math.Pow(1.0+phi, float64(group))
}

type LDSVertex struct {
	Level uint
}

type LDS struct {
	n              int
	levelsPerGroup float64
	lock           sync.RWMutex
	L              []LDSVertex
}

func NewLDS(n int, levelsPerGroup float64) *LDS {
	L := make([]LDSVertex, n)
	for i := 0; i < n; i++ {
		L[i] = LDSVertex{Level: 0}
	}
	return &LDS{
		n:              n,
		levelsPerGroup: levelsPerGroup,
		L:              L,
	}
}

func (lds *LDS) GetLevel(ngh uint) (uint, error) {
	// lds.lock.RLock()
	// defer lds.lock.RUnlock()

	if int(ngh) >= lds.n {
		return 0, fmt.Errorf("vertex index %v out of bounds\n", ngh)
	}
	return lds.L[ngh].Level, nil
}

func (lds *LDS) LevelIncrease(u uint) error {
	if int(u) >= lds.n {
		return fmt.Errorf("vertex index %v out of bounds\n", u)
	}
	lds.L[u].Level++
	// fmt.Printf("Level Increased for %d\n", u)
	return nil
}

func (lds *LDS) GroupForLevel(level uint) uint {
	return uint(math.Floor(float64(level) / lds.levelsPerGroup))
}
