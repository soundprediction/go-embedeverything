package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/soundprediction/go-embedeverything/pkg/embedder"
	"github.com/spf13/cobra"
)

var (
	modelID       string
	rerankModelID string
	port          int
)

var rootCmd = &cobra.Command{
	Use:   "embed-server",
	Short: "A REST server for generating embeddings and reranking",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the REST API server",
	Run: func(cmd *cobra.Command, args []string) {
		startServer()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVar(&modelID, "model", "Qwen/Qwen3-Embedding-0.6B", "HuggingFace embedding model ID")
	serveCmd.Flags().StringVar(&rerankModelID, "rerank-model", "jinaai/jina-reranker-v1-turbo-en", "HuggingFace reranker model ID")
	serveCmd.Flags().IntVar(&port, "port", 8080, "Port to listen on")
}

func startServer() {
	fmt.Printf("Loading embedding model %s...\n", modelID)
	emb, err := embedder.NewEmbedder(modelID)
	if err != nil {
		log.Fatalf("Failed to initialize embedder: %v", err)
	}
	defer emb.Close()
	fmt.Println("Embedding model loaded.")

	var reranker *embedder.Reranker
	if rerankModelID != "" {
		fmt.Printf("Loading reranker model %s...\n", rerankModelID)
		r, err := embedder.NewReranker(rerankModelID)
		if err != nil {
			log.Printf("Failed to initialize reranker: %v. Reranking will be disabled.", err)
		} else {
			reranker = r
			defer reranker.Close()
			fmt.Println("Reranker model loaded.")
		}
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/v1/embeddings", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input interface{} `json:"input"`
			Model string      `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Input == nil {
			http.Error(w, "Input text is required", http.StatusBadRequest)
			return
		}

		results, err := emb.Embed(req.Input)
		if err != nil {
			http.Error(w, fmt.Sprintf("Embedding failed: %v", err), http.StatusInternalServerError)
			return
		}

		type EmbeddingObject struct {
			Object    string    `json:"object"`
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		}

		data := make([]EmbeddingObject, len(results))
		for i, vec := range results {
			data[i] = EmbeddingObject{
				Object:    "embedding",
				Embedding: vec,
				Index:     i,
			}
		}

		resp := struct {
			Object string            `json:"object"`
			Data   []EmbeddingObject `json:"data"`
			Model  string            `json:"model"`
			Usage  struct {
				PromptTokens int `json:"prompt_tokens"`
				TotalTokens  int `json:"total_tokens"`
			} `json:"usage"`
		}{
			Object: "list",
			Data:   data,
			Model:  modelID,
			Usage: struct {
				PromptTokens int `json:"prompt_tokens"`
				TotalTokens  int `json:"total_tokens"`
			}{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	r.Post("/v1/rerank", func(w http.ResponseWriter, r *http.Request) {
		if reranker == nil {
			http.Error(w, "Reranking functionality is not enabled", http.StatusNotImplemented)
			return
		}

		var req struct {
			Model     string   `json:"model"`
			Query     string   `json:"query"`
			Documents []string `json:"documents"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Query == "" || len(req.Documents) == 0 {
			http.Error(w, "Query and documents are required", http.StatusBadRequest)
			return
		}

		results, err := reranker.Rerank(req.Query, req.Documents)
		if err != nil {
			http.Error(w, fmt.Sprintf("Rerank failed: %v", err), http.StatusInternalServerError)
			return
		}

		resp := struct {
			Model   string                  `json:"model"`
			Results []embedder.RerankResult `json:"results"`
		}{
			Model:   rerankModelID,
			Results: results,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Listening on %s\n", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
