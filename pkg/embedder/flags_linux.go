//go:build linux

package embedder

/*
#cgo amd64 LDFLAGS: -L${SRCDIR}/lib/linux-amd64
#cgo arm64 LDFLAGS: -L${SRCDIR}/lib/linux-arm64
#cgo LDFLAGS: -lstdc++
*/
import "C"
