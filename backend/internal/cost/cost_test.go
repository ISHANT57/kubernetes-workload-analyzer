package cost

import (
	"errors"
	"testing"
)

func TestLoadPricing_Unset(t *testing.T) {
	t.Setenv("CPU_CORE_HOUR_USD", "")
	t.Setenv("MEMORY_GIB_HOUR_USD", "")

	_, err := LoadPricing()
	if !errors.Is(err, PricingUnset) {
		t.Errorf("LoadPricing() with nothing set: err = %v, want PricingUnset", err)
	}
}

func TestLoadPricing_PartiallySet_StillUnset(t *testing.T) {
	// Only one of the two prices configured must still be treated as fully unset -- a cost
	// estimate needs both, and computing one with a silent $0 for the other would be worse than
	// refusing outright.
	t.Setenv("CPU_CORE_HOUR_USD", "0.05")
	t.Setenv("MEMORY_GIB_HOUR_USD", "")

	_, err := LoadPricing()
	if !errors.Is(err, PricingUnset) {
		t.Errorf("LoadPricing() with only CPU price set: err = %v, want PricingUnset", err)
	}
}

func TestLoadPricing_Valid(t *testing.T) {
	t.Setenv("CPU_CORE_HOUR_USD", "0.05")
	t.Setenv("MEMORY_GIB_HOUR_USD", "0.01")
	t.Setenv("PRICE_SOURCE", "test fixture")

	p, err := LoadPricing()
	if err != nil {
		t.Fatalf("LoadPricing(): unexpected error: %v", err)
	}
	if p.CPUCoreHourUSD != 0.05 || p.MemoryGiBHourUSD != 0.01 {
		t.Errorf("Pricing = %+v, want CPU=0.05 Mem=0.01", p)
	}
}

func TestLoadPricing_RejectsZeroOrNegative(t *testing.T) {
	t.Setenv("CPU_CORE_HOUR_USD", "0")
	t.Setenv("MEMORY_GIB_HOUR_USD", "0.01")

	if _, err := LoadPricing(); !errors.Is(err, PricingUnset) {
		t.Errorf("LoadPricing() with CPU price = 0: err = %v, want PricingUnset", err)
	}
}

func TestEstimate_AllocationUsesMaxOfRequestAndUsage(t *testing.T) {
	pricing := Pricing{CPUCoreHourUSD: 0.10, MemoryGiBHourUSD: 0.02, Source: "test"}

	// allocationCores is passed in already as max(request, usage) by the caller (findings.go) --
	// this test just checks the arithmetic once that number is given.
	impact := Estimate(pricing, 1.0, 0.5, 0, 0, 100)

	wantAllocation := 1.0 * 0.10 * 100 // 10.0
	if impact.AllocationCostUSD != wantAllocation {
		t.Errorf("AllocationCostUSD = %v, want %v", impact.AllocationCostUSD, wantAllocation)
	}
	wantOptimized := 0.5 * 0.10 * 100 // 5.0
	if impact.OptimizedCostUSD != wantOptimized {
		t.Errorf("OptimizedCostUSD = %v, want %v", impact.OptimizedCostUSD, wantOptimized)
	}
	wantDiff := wantAllocation - wantOptimized
	if impact.PotentialDifferenceUSD != wantDiff {
		t.Errorf("PotentialDifferenceUSD = %v, want %v", impact.PotentialDifferenceUSD, wantDiff)
	}
}

func TestEstimate_IncludesMemoryInGiB(t *testing.T) {
	pricing := Pricing{CPUCoreHourUSD: 0, MemoryGiBHourUSD: 1.0, Source: "test"}
	oneGiB := float64(1024 * 1024 * 1024)

	impact := Estimate(pricing, 0, 0, oneGiB, oneGiB, 1)
	if impact.AllocationCostUSD != 1.0 {
		t.Errorf("AllocationCostUSD = %v, want 1.0 (1 GiB for 1 hour at $1/GiB-hour)", impact.AllocationCostUSD)
	}
}

// TestEstimate_NeverLabelsDifferenceAsSavings guards against a regression that would violate
// AGENTS.md directly: the word "saving" must never appear in the assumptions text.
func TestEstimate_NeverLabelsDifferenceAsSavings(t *testing.T) {
	pricing := Pricing{CPUCoreHourUSD: 0.1, MemoryGiBHourUSD: 0.02, Source: "test"}
	impact := Estimate(pricing, 1.0, 0.5, 0, 0, 730)

	for _, a := range impact.Assumptions {
		if containsFold(a, "saving") {
			t.Errorf("assumption text uses the word 'saving': %q (must say 'potential difference', per AGENTS.md)", a)
		}
	}
}

func containsFold(s, sub string) bool {
	sLower, subLower := []byte(s), []byte(sub)
	toLower := func(b byte) byte {
		if b >= 'A' && b <= 'Z' {
			return b + 32
		}
		return b
	}
	for i := range sLower {
		sLower[i] = toLower(sLower[i])
	}
	for i := range subLower {
		subLower[i] = toLower(subLower[i])
	}
	s2, sub2 := string(sLower), string(subLower)
	for i := 0; i+len(sub2) <= len(s2); i++ {
		if s2[i:i+len(sub2)] == sub2 {
			return true
		}
	}
	return false
}
