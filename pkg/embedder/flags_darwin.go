//go:build darwin

package embedder

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Security -lc++ -framework Metal -framework MetalKit -framework Foundation -framework MetalPerformanceShaders
*/
import "C"
