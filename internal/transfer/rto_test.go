package transfer

import (
	"testing"
	"time"
)

func TestRTOEstimator(t *testing.T) {
	r := rtoEstimator{current: 500 * time.Millisecond, minimum: 50 * time.Millisecond, maximum: 2 * time.Second}
	r.sample(100 * time.Millisecond)
	if r.srtt != 100*time.Millisecond || r.variation != 50*time.Millisecond || r.current != 300*time.Millisecond {
		t.Fatalf("first sample: %+v", r)
	}
	r.sample(140 * time.Millisecond)
	if r.srtt != 105*time.Millisecond || r.variation != 47500*time.Microsecond || r.current != 295*time.Millisecond {
		t.Fatalf("second sample: %+v", r)
	}
	if r.backoff(1500*time.Millisecond) != 2*time.Second {
		t.Fatal("backoff cap")
	}
	r.sample(10 * time.Second)
	if r.current != 2*time.Second {
		t.Fatal("upper bound")
	}
	r = rtoEstimator{minimum: 50 * time.Millisecond, maximum: 2 * time.Second}
	r.sample(0)
	if r.current != 50*time.Millisecond {
		t.Fatal("zero sample floor")
	}
}
func TestAdaptiveConfig(t *testing.T) {
	c := DefaultConfig()
	c.Idle = c.MaxRTO
	if c.Validate() == nil {
		t.Fatal("idle must outlast largest data timer")
	}
	c = DefaultConfig()
	c.MinRTO = 3 * time.Second
	if c.Validate() == nil {
		t.Fatal("invalid bounds accepted")
	}
}
