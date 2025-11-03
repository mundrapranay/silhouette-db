package noise

// Code snippets from google-dp : https://github.com/google/differential-privacy/tree/main/go/v2/noise/laplace_noise.go
import (
	"math"

	"github.com/google/differential-privacy/go/v2/rand"
)

type GeomDistribution struct {
	lambda float64
}

func NewGeomDistribution(lambda float64) *GeomDistribution {
	return &GeomDistribution{
		lambda: lambda,
	}
}

// geometric draws a sample drawn from a geometric distribution with parameter
//
//	p = 1 - e^-λ.
//
// More precisely, it returns the number of Bernoulli trials until the first success
// where the success probability is p = 1 - e^-λ. The returned sample is truncated
// to the max int64 value.
//
// Note that to ensure that a truncation happens with probability less than 10⁻⁶,
// λ must be greater than 2⁻⁵⁹.
func (geom *GeomDistribution) geometric() int64 {
	// Return truncated sample in the case that the sample exceeds the max int64.
	if rand.Uniform() > -1.0*math.Expm1(-1.0*geom.lambda*math.MaxInt64) {
		return math.MaxInt64
	}

	// Perform a binary search for the sample in the interval from 1 to max int64.
	// Each iteration splits the interval in two and randomly keeps either the
	// left or the right subinterval depending on the respective probability of
	// the sample being contained in them. The search ends once the interval only
	// contains a single sample.
	var left int64 = 0              // exclusive bound
	var right int64 = math.MaxInt64 // inclusive bound

	for left+1 < right {
		// Compute a midpoint that divides the probability mass of the current interval
		// approximately evenly between the left and right subinterval. The resulting
		// midpoint will be less or equal to the arithmetic mean of the interval. This
		// reduces the expected number of iterations of the binary search compared to a
		// search that uses the arithmetic mean as a midpoint. The speed up is more
		// pronounced the higher the success probability p is.
		mid := left - int64(math.Floor((math.Log(0.5)+math.Log1p(math.Exp(geom.lambda*float64(left-right))))/geom.lambda))
		// Ensure that mid is contained in the search interval. This is a safeguard to
		// account for potential mathematical inaccuracies due to finite precision arithmetic.
		if mid <= left {
			mid = left + 1
		} else if mid >= right {
			mid = right - 1
		}

		// Probability that the sample is at most mid, i.e.,
		//   q = Pr[X ≤ mid | left < X ≤ right]
		// where X denotes the sample. The value of q should be approximately one half.
		q := math.Expm1(geom.lambda*float64(left-mid)) / math.Expm1(geom.lambda*float64(left-right))
		if rand.Uniform() <= q {
			right = mid
		} else {
			left = mid
		}
	}
	return right
}

// twoSidedGeometric draws a sample from a geometric distribution that is
// mirrored at 0. The non-negative part of the distribution's PDF matches
// the PDF of a geometric distribution of parameter p = 1 - e^-λ that is
// shifted to the left by 1 and scaled accordingly.
func (geom *GeomDistribution) TwoSidedGeometric() int64 {
	var sample int64 = 0
	var sign int64 = -1
	// Keep a sample of 0 only if the sign is positive. Otherwise, the
	// probability of 0 would be twice as high as it should be.
	for sample == 0 && sign == -1 {
		sample = geom.geometric() - 1
		sign = int64(rand.Sign())
	}
	return sample * sign
}
