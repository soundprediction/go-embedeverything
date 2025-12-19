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
	modelID      string
	architecture string
	port         int
)

var rootCmd = &cobra.Command{
	Use:   "embed-server",
	Short: "A REST server for generating embeddings",
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
	serveCmd.Flags().StringVar(&modelID, "model", "Qwen/Qwen3-Embedding-0.6B", "HuggingFace model ID")
	serveCmd.Flags().StringVar(&architecture, "arch", "Bert", "Model architecture (Bert, etc.)")
	serveCmd.Flags().IntVar(&port, "port", 8080, "Port to listen on")
}

func startServer() {
	fmt.Printf("Loading model %s (arch: %s)...\n", modelID, architecture)
	emb, err := embedder.NewEmbedder(modelID, architecture)
	if err != nil {
		log.Fatalf("Failed to initialize embedder: %v", err)
	}
	defer emb.Close()
	fmt.Println("Model loaded successfully.")

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/v1/embeddings", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input string `json:"input"`
			Model string `json:"model"` // Optional, ignored for now as we load one model
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Input == "" {
			http.Error(w, "Input text is required", http.StatusBadRequest)
			return
		}

		vec, err := emb.Embed(req.Input)
		if err != nil {
			http.Error(w, fmt.Sprintf("Embedding failed: %v", err), http.StatusInternalServerError)
			return
		}

		resp := struct {
			Object string `json:"object"`
			Data   []struct {
				Object    string    `json:"object"`
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			} `json:"data"`
			Model string `json:"model"`
			Usage struct {
				PromptTokens int `json:"prompt_tokens"`
				TotalTokens  int `json:"total_tokens"`
			} `json:"usage"`
		}{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{
					Object:    "embedding",
					Embedding: vec,
					Index:     0,
				},
			},
			Model: modelID,
			Usage: struct {
				PromptTokens int `json:"prompt_tokens"`
				TotalTokens  int `json:"total_tokens"`
			}{
				PromptTokens: 0, // Placeholder
				TotalTokens:  0, // Placeholder
			},
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
