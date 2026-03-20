package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// resetBlockchain re-initialises the global chain with a fresh genesis block.
func resetBlockchain() {
	mu.Lock()
	defer mu.Unlock()
	genesis := Block{
		Index:     0,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	genesis.Hash = calculateHash(genesis)
	blockchain = []Block{genesis}
}

// ---------------------------------------------------------------------------
// Unit tests for blockchain helpers
// ---------------------------------------------------------------------------

func TestCalculateHash_Deterministic(t *testing.T) {
	b := Block{Index: 1, Timestamp: "2025-01-01T00:00:00Z", Logline: "hello", PrevHash: "abc"}
	h1 := calculateHash(b)
	h2 := calculateHash(b)
	if h1 != h2 {
		t.Fatalf("expected identical hashes, got %s and %s", h1, h2)
	}
}

func TestCalculateHash_DifferentInputs(t *testing.T) {
	b1 := Block{Index: 1, Timestamp: "2025-01-01T00:00:00Z", Logline: "hello", PrevHash: "abc"}
	b2 := Block{Index: 2, Timestamp: "2025-01-01T00:00:00Z", Logline: "hello", PrevHash: "abc"}
	if calculateHash(b1) == calculateHash(b2) {
		t.Fatal("different blocks should produce different hashes")
	}
}

func TestGenerateBlock(t *testing.T) {
	genesis := Block{Index: 0, Timestamp: "2025-01-01T00:00:00Z", Hash: "aaa"}
	b := generateBlock(genesis, "test line")
	if b.Index != 1 {
		t.Fatalf("expected index 1, got %d", b.Index)
	}
	if b.PrevHash != genesis.Hash {
		t.Fatalf("expected PrevHash %s, got %s", genesis.Hash, b.PrevHash)
	}
	if b.Logline != "test line" {
		t.Fatalf("expected logline 'test line', got %s", b.Logline)
	}
	if b.Hash == "" {
		t.Fatal("hash should not be empty")
	}
}

func TestIsBlockValid(t *testing.T) {
	genesis := Block{Index: 0, Timestamp: "2025-01-01T00:00:00Z"}
	genesis.Hash = calculateHash(genesis)

	good := generateBlock(genesis, "valid")
	if !isBlockValid(good, genesis) {
		t.Fatal("expected block to be valid")
	}

	bad := good
	bad.Hash = "tampered"
	if isBlockValid(bad, genesis) {
		t.Fatal("expected tampered block to be invalid")
	}

	badIndex := good
	badIndex.Index = 99
	if isBlockValid(badIndex, genesis) {
		t.Fatal("expected wrong-index block to be invalid")
	}

	badPrev := good
	badPrev.PrevHash = "wrong"
	if isBlockValid(badPrev, genesis) {
		t.Fatal("expected wrong-prevhash block to be invalid")
	}
}

// ---------------------------------------------------------------------------
// Integration tests for HTTP handlers
// ---------------------------------------------------------------------------

func TestHealthCheck(t *testing.T) {
	resetBlockchain()
	router := makeMuxRouter()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}
}

func TestGetBlockchain_ReturnsGenesis(t *testing.T) {
	resetBlockchain()
	router := makeMuxRouter()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var chain []Block
	if err := json.Unmarshal(rr.Body.Bytes(), &chain); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(chain) != 1 {
		t.Fatalf("expected 1 block (genesis), got %d", len(chain))
	}
}

func TestPostBlock_Success(t *testing.T) {
	resetBlockchain()
	router := makeMuxRouter()

	body := bytes.NewBufferString(`{"logline":"test entry"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var b Block
	if err := json.Unmarshal(rr.Body.Bytes(), &b); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if b.Index != 1 {
		t.Fatalf("expected index 1, got %d", b.Index)
	}
	if b.Logline != "test entry" {
		t.Fatalf("expected logline 'test entry', got %s", b.Logline)
	}
}

func TestPostBlock_EmptyLogline(t *testing.T) {
	resetBlockchain()
	router := makeMuxRouter()

	body := bytes.NewBufferString(`{"logline":""}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestPostBlock_InvalidJSON(t *testing.T) {
	resetBlockchain()
	router := makeMuxRouter()

	body := bytes.NewBufferString(`not json`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestPostBlock_ChainGrows(t *testing.T) {
	resetBlockchain()
	router := makeMuxRouter()

	for i := 0; i < 5; i++ {
		body := bytes.NewBufferString(`{"logline":"entry"}`)
		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("iteration %d: expected 201, got %d", i, rr.Code)
		}
	}

	// Verify chain length via GET.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var chain []Block
	if err := json.Unmarshal(rr.Body.Bytes(), &chain); err != nil {
		t.Fatalf("failed to decode chain: %v", err)
	}
	if len(chain) != 6 { // genesis + 5
		t.Fatalf("expected 6 blocks, got %d", len(chain))
	}

	// Verify chain integrity.
	for i := 1; i < len(chain); i++ {
		if !isBlockValid(chain[i], chain[i-1]) {
			t.Fatalf("block %d is invalid", i)
		}
	}
}
