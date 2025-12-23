package memsize

import (
	"fmt"
	"reflect"
)

// Sizes mirrors the structure expected by memsizeui. The implementation below
// is intentionally minimal: instead of walking the heap it simply returns
// zeroed statistics so that code linking against github.com/fjl/memsize keeps
// compiling even on toolchains where the original package breaks (e.g. Go 1.24).
type Sizes struct {
	Total             uintptr
	ByType            map[reflect.Type]*TypeSize
	BitmapSize        uintptr
	BitmapUtilization float32
}

type TypeSize struct {
	Total uintptr
	Count uintptr
}

// Scan pretends to analyse the memory footprint of v but in practice just
// returns an empty structure. This is sufficient for the debug endpoints that
// only need a syntactically valid response.
func Scan(v interface{}) Sizes {
	return Sizes{ByType: make(map[reflect.Type]*TypeSize)}
}

// HumanSize formats n using a human readable suffix.
func HumanSize(n uintptr) string {
	const (
		_            = iota
		kilo float64 = 1 << (10 * iota)
		mega
		giga
	)
	value := float64(n)
	switch {
	case value >= giga:
		return fmt.Sprintf("%.2f GiB", value/giga)
	case value >= mega:
		return fmt.Sprintf("%.2f MiB", value/mega)
	case value >= kilo:
		return fmt.Sprintf("%.2f KiB", value/kilo)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
