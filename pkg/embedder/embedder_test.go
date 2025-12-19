package embedder_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/soundprediction/go-embedeverything/pkg/embedder"
)

// Qwen3 models (0.6B) for testing
const (
	EmbeddingModelID = "Qwen/Qwen3-Embedding-0.6B"
	RerankModelID    = "zhiqing/Qwen3-Reranker-0.6B-ONNX"
)

func TestEmbedding(t *testing.T) {
	fmt.Printf("Loading embedding model: %s\n", EmbeddingModelID)
	// Initialize Embedder
	e, err := embedder.NewEmbedder(EmbeddingModelID)
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}
	defer e.Close()

	// Test single string embedding
	input := "Hello world"
	vecs, err := e.Embed(input)
	if err != nil {
		t.Fatalf("Embed single failed: %v", err)
	}
	if len(vecs) != 1 {
		t.Errorf("Expected 1 vector, got %d", len(vecs))
	}
	if len(vecs[0]) == 0 {
		t.Error("Vector is empty")
	}
	t.Logf("Single embedding dim: %d", len(vecs[0]))

	// Test batch embedding
	batchInput := []string{"First sentence", "Second sentence"}
	batchVecs, err := e.Embed(batchInput)
	if err != nil {
		t.Fatalf("Embed batch failed: %v", err)
	}
	if len(batchVecs) != 2 {
		t.Errorf("Expected 2 vectors, got %d", len(batchVecs))
	}
	for i, v := range batchVecs {
		if len(v) == 0 {
			t.Errorf("Vector %d is empty", i)
		}
	}
}

func TestReranking(t *testing.T) {
	fmt.Printf("Loading reranker model: %s\n", RerankModelID)
	r, err := embedder.NewReranker(RerankModelID)
	if err != nil {
		t.Fatalf("Failed to create reranker: %v", err)
	}
	defer r.Close()

	query := "What is the capital of France?"
	documents := []string{
		"Berlin is the capital of Germany.",
		"Paris is the capital of France.",
		"London is the capital of the UK.",
		"Madrid is the capital of Spain.",
	}

	results, err := r.Rerank(query, documents)
	if err != nil {
		t.Fatalf("Rerank failed: %v", err)
	}
	if len(results) == 0 {
		t.Errorf("Expected results, got 0")
	} else if len(results) < len(documents) {
		t.Logf("Warning: Expected %d results, got %d. Reranker might be doing top-k", len(documents), len(results))
	}

	t.Logf("Rerank results:")
	for i, res := range results {
		t.Logf("Rank %d: Doc Index %d, Score %f", i, res.Index, res.Score)
	}

	// Check if Paris is ranked first (by index or content)
	if len(results) > 0 {
		best := results[0]
		t.Logf("Best result: Index=%d Score=%f Text=%s", best.Index, best.Score, best.Text)

		if best.Index != 1 && !strings.Contains(best.Text, "Paris") {
			t.Errorf("Expected Paris (index 1) to be first, got index %d with text: %s", best.Index, best.Text)
		}
	}
}
