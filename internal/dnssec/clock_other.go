//go:build !linux

package dnssec

// kernelUnsynced cannot tell outside Linux; only the build floor applies.
func kernelUnsynced() (bool, bool) { return false, false }
