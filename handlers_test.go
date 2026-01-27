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
	storage.Write(context.Background(), "test123", "test content")

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
	json.NewDecoder(rec.Body).Decode(&resp)

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
	json.NewDecoder(rec.Body).Decode(&resp)

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
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Success {
		t.Errorf("Expected success=false for invalid ID")
	}
}

// TestHandlePostDelete tests POST request with empty content (delete)
func TestHandlePostDelete(t *testing.T) {
	storage := NewMockStorage()
	storage.Write(context.Background(), "test123", "original content")

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
