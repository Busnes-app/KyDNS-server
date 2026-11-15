package dnssec

import "golang.org/x/sys/unix"

// kernelUnsynced reads the kernel clock state. adjtimex with no modes set only
// reads, so it needs no capability.
func kernelUnsynced() (bool, bool) {
	var t unix.Timex
	state, err := unix.Adjtimex(&t)
	if err != nil {
		return false, false
	}
	return state == unix.TIME_ERROR, true
}
