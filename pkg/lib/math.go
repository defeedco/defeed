package lib

import "math"

// FitGrowthRate finds a growth parameter `k` for the LogAsymptote function
// so that a given raw input maps to a desired normalized target.
//
// This is useful when you need to dynamically adapt the curve scaling,
// for example, normalizing values from different platforms or datasets
// where the maximum or "top" raw count varies.
//
// Parameters:
//   - x: The raw input value at which you want to hit the target (e.g., topStat).
//   - target: The normalized value you want at x. Must be between 0 and limit.
//   - limit: The asymptotic maximum of the normalization function, usually 1.0.
//
// Returns:
//   - k: The fitted growth parameter.
//   - Returns 0 if the input is invalid.
//
// Example:
//
//	Find k so that 20,000 maps to ~0.8
//	=> k := FitGrowthRate(20000, 0.8, 1.0)
//
//	normalized := LogAsymptote(20000, 1.0, k)
//	=> normalized ≈ 0.8
func FitGrowthRate(x, target, limit float64, steps int) float64 {
	if x <= 0 || target <= 0 || target >= limit {
		return -1
	}

	// Binary search bounds for k
	low := 1e-9     // very flat growth
	high := 10.0    // very steep growth
	epsilon := 1e-9 // convergence tolerance

	for range steps {
		mid := (low + high) / 2
		value := LogAsymptote(x, limit, mid)

		if math.Abs(value-target) < epsilon {
			return mid
		}

		if value < target {
			low = mid // increase k to steepen curve
		} else {
			high = mid // decrease k to flatten curve
		}
	}

	return (low + high) / 2
}

// LogAsymptote computes a logarithmic growth curve that asymptotically approaches
// a specified upper limit, using a single growth parameter `k`.
//
// The formula is:
//
//	f(x) = limit * ln(1 + k*x) / (1 + ln(1 + k*x))
//
// Parameters:
//   - x: The independent variable (e.g., time, steps, iterations). Must be >= 0.
//   - limit: The asymptotic maximum value. As x → ∞, f(x) → limit.
//   - k: Growth rate parameter. Larger values make the curve rise more quickly.
//
// Behavior:
//   - At x = 0, f(0) = 0.
//   - As x increases, f(x) monotonically increases but never exceeds limit.
//   - f(x) approaches limit asymptotically as x → ∞.
//
// Example:
//
//	value := LogAsymptote(10, 1.0, 0.5)
//	=> value ≈ 0.642
//
// Practical Use Cases:
//   - Smoothly normalizing an increasing metric to a fixed upper bound.
//   - Growth curves for product metrics or game progression systems.
func LogAsymptote(x, limit, k float64) float64 {
	// Clamp negative inputs to zero
	if x < 0 {
		x = 0
	}

	ln := math.Log(1 + k*x)
	return limit * ln / (1 + ln)
}
