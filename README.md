# go-embedeverything

Golang bindings for [EmbedAnything](https://github.com/StarlightSearch/EmbedAnything) enabling high-performance, local embedding generation and reranking.

## Features

- **Rust Backend**: Uses `candle` and `ort` for fast inference.
- **Metal Acceleration**: Optimized for Apple Silicon.
- **OpenAI & Cohere Compatible API**: Drop-in replacement for embedding and reranking endpoints.
- **CLI**: Easy-to-use command line interface.

## Quick Start

### 1. Run the Server

```bash
# Downloads the default Qwen3-0.6B embedding model and Jina-v1-Turbo reranker
go run main.go serve
```

Custom configuration:

```bash
go run main.go serve --model "Qwen/Qwen3-Embedding-0.6B" --rerank-model "jinaai/jina-reranker-v1-turbo-en" --port 9090
```

### 2. Generate Embeddings (OpenAI Compatible)

```bash
curl -X POST http://localhost:8080/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Qwen/Qwen3-Embedding-0.6B",
    "input": ["Hello world", "Embeddings are cool"]
  }'
```

### 3. Rerank Documents (Cohere Compatible)

```bash
curl -X POST http://localhost:8080/v1/rerank \
  -H "Content-Type: application/json" \
  -d '{
    "model": "jinaai/jina-reranker-v1-turbo-en",
    "query": "What is the capital of France?",
    "documents": ["Berlin", "Paris", "London", "Madrid"]
  }'
```

## Golang API Usage

Use the `pkg/embedder` package to embed or rerank text directly in your Go code.

```go
package main

import (
	"fmt"
	"log"

	"github.com/soundprediction/go-embedeverything/pkg/embedder"
)

func main() {
	// Initialize embedder with Qwen3-0.6B
	e, err := embedder.NewEmbedder("Qwen/Qwen3-Embedding-0.6B")
	if err != nil {
		log.Fatalf("Failed to create embedder: %v", err)
	}
	defer e.Close()

	// Generate embeddings
	texts := []string{"First sentence", "Second sentence"}
	vectors, err := e.Embed(texts)
	if err != nil {
		log.Fatalf("Embed failed: %v", err)
	}
    fmt.Printf("Generated %d vectors\n", len(vectors))

    // Initialize reranker
    r, err := embedder.NewReranker("jinaai/jina-reranker-v1-turbo-en")
    if err != nil {
        log.Fatalf("Failed to create reranker: %v", err)
    }
    defer r.Close()

    // Rerank
    query := "Apple"
    docs := []string{"Banana", "iPhone", "Orange"}
    results, err := r.Rerank(query, docs)
    if err != nil {
        log.Fatalf("Rerank failed: %v", err)
    }

    for _, res := range results {
        fmt.Printf("Doc %d: Score %f\n", res.Index, res.Score)
    }
}
```

## Build Requirements

- **Go**: 1.20+
- **Rust**: Latest stable (with `cargo`)
- **macOS (Apple Silicon)**: Optimized with `metal` feature.
