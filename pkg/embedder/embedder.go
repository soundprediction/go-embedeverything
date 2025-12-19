package embedder

/*
#cgo LDFLAGS: -L${SRCDIR}/../../embed_anything_binding/target/debug -lembed_anything_binding -framework CoreFoundation -framework Security -lc++ -framework Metal -framework MetalKit -framework Foundation -framework MetalPerformanceShaders
#include <stdlib.h>

typedef struct {
    void* inner;
    void* runtime;
} EmbedderWrapper;

typedef struct {
    float* data;
    size_t len;
} EmbeddingVector;

extern EmbedderWrapper* new_embedder(const char* model_id, const char* architecture);
extern EmbeddingVector* embed_text(EmbedderWrapper* wrapper, const char* text);
extern void free_embedder(EmbedderWrapper* wrapper);
extern void free_embedding_vector(EmbeddingVector* vec);
*/
import "C"
import (
	"errors"
	"unsafe"
)

// Embedder wraps the Rust-based Embedder
type Embedder struct {
	ptr *C.EmbedderWrapper
}

// NewEmbedder creates a new Embedder instance.
// modelID: e.g. "Qwen/Qwen3-Embedding-0.6B" or "sentence-transformers/all-MiniLM-L6-v2"
// architecture: e.g. "Bert"
func NewEmbedder(modelID, architecture string) (*Embedder, error) {
	cModelID := C.CString(modelID)
	cArch := C.CString(architecture)
	defer C.free(unsafe.Pointer(cModelID))
	defer C.free(unsafe.Pointer(cArch))

	ptr := C.new_embedder(cModelID, cArch)
	if ptr == nil {
		return nil, errors.New("failed to create embedder (check logs)")
	}

	return &Embedder{ptr: ptr}, nil
}

// Close frees the underlying Rust resources.
func (e *Embedder) Close() {
	if e.ptr != nil {
		C.free_embedder(e.ptr)
		e.ptr = nil
	}
}

// Embed generates embeddings for the given text.
// Returns a slice of floats.
func (e *Embedder) Embed(text string) ([]float32, error) {
	if e.ptr == nil {
		return nil, errors.New("embedder is closed")
	}

	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	res := C.embed_text(e.ptr, cText)
	if res == nil {
		return nil, errors.New("failed to generate embedding")
	}
	defer C.free_embedding_vector(res)

	// Convert C array to Go slice
	// We have length res.len and data res.data
	// The C data is allocated by Rust (Box::into_raw), and we free it with free_embedding_vector.
	// So we need to copy the data into a Go slice.

	length := int(res.len)
	// Create a Go slice backed by the C array temporarily to copy
	slice := unsafe.Slice((*float32)(unsafe.Pointer(res.data)), length)

	// Make a copy so we can free the C vector safely
	out := make([]float32, length)
	copy(out, slice)

	return out, nil
}
