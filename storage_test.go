package main

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorageRead(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Write test file
	testContent := "test content"
	filePath := filepath.Join(tmpDir, "test123")
	if err := ioutil.WriteFile(filePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create storage
	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	// Test read
	content, err := storage.Read(context.Background(), "test123")
	if err != nil {
		t.Fatalf("Failed to read note: %v", err)
	}

	if content != testContent {
		t.Errorf("Expected %s, got %s", testContent, content)
	}
}

func TestLocalStorageReadMissing(t *testing.T) {
	tmpDir := t.TempDir()

	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	// Test read non-existent note
	content, err := storage.Read(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Expected nil error for missing note, got %v", err)
	}

	if content != "" {
		t.Errorf("Expected empty string for missing note, got %s", content)
	}
}

func TestLocalStorageWrite(t *testing.T) {
	tmpDir := t.TempDir()

	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	testContent := "test content"

	// Test write
	err = storage.Write(context.Background(), "test123", testContent)
	if err != nil {
		t.Fatalf("Failed to write note: %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(tmpDir, "test123")
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("Note file not created: %v", err)
	}

	// Read and verify content
	content, _ := ioutil.ReadFile(filePath)
	if string(content) != testContent {
		t.Errorf("Expected %s, got %s", testContent, string(content))
	}
}

func TestLocalStorageDelete(t *testing.T) {
	tmpDir := t.TempDir()

	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	// Create test file
	filePath := filepath.Join(tmpDir, "test123")
	if err := ioutil.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test delete
	err = storage.Delete(context.Background(), "test123")
	if err != nil {
		t.Fatalf("Failed to delete note: %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(filePath); err == nil {
		t.Errorf("Note file still exists after deletion")
	}
}

func TestLocalStorageCreateDir(t *testing.T) {
	tmpDir := t.TempDir()
	noteDir := filepath.Join(tmpDir, "note")

	// Verify directory doesn't exist
	if _, err := os.Stat(noteDir); err == nil {
		t.Fatalf("Directory should not exist yet")
	}

	// Create storage (should create directory)
	storage, err := NewLocalStorage(noteDir)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(noteDir); err != nil {
		t.Errorf("Directory was not created: %v", err)
	}

	// Verify we can write to it
	err = storage.Write(context.Background(), "test", "content")
	if err != nil {
		t.Fatalf("Failed to write to created directory: %v", err)
	}
}
