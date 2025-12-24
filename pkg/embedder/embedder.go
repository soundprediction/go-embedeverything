package embedder

/*
#cgo LDFLAGS: -ldl
#include <stdlib.h>
#include <dlfcn.h>
#include <stdio.h>

// Struct definitions matching Rust repr(C)
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

// Helper to dlopen
void* open_lib(const char* path) {
    return dlopen(path, RTLD_LAZY | RTLD_GLOBAL); // RTLD_GLOBAL needed for ort?
}

char* get_dlerror() {
    return dlerror();
}

void* get_sym(void* handle, const char* name) {
    return dlsym(handle, name);
}

// Function pointer typedefs and callers
typedef int (*init_onnx_runtime_t)(const char*);
int call_init_onnx_runtime(void* f, const char* path) {
    return ((init_onnx_runtime_t)f)(path);
}

typedef EmbedderWrapper* (*new_embedder_t)(const char*, const char*);
EmbedderWrapper* call_new_embedder(void* f, const char* m, const char* a) {
    return ((new_embedder_t)f)(m, a);
}

typedef BatchEmbeddingResult* (*embed_text_batch_t)(EmbedderWrapper*, const char**, size_t);
BatchEmbeddingResult* call_embed_text_batch(void* f, EmbedderWrapper* w, const char** t, size_t c) {
    return ((embed_text_batch_t)f)(w, t, c);
}

typedef void (*free_embedder_t)(EmbedderWrapper*);
void call_free_embedder(void* f, EmbedderWrapper* w) {
    ((free_embedder_t)f)(w);
}

typedef void (*free_batch_result_t)(BatchEmbeddingResult*);
void call_free_batch_result(void* f, BatchEmbeddingResult* r) {
    ((free_batch_result_t)f)(r);
}

typedef RerankerWrapper* (*new_reranker_t)(const char*);
RerankerWrapper* call_new_reranker(void* f, const char* m) {
    return ((new_reranker_t)f)(m);
}

typedef BatchRerankResult* (*rerank_documents_t)(RerankerWrapper*, const char*, const char**, size_t);
BatchRerankResult* call_rerank_documents(void* f, RerankerWrapper* w, const char* q, const char** d, size_t c) {
    return ((rerank_documents_t)f)(w, q, d, c);
}

typedef void (*free_reranker_t)(RerankerWrapper*);
void call_free_reranker(void* f, RerankerWrapper* w) {
    ((free_reranker_t)f)(w);
}

typedef void (*free_rerank_result_t)(BatchRerankResult*);
void call_free_rerank_result(void* f, BatchRerankResult* r) {
    ((free_rerank_result_t)f)(r);
}

*/
import "C"
import (
	"compress/gzip"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"
)

//go:embed lib
var libFS embed.FS

var (
	initialized        bool
	dlHandle           unsafe.Pointer
	fnInitOnnxRuntime  unsafe.Pointer
	fnNewEmbedder      unsafe.Pointer
	fnEmbedTextBatch   unsafe.Pointer
	fnFreeEmbedder     unsafe.Pointer
	fnFreeBatchResult  unsafe.Pointer
	fnNewReranker      unsafe.Pointer
	fnRerankDocuments  unsafe.Pointer
	fnFreeReranker     unsafe.Pointer
	fnFreeRerankResult unsafe.Pointer
)

// extractAndDecompress extracts a file from embed.FS, optionally decompressing if it ends in .gz
func extractAndDecompress(srcPath, destPath string) error {
	// Open source
	f, err := libFS.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open embedded %s: %w", srcPath, err)
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(srcPath, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("gzip reader %s: %w", srcPath, err)
		}
		defer gz.Close()
		r = gz
	}

	// Create dest
	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("create dest %s: %w", destPath, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, r); err != nil {
		return fmt.Errorf("copy %s: %w", srcPath, err)
	}
	return nil
}

// Init initializes the embedding library extracting the ONNX Runtime library.
func Init() error {
	if initialized {
		return nil
	}

	goOS := runtime.GOOS
	goArch := runtime.GOARCH

	var libPath string
	var libNameBinding string
	var libNameORT string

	switch goOS {
	case "darwin":
		libPath = "lib/darwin"
		libNameBinding = "libembed_anything_binding.dylib.gz"
		libNameORT = "libonnxruntime.dylib.gz" // Assumes we gzipped this too
	case "linux":
		if goArch == "amd64" {
			libPath = "lib/linux-amd64"
		} else if goArch == "arm64" {
			libPath = "lib/linux-arm64"
		} else {
			return fmt.Errorf("unsupported linux architecture: %s", goArch)
		}
		libNameBinding = "libembed_anything_binding.so.gz"
		libNameORT = "libonnxruntime.so.gz"
	default:
		return fmt.Errorf("unsupported OS: %s", goOS)
	}

	tmpDir, err := os.MkdirTemp("", "go-embedeverything-lib")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Extract Binding Lib
	bindingDest := filepath.Join(tmpDir, strings.TrimSuffix(libNameBinding, ".gz"))
	if err := extractAndDecompress(filepath.Join(libPath, libNameBinding), bindingDest); err != nil {
		return err
	}

	// Extract ORT Lib
	ortDest := filepath.Join(tmpDir, strings.TrimSuffix(libNameORT, ".gz"))
	if err := extractAndDecompress(filepath.Join(libPath, libNameORT), ortDest); err != nil {
		return err
	}

	// DLOPEN Binding Lib
	cBindingPath := C.CString(bindingDest)
	defer C.free(unsafe.Pointer(cBindingPath))
	dlHandle = C.open_lib(cBindingPath)
	if dlHandle == nil {
		cErr := C.get_dlerror()
		return fmt.Errorf("dlopen failed: %s", C.GoString(cErr))
	}

	// Load Symbols
	loadSym := func(name string) (unsafe.Pointer, error) {
		cName := C.CString(name)
		defer C.free(unsafe.Pointer(cName))
		sym := C.get_sym(dlHandle, cName)
		if sym == nil {
			return nil, fmt.Errorf("symbol not found: %s", name)
		}
		return sym, nil
	}

	if fnInitOnnxRuntime, err = loadSym("init_onnx_runtime"); err != nil {
		return err
	}
	if fnNewEmbedder, err = loadSym("new_embedder"); err != nil {
		return err
	}
	if fnEmbedTextBatch, err = loadSym("embed_text_batch"); err != nil {
		return err
	}
	if fnFreeEmbedder, err = loadSym("free_embedder"); err != nil {
		return err
	}
	if fnFreeBatchResult, err = loadSym("free_batch_result"); err != nil {
		return err
	}
	if fnNewReranker, err = loadSym("new_reranker"); err != nil {
		return err
	}
	if fnRerankDocuments, err = loadSym("rerank_documents"); err != nil {
		return err
	}
	if fnFreeReranker, err = loadSym("free_reranker"); err != nil {
		return err
	}
	if fnFreeRerankResult, err = loadSym("free_rerank_result"); err != nil {
		return err
	}

	// Init ORT
	cORTPath := C.CString(ortDest)
	defer C.free(unsafe.Pointer(cORTPath))
	ret := C.call_init_onnx_runtime(fnInitOnnxRuntime, cORTPath)
	if ret != 0 {
		return fmt.Errorf("init_onnx_runtime failed")
	}

	initialized = true
	return nil
}

func init() {
	if err := Init(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: go-embedeverything failed to initialize: %v\n", err)
	}
}

// Embedder struct
type Embedder struct {
	ptr *C.EmbedderWrapper
}

// NewEmbedder
func NewEmbedder(modelID string) (*Embedder, error) {
	if !initialized {
		return nil, errors.New("library not initialized")
	}
	arch := inferArchitecture(modelID)
	cModelID := C.CString(modelID)
	cArch := C.CString(arch)
	defer C.free(unsafe.Pointer(cModelID))
	defer C.free(unsafe.Pointer(cArch))

	ptr := C.call_new_embedder(fnNewEmbedder, cModelID, cArch)
	if ptr == nil {
		return nil, errors.New("failed to create embedder")
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
	if strings.Contains(lower, "sentence-transformers") || strings.Contains(lower, "bert") || strings.Contains(lower, "bge") || strings.Contains(lower, "e5") || strings.Contains(lower, "nomic") || strings.Contains(lower, "snowflake") {
		return "Bert"
	}
	if strings.Contains(lower, "jina") {
		return "JinaBert"
	}
	if strings.Contains(lower, "clip") {
		return "Clip"
	}
	return "Bert"
}

// Close
func (e *Embedder) Close() {
	if e.ptr != nil {
		C.call_free_embedder(fnFreeEmbedder, e.ptr)
		e.ptr = nil
	}
}

// Embed
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

	res := C.call_embed_text_batch(fnEmbedTextBatch, e.ptr, (**C.char)(unsafe.Pointer(&cArray[0])), C.size_t(len(texts)))
	if res == nil {
		return nil, errors.New("failed to generate embeddings")
	}
	defer C.call_free_batch_result(fnFreeBatchResult, res)

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

// Reranker
type Reranker struct {
	ptr *C.RerankerWrapper
}

// NewReranker
func NewReranker(modelID string) (*Reranker, error) {
	if !initialized {
		return nil, errors.New("library not initialized")
	}
	cModelID := C.CString(modelID)
	defer C.free(unsafe.Pointer(cModelID))

	ptr := C.call_new_reranker(fnNewReranker, cModelID)
	if ptr == nil {
		return nil, errors.New("failed to create reranker")
	}
	return &Reranker{ptr: ptr}, nil
}

// Close
func (r *Reranker) Close() {
	if r.ptr != nil {
		C.call_free_reranker(fnFreeReranker, r.ptr)
		r.ptr = nil
	}
}

type RerankResult struct {
	Index int     `json:"index"`
	Score float32 `json:"relevance_score"`
	Text  string  `json:"text"`
}

// Rerank
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

	res := C.call_rerank_documents(fnRerankDocuments, r.ptr, cQuery, (**C.char)(unsafe.Pointer(&cDocs[0])), C.size_t(len(documents)))
	if res == nil {
		return nil, errors.New("failed to rerank documents")
	}
	defer C.call_free_rerank_result(fnFreeRerankResult, res)

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
