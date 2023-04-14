package peds

import "unsafe"

func nilinterhash(p unsafe.Pointer, h uintptr) uintptr { return 0 }

// TODO: Try to avoid interfaces for hashing
func genericHash(x interface{}) uint32 {
	return uint32(nilinterhash(unsafe.Pointer(&x), 0))
}
