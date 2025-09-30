package lib

import (
	"math"
	"testing"
)

// TestLogAsymptote_Basic checks basic expected behavior of the LogAsymptote function.
func TestLogAsymptote_Basic(t *testing.T) {
	limit := 1.0
	k := 0.5

	// At x=0, output should be exactly 0
	if v := LogAsymptote(0, limit, k); v != 0 {
		t.Errorf("expected 0 at x=0, got %.6f", v)
	}

	// As x increases, output should increase but never exceed limit
	prev := 0.0
	for x := 1.0; x <= 100.0; x *= 2 {
		v := LogAsymptote(x, limit, k)
		if v <= prev {
			t.Errorf("expected strictly increasing output, got %.6f <= %.6f at x=%.2f", v, prev, x)
		}
		if v > limit {
			t.Errorf("expected output <= limit (%.2f), got %.6f at x=%.2f", limit, v, x)
		}
		prev = v
	}

	// Large x should asymptotically approach limit
	largeX := 1e20
	v := LogAsymptote(largeX, limit, k)
	// With the formula ln/(1+ln), the function approaches the limit slowly
	// For very large x, we expect to be within ~2% of the limit
	delta := math.Abs(v - limit)
	if delta > 0.022 {
		t.Errorf("expected value close to limit %.2f, got %.6f (delta=%.6f)", limit, v, delta)
	}
}

// TestFitGrowthRate verifies that FitGrowthRate finds a k that maps top value to the desired target.
func TestFitGrowthRate(t *testing.T) {
	topValue := 20000.0
	target := 0.8
	limit := 1.0

	k := FitGrowthRate(topValue, target, limit, 50)
	if k <= 0 {
		t.Fatalf("expected positive k, got %.8f", k)
	}

	// Verify that when we plug topValue into LogAsymptote, we get ~target
	normalized := LogAsymptote(topValue, limit, k)
	if math.Abs(normalized-target) > 0.022 {
		t.Errorf("expected normalized %.6f, got %.6f (k=%.8f)", target, normalized, k)
	}
}

// TestFitGrowthRate_InvalidInputs checks behavior for invalid inputs.
func TestFitGrowthRate_InvalidInputs(t *testing.T) {
	cases := []struct {
		x, target, limit float64
	}{
		{-10, 0.8, 1.0},  // negative x
		{100, -0.5, 1.0}, // negative target
		{100, 1.0, 1.0},  // target == limit
		{100, 1.1, 1.0},  // target > limit
	}

	for _, c := range cases {
		k := FitGrowthRate(c.x, c.target, c.limit, 100)
		if k != -1 {
			t.Errorf("expected k=0 for invalid inputs (x=%.2f, target=%.2f, limit=%.2f), got %.8f",
				c.x, c.target, c.limit, k)
		}
	}
}
