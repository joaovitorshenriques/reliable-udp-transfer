package transfer

import "syscall"

// MoveFile (unlike MoveFileEx with REPLACE_EXISTING) refuses existing targets.
func publish(src, dst string) error {
	from, e := syscall.UTF16PtrFromString(src)
	if e != nil {
		return e
	}
	to, e := syscall.UTF16PtrFromString(dst)
	if e != nil {
		return e
	}
	return syscall.MoveFile(from, to)
}
