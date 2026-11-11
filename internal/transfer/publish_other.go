//go:build !windows

package transfer

import "os"

func publish(src, dst string) error { return os.Link(src, dst) }
