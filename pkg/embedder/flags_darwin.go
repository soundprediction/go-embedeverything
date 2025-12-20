//go:build darwin

package embedder

/*
#cgo LDFLAGS: -L${SRCDIR}/lib/darwin -framework CoreFoundation -framework Security -lc++ -framework Metal -framework MetalKit -framework Foundation -framework MetalPerformanceShaders
*/
import "C"
