package cdn

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSelection(t *testing.T) {
	calls := map[string]int{}
	probe := func(_ context.Context, a string) (time.Duration, error) {
		calls[a]++
		if a == "dead" {
			return 0, errors.New("timeout")
		}
		if a == "fast" && calls[a] == 1 {
			return time.Second, nil
		}
		if a == "fast" {
			return 10 * time.Millisecond, nil
		}
		return 20 * time.Millisecond, nil
	}
	selected, results, err := Select(context.Background(), []string{"slow", "dead", "fast"}, 5, probe)
	if err != nil || selected != "fast" || results[1].Eligible || results[2].MedianMS != 10 {
		t.Fatalf("%s %+v %v", selected, results, err)
	}
}
func TestSelectionFailures(t *testing.T) {
	fail := func(context.Context, string) (time.Duration, error) { return 0, errors.New("timeout") }
	if _, _, err := Select(context.Background(), []string{"a", "b"}, 3, fail); err == nil {
		t.Fatal("accepted all failed")
	}
	if _, _, err := Select(context.Background(), []string{"a", "a"}, 3, fail); err == nil {
		t.Fatal("accepted duplicates")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Select(ctx, []string{"a", "b"}, 3, fail); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
