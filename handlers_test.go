package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// MockStorage is a mock implementation of Storage for testing
type MockStorage struct {
	data map[string]string
}

// NewMockStorage creates a new mock storage
func NewMockStorage() *MockStorage {
	return &MockStorage{
		data: make(map[string]string),
	}
}

// Read retrieves note content
func (ms *MockStorage) Read(ctx context.Context, noteID string) (string, error) {
	if content, ok := ms.data[noteID]; ok {
		return content, nil
	}
	return "", nil
}

// Write saves note content
func (ms *MockStorage) Write(ctx context.Context, noteID string, content string) error {
	ms.data[noteID] = content
	return nil
}

// Delete removes a note
func (ms *MockStorage) Delete(ctx context.Context, noteID string) error {
	delete(ms.data, noteID)
	return nil
}

// TestHandleGetEmpty tests GET request for empty note
func TestHandleGetEmpty(t *testing.T) {
	storage := NewMockStorage()
	handler := HandleGet(storage)

	req := httptest.NewRequest("GET", "/?note=", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Expected text/html, got %s", rec.Header().Get("Content-Type"))
	}

	if !bytes.Contains(rec.Body.Bytes(), []byte("<textarea")) {
		t.Errorf("Expected textarea in response")
	}
}

// TestHandleGetExisting tests GET request for existing note
func TestHandleGetExisting(t *testing.T) {
	storage := NewMockStorage()
	_ = storage.Write(context.Background(), "test123", "test content")

	handler := HandleGet(storage)
	req := httptest.NewRequest("GET", "/?note=test123", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !bytes.Contains([]byte(body), []byte("test content")) {
		t.Errorf("Expected note content in response")
	}
}

// TestHandlePostNewNote tests POST request to create new note
func TestHandlePostNewNote(t *testing.T) {
	storage := NewMockStorage()
	handler := HandlePost(storage)

	payload := NoteRequest{
		NoteID:  "",
		Content: "test content",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var resp NoteResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success=true")
	}

	if resp.NoteID == "" {
		t.Errorf("Expected noteId to be generated")
	}

	// Verify content was saved
	if content, _ := storage.Read(context.Background(), resp.NoteID); content != "test content" {
		t.Errorf("Expected content to be saved")
	}
}

// TestHandlePostExisting tests POST request to update existing note
func TestHandlePostExisting(t *testing.T) {
	storage := NewMockStorage()
	handler := HandlePost(storage)

	payload := NoteRequest{
		NoteID:  "test123",
		Content: "updated content",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var resp NoteResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success=true")
	}

	if content, _ := storage.Read(context.Background(), "test123"); content != "updated content" {
		t.Errorf("Expected content to be updated")
	}
}

// TestHandlePostInvalidID tests POST request with invalid note ID
func TestHandlePostInvalidID(t *testing.T) {
	storage := NewMockStorage()
	handler := HandlePost(storage)

	payload := NoteRequest{
		NoteID:  "invalid@id",
		Content: "test content",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rec.Code)
	}

	var resp NoteResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Success {
		t.Errorf("Expected success=false for invalid ID")
	}
}

// TestHandlePostDelete tests POST request with empty content (delete)
func TestHandlePostDelete(t *testing.T) {
	storage := NewMockStorage()
	_ = storage.Write(context.Background(), "test123", "original content")

	handler := HandlePost(storage)

	payload := NoteRequest{
		NoteID:  "test123",
		Content: "",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// Verify note was deleted
	if content, _ := storage.Read(context.Background(), "test123"); content != "" {
		t.Errorf("Expected note to be deleted")
	}
}

// TestExtractPathNoteID tests extracting note ID from path
func TestExtractPathNoteID(t *testing.T) {
	req := httptest.NewRequest("GET", "/noteid/ABC123", nil)
	id := extractPathNoteID(req)
	if id != "ABC123" {
		t.Fatalf("expected ABC123, got %s", id)
	}

	req = httptest.NewRequest("GET", "/app/noteid/XYZ/", nil)
	id = extractPathNoteID(req)
	if id != "XYZ" {
		t.Fatalf("expected XYZ, got %s", id)
	}
}

// TestExtractNoteIDPrefersQuery tests that query param takes precedence over path
func TestExtractNoteIDPrefersQuery(t *testing.T) {
	req := httptest.NewRequest("GET", "/noteid/SHOULDNOT?note=Q123", nil)
	id := extractNoteID(req)
	if id != "Q123" {
		t.Fatalf("expected Q123, got %s", id)
	}
}

// TestParseNoteRequestJSONPath tests parseNoteRequest picks up path-based note ID when JSON omits it
func TestParseNoteRequestJSONPath(t *testing.T) {
	body := `{"content":"hello"}`
	req := httptest.NewRequest("POST", "/noteid/PATHID", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	noteReq, _, err := parseNoteRequest(req, []byte(body), "127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if noteReq.NoteID != "PATHID" {
		t.Fatalf("expected PATHID, got %s", noteReq.NoteID)
	}
	if noteReq.Content != "hello" {
		t.Fatalf("expected content 'hello', got %s", noteReq.Content)
	}
}

// TestParseNoteRequestFormAndRaw tests form parsing and raw body path fallback
func TestParseNoteRequestFormAndRaw(t *testing.T) {
	form := "text=hi&noteId=FORMID"
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	noteReq, _, err := parseNoteRequest(req, []byte(form), "127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if noteReq.NoteID != "FORMID" || noteReq.Content != "hi" {
		t.Fatalf("unexpected form parse result: %#v", noteReq)
	}

	raw := "raw body"
	req = httptest.NewRequest("POST", "/noteid/RAWID", bytes.NewBufferString(raw))
	req.Header.Set("Content-Type", "text/plain")
	noteReq, _, err = parseNoteRequest(req, []byte(raw), "127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if noteReq.NoteID != "RAWID" || noteReq.Content != "raw body" {
		t.Fatalf("unexpected raw parse result: %#v", noteReq)
	}
}
