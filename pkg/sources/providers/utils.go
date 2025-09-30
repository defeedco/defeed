package providers

import (
	"github.com/defeedco/defeed/pkg/lib"
)

// NormSocialScore normalizes a social score to a value between 0 and 1,
// using a logarithmic curve that maps the maxScore to approximately 0.8,
// allowing for values beyond maxScore to still increase but with diminishing returns.
func NormSocialScore(score float64, maxScore float64) float64 {
	if score <= 0 {
		return 0
	}
	// Fit k so that maxScore maps to ~0.8, leaving headroom for outliers
	k := lib.FitGrowthRate(maxScore, 0.8, 1.0, 50)
	if k <= 0 {
		// This should never happen
		panic("failed to fit k for social score normalization")
	}
	return lib.LogAsymptote(score, 1.0, k)
}
