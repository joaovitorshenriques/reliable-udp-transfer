// Package cdn selects replicas using repeated application RTT observations.
package cdn

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type Result struct {
	Address   string    `json:"address"`
	SamplesMS []float64 `json:"samples_ms"`
	Errors    []string  `json:"errors"`
	MedianMS  float64   `json:"median_ms"`
	Eligible  bool      `json:"eligible"`
}
type Probe func(context.Context, string) (time.Duration, error)

// Select rotates the candidate order each round. At least a majority of samples
// must succeed. Ties preserve input order. Failed samples are never zero RTTs.
func Select(ctx context.Context, addresses []string, samples int, probe Probe) (string, []Result, error) {
	if len(addresses) < 2 || len(addresses) > 8 || samples < 3 || samples > 15 {
		return "", nil, fmt.Errorf("require 2..8 replicas and 3..15 samples")
	}
	seen := map[string]bool{}
	results := make([]Result, len(addresses))
	for i, a := range addresses {
		if a == "" || seen[a] {
			return "", nil, fmt.Errorf("empty or duplicate replica")
		}
		seen[a] = true
		results[i].Address = a
	}
	for round := 0; round < samples; round++ {
		for offset := range addresses {
			if err := ctx.Err(); err != nil {
				return "", results, err
			}
			i := (round + offset) % len(addresses)
			elapsed, err := probe(ctx, addresses[i])
			if err != nil {
				results[i].Errors = append(results[i].Errors, err.Error())
			} else if elapsed < 0 {
				results[i].Errors = append(results[i].Errors, "invalid RTT")
			} else {
				results[i].SamplesMS = append(results[i].SamplesMS, float64(elapsed)/float64(time.Millisecond))
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return "", results, err
	}
	best := -1
	for i := range results {
		r := &results[i]
		r.Eligible = len(r.SamplesMS) >= samples/2+1
		if !r.Eligible {
			continue
		}
		sorted := append([]float64(nil), r.SamplesMS...)
		sort.Float64s(sorted)
		r.MedianMS = sorted[len(sorted)/2]
		if len(sorted)%2 == 0 {
			r.MedianMS = (sorted[len(sorted)/2-1] + r.MedianMS) / 2
		}
		if best < 0 || r.MedianMS < results[best].MedianMS {
			best = i
		}
	}
	if best < 0 {
		return "", results, fmt.Errorf("no replica has a majority of successful probes")
	}
	return addresses[best], results, nil
}
