package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/olivere/elastic/v7"
)

// Block represents a single entry in the blockchain. Each block contains a log
// line, a SHA-256 hash of its contents, and a reference to the previous block's
// hash so that the chain's integrity can be verified.
type Block struct {
	Index     int    `json:"index"`
	Timestamp string `json:"timestamp"`
	Logline   string `json:"logline"`
	Hash      string `json:"hash"`
	PrevHash  string `json:"prevHash"`
}

// Message is the JSON payload accepted by the POST endpoint.
type Message struct {
	Logline string `json:"logline"`
}

// blockchain holds the in-memory chain and a mutex to protect concurrent access.
var (
	blockchain []Block
	mu         sync.Mutex
)

// esClient is the optional Elasticsearch client. When nil, blocks are still
// stored in the in-memory chain but not indexed externally.
var esClient *elastic.Client

func main() {
	// .env is optional — environment variables can be set directly.
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading configuration from environment")
	}

	initElasticsearch()

	// Create the genesis block before the server starts accepting requests.
	genesis := Block{
		Index:     0,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	genesis.Hash = calculateHash(genesis)
	blockchain = append(blockchain, genesis)
	log.Printf("Genesis block created: %s", genesis.Hash)

	if err := run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// initElasticsearch creates the Elasticsearch client if ELASTICSEARCH_URL is
// set. When the variable is empty or the connection fails, Elasticsearch
// indexing is silently disabled so the service can still run standalone.
func initElasticsearch() {
	esURL := os.Getenv("ELASTICSEARCH_URL")
	if esURL == "" {
		log.Println("ELASTICSEARCH_URL not set — Elasticsearch indexing disabled")
		return
	}

	client, err := elastic.NewClient(elastic.SetURL(esURL), elastic.SetSniff(false))
	if err != nil {
		log.Printf("Failed to connect to Elasticsearch at %s: %v (indexing disabled)", esURL, err)
		return
	}
	esClient = client
	log.Printf("Connected to Elasticsearch at %s", esURL)
}

// ---------------------------------------------------------------------------
// Blockchain helpers
// ---------------------------------------------------------------------------

// calculateHash returns the SHA-256 hex digest of the block's deterministic
// fields (index, timestamp, logline, and previous hash).
func calculateHash(block Block) string {
	record := strconv.Itoa(block.Index) + block.Timestamp + block.Logline + block.PrevHash
	h := sha256.New()
	h.Write([]byte(record))
	return hex.EncodeToString(h.Sum(nil))
}

// generateBlock creates the next block in the chain using the provided log line.
func generateBlock(oldBlock Block, logline string) Block {
	newBlock := Block{
		Index:     oldBlock.Index + 1,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Logline:   logline,
		PrevHash:  oldBlock.Hash,
	}
	newBlock.Hash = calculateHash(newBlock)
	return newBlock
}

// isBlockValid checks that newBlock correctly follows oldBlock.
func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index {
		return false
	}
	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}
	if calculateHash(newBlock) != newBlock.Hash {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// HTTP server
// ---------------------------------------------------------------------------

// run starts the HTTP server on the port specified by the ADDR environment
// variable (defaults to 8080).
func run() error {
	router := makeMuxRouter()

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "8080"
	}

	s := &http.Server{
		Addr:           ":" + addr,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Printf("Listening on :%s", addr)
	return s.ListenAndServe()
}

func makeMuxRouter() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/", handleGetBlockchain).Methods("GET")
	r.HandleFunc("/", handleWriteBlock).Methods("POST")
	r.HandleFunc("/healthz", handleHealthCheck).Methods("GET")
	return r
}

// handleHealthCheck returns a simple 200 OK so load balancers and container
// orchestrators can verify the service is alive.
func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleGetBlockchain returns the full blockchain as a JSON array.
func handleGetBlockchain(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	chain := make([]Block, len(blockchain))
	copy(chain, blockchain)
	mu.Unlock()

	respondWithJSON(w, http.StatusOK, chain)
}

// handleWriteBlock accepts a JSON body with a "logline" field, appends a new
// block to the chain, and optionally indexes it in Elasticsearch.
func handleWriteBlock(w http.ResponseWriter, r *http.Request) {
	var m Message
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		respondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("invalid JSON: %v", err),
		})
		return
	}
	defer r.Body.Close()

	if m.Logline == "" {
		respondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "logline must not be empty",
		})
		return
	}

	mu.Lock()
	prevBlock := blockchain[len(blockchain)-1]
	newBlock := generateBlock(prevBlock, m.Logline)

	if !isBlockValid(newBlock, prevBlock) {
		mu.Unlock()
		respondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "generated block failed validation",
		})
		return
	}

	blockchain = append(blockchain, newBlock)
	mu.Unlock()

	// Index asynchronously so the HTTP response is not delayed by Elasticsearch.
	go writeToElasticsearch(newBlock)

	respondWithJSON(w, http.StatusCreated, newBlock)
}

// respondWithJSON marshals the payload and writes it with the correct
// Content-Type header.
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	resp, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(resp)
}

// writeToElasticsearch indexes a block if an Elasticsearch client is available.
func writeToElasticsearch(block Block) {
	if esClient == nil {
		return
	}
	_, err := esClient.Index().
		Index("loggos").
		BodyJson(block).
		Do(context.Background())
	if err != nil {
		log.Printf("Elasticsearch indexing error: %v", err)
	}
}
