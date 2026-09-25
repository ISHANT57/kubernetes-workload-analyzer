// Package cost turns a resource finding into an *estimated* dollar impact
// (docs/requirements.md §4). It never claims real savings: the output is always "allocation
// cost" vs "optimized cost" vs "potential difference", and AGENTS.md is explicit that a request
// reduction is never called "savings" -- reducing a request only reduces the bill if it lets a
// node be removed or avoided, which this package has no way to know.
package cost

import (
	"fmt"
	"os"
	"strconv"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// PricingUnset is returned by Load when no pricing configuration is present. This is a valid,
// expected state (pricing is optional -- health and rightsizing findings work without it) that
// callers must check explicitly, never silently substitute a made-up number for.
var PricingUnset = fmt.Errorf("cost: no pricing configured (set CPU_CORE_HOUR_USD and MEMORY_GIB_HOUR_USD)")

// Pricing is a flat-rate cost model: one price per CPU core-hour, one per GiB-hour of memory.
// There is deliberately no built-in default -- docs/requirements.md §4: "no built-in defaults
// that imply a real cloud price". The source of these numbers is the caller's responsibility to
// document (see Assumptions below); a placeholder value used for demonstration purposes must say
// so explicitly rather than be presented as if it came from a real provider's price list.
type Pricing struct {
	CPUCoreHourUSD   float64
	MemoryGiBHourUSD float64
	Source           string // e.g. "example only, not sourced from a real cloud provider"
}

// LoadPricing reads pricing from CPU_CORE_HOUR_USD, MEMORY_GIB_HOUR_USD and PRICE_SOURCE. It
// returns PricingUnset (not a zero-valued Pricing) when either price is missing, so a caller
// cannot accidentally compute a cost of $0.00 and mistake that for "this workload is free" --
// zero cost from unset pricing and zero cost from a genuinely free workload must never look the
// same.
func LoadPricing() (Pricing, error) {
	cpu, cpuErr := getPositiveFloat("CPU_CORE_HOUR_USD")
	mem, memErr := getPositiveFloat("MEMORY_GIB_HOUR_USD")
	if cpuErr != nil || memErr != nil {
		return Pricing{}, PricingUnset
	}
	source := os.Getenv("PRICE_SOURCE")
	if source == "" {
		source = "not documented -- set PRICE_SOURCE to say where these numbers come from"
	}
	return Pricing{CPUCoreHourUSD: cpu, MemoryGiBHourUSD: mem, Source: source}, nil
}

func getPositiveFloat(key string) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return 0, fmt.Errorf("%s not set", key)
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid number %q: %w", key, v, err)
	}
	if f <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %v", key, f)
	}
	return f, nil
}

// Estimate computes the CostImpact for one resource finding. allocationCores/allocationBytes is
// the *allocation*, not the raw request -- per docs/requirements.md's cost model table,
// allocation = max(request, observed usage), matching OpenCost's own definition, so a workload
// that is bursting above its request is not under-costed. optimizedCores/optimizedBytes is the
// rule's suggested request. hours is how many hours this estimate covers (e.g. 730 for a
// monthly figure); the caller decides that, this function does not assume a period.
func Estimate(pricing Pricing, allocationCores, optimizedCores, allocationBytes, optimizedBytes, hours float64) model.CostImpact {
	const bytesPerGiB = 1024 * 1024 * 1024

	allocationCost := allocationCores*pricing.CPUCoreHourUSD*hours + (allocationBytes/bytesPerGiB)*pricing.MemoryGiBHourUSD*hours
	optimizedCost := optimizedCores*pricing.CPUCoreHourUSD*hours + (optimizedBytes/bytesPerGiB)*pricing.MemoryGiBHourUSD*hours

	assumptions := []string{
		fmt.Sprintf("CPU: $%.5f/core-hour", pricing.CPUCoreHourUSD),
		fmt.Sprintf("Memory: $%.5f/GiB-hour", pricing.MemoryGiBHourUSD),
		fmt.Sprintf("Price source: %s", pricing.Source),
		fmt.Sprintf("Period: %.0f hours", hours),
		"Allocation = max(request, observed usage), not the raw request (matches OpenCost's definition)",
		"This is an ESTIMATE, not an invoice. Reducing a request does not by itself reduce a cloud bill -- " +
			"it only does if the freed capacity lets a node be removed, downsized, or not added.",
	}

	return model.CostImpact{
		AllocationCostUSD:      allocationCost,
		OptimizedCostUSD:       optimizedCost,
		PotentialDifferenceUSD: allocationCost - optimizedCost, // never called "savings" -- see the package doc
		Assumptions:            assumptions,
	}
}
