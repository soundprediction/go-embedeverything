package embedder

//go:generate sh ../../scripts/download_lib.sh

/*
#cgo LDFLAGS: -lembed_anything_binding
#include <stdlib.h>

typedef struct {
    void* inner;
    void* runtime;
} EmbedderWrapper;

typedef struct {
    void* inner;
    void* runtime;
} RerankerWrapper;

typedef struct {
    float* data;
    size_t len;
} EmbeddingVector;

typedef struct {
    EmbeddingVector* vectors;
    size_t count;
} BatchEmbeddingResult;

typedef struct {
    size_t index;
    float score;
    char* text;
} RerankResult;

typedef struct {
    RerankResult* results;
    size_t count;
} BatchRerankResult;

extern EmbedderWrapper* new_embedder(const char* model_id, const char* architecture);
extern BatchEmbeddingResult* embed_text_batch(EmbedderWrapper* wrapper, const char** texts, size_t count);
extern void free_embedder(EmbedderWrapper* wrapper);
extern void free_batch_result(BatchEmbeddingResult* result);

extern RerankerWrapper* new_reranker(const char* model_id);
extern BatchRerankResult* rerank_documents(RerankerWrapper* wrapper, const char* query, const char** documents, size_t count);
extern void free_reranker(RerankerWrapper* wrapper);
extern void free_rerank_result(BatchRerankResult* result);
*/
import "C"
import (
	"errors"
	"strings"
	"unsafe"
)

// Embedder wraps the Rust-based Embedder
type Embedder struct {
	ptr *C.EmbedderWrapper
}

// NewEmbedder creates a new Embedder instance.
func NewEmbedder(modelID string) (*Embedder, error) {
	arch := inferArchitecture(modelID)
	cModelID := C.CString(modelID)
	cArch := C.CString(arch)
	defer C.free(unsafe.Pointer(cModelID))
	defer C.free(unsafe.Pointer(cArch))

	ptr := C.new_embedder(cModelID, cArch)
	if ptr == nil {
		return nil, errors.New("failed to create embedder (check logs)")
	}

	return &Embedder{ptr: ptr}, nil
}

func inferArchitecture(modelID string) string {
	lower := strings.ToLower(modelID)
	if strings.Contains(lower, "qwen2.5") || strings.Contains(lower, "qwen2") {
		return "Qwen2"
	}
	if strings.Contains(lower, "qwen3") {
		return "Qwen3"
	}
	if strings.Contains(lower, "qwen") {
		return "Qwen2"
	}
	// Common Bert-based models
	if strings.Contains(lower, "sentence-transformers") ||
		strings.Contains(lower, "bert") ||
		strings.Contains(lower, "bge") ||
		strings.Contains(lower, "e5") ||
		strings.Contains(lower, "nomic") ||
		strings.Contains(lower, "snowflake") {
		return "Bert"
	}
	if strings.Contains(lower, "jina") {
		return "JinaBert"
	}
	if strings.Contains(lower, "clip") {
		return "Clip"
	}

	// Default to Bert
	return "Bert"
}

// Close frees the underlying Rust resources.
func (e *Embedder) Close() {
	if e.ptr != nil {
		C.free_embedder(e.ptr)
		e.ptr = nil
	}
}

// Embed generates embeddings for the given text(s).
func (e *Embedder) Embed(input interface{}) ([][]float32, error) {
	if e.ptr == nil {
		return nil, errors.New("embedder is closed")
	}

	var texts []string
	switch v := input.(type) {
	case string:
		texts = []string{v}
	case []string:
		texts = v
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				texts = append(texts, s)
			} else {
				return nil, errors.New("input array must contain strings")
			}
		}
	default:
		return nil, errors.New("input must be string or array of strings")
	}

	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	cArray := make([]*C.char, len(texts))
	for i, s := range texts {
		cStr := C.CString(s)
		defer C.free(unsafe.Pointer(cStr))
		cArray[i] = cStr
	}

	res := C.embed_text_batch(e.ptr, (**C.char)(unsafe.Pointer(&cArray[0])), C.size_t(len(texts)))
	if res == nil {
		return nil, errors.New("failed to generate embeddings")
	}
	defer C.free_batch_result(res)

	count := int(res.count)
	vectors := unsafe.Slice(res.vectors, count)

	out := make([][]float32, count)
	for i := 0; i < count; i++ {
		v := vectors[i]
		if v.data == nil {
			out[i] = []float32{}
			continue
		}
		length := int(v.len)
		dataSlice := unsafe.Slice((*float32)(unsafe.Pointer(v.data)), length)
		vec := make([]float32, length)
		copy(vec, dataSlice)
		out[i] = vec
	}

	return out, nil
}

// Reranker wraps the Rust-based Reranker
type Reranker struct {
	ptr *C.RerankerWrapper
}

// NewReranker creates a new Reranker instance.
func NewReranker(modelID string) (*Reranker, error) {
	cModelID := C.CString(modelID)
	defer C.free(unsafe.Pointer(cModelID))

	ptr := C.new_reranker(cModelID)
	if ptr == nil {
		return nil, errors.New("failed to create reranker (check logs)")
	}
	return &Reranker{ptr: ptr}, nil
}

// Close frees the underlying Rust resources.
func (r *Reranker) Close() {
	if r.ptr != nil {
		C.free_reranker(r.ptr)
		r.ptr = nil
	}
}

type RerankResult struct {
	Index int     `json:"index"`
	Score float32 `json:"relevance_score"`
	Text  string  `json:"text"`
}

// Rerank reranks the documents based on query.
func (r *Reranker) Rerank(query string, documents []string) ([]RerankResult, error) {
	if r.ptr == nil {
		return nil, errors.New("reranker is closed")
	}
	if len(documents) == 0 {
		return []RerankResult{}, nil
	}

	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))

	cDocs := make([]*C.char, len(documents))
	for i, doc := range documents {
		cStr := C.CString(doc)
		defer C.free(unsafe.Pointer(cStr))
		cDocs[i] = cStr
	}

	res := C.rerank_documents(r.ptr, cQuery, (**C.char)(unsafe.Pointer(&cDocs[0])), C.size_t(len(documents)))
	if res == nil {
		return nil, errors.New("failed to rerank documents")
	}
	defer C.free_rerank_result(res)

	count := int(res.count)
	results := unsafe.Slice(res.results, count)

	out := make([]RerankResult, count)
	for i := 0; i < count; i++ {
		out[i] = RerankResult{
			Index: int(results[i].index),
			Score: float32(results[i].score),
			Text:  C.GoString(results[i].text),
		}
	}
	return out, nil
}
