package transfer

import "time"

// Educational estimator inspired by RFC 6298. This is not a TCP timer:
// RC2 uses per-packet timers and configurable bounds for controlled experiments.
type rtoEstimator struct {
	current, srtt, variation time.Duration
	minimum, maximum         time.Duration
	samples                  int
}

func (r *rtoEstimator) sample(d time.Duration) {
	d = max(d, time.Microsecond)
	if r.samples == 0 {
		r.srtt = d
		r.variation = d / 2
	} else {
		delta := r.srtt - d
		if delta < 0 {
			delta = -delta
		}
		r.variation = (3*r.variation + delta) / 4
		r.srtt = (7*r.srtt + d) / 8
	}
	r.samples++
	r.current = min(r.maximum, max(r.minimum, r.srtt+max(time.Millisecond, 4*r.variation)))
}
func (r *rtoEstimator) backoff(previous time.Duration) time.Duration {
	return min(r.maximum, max(r.current, 2*previous))
}
