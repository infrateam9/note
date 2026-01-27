package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

// Storage defines the interface for note storage
type Storage interface {
	Read(ctx context.Context, noteID string) (string, error)
	Write(ctx context.Context, noteID string, content string) error
	Delete(ctx context.Context, noteID string) error
}

// LocalStorage implements Storage using the local filesystem
type LocalStorage struct {
	dir string
}

// NewLocalStorage creates a new LocalStorage instance
func NewLocalStorage(dir string) (*LocalStorage, error) {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[ERROR] Failed to create notes directory %s: %v", dir, err)
		return nil, fmt.Errorf("failed to create notes directory: %w", err)
	}
	log.Printf("[INFO] LocalStorage initialized at: %s", dir)
	return &LocalStorage{dir: dir}, nil
}

// Read retrieves note content from disk
func (ls *LocalStorage) Read(ctx context.Context, noteID string) (string, error) {
	filePath := filepath.Join(ls.dir, noteID)
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("[INFO] Note %s does not exist at %s", noteID, filePath)
			return "", nil // Return empty string for missing notes
		}
		log.Printf("[ERROR] Failed to read note %s from %s: %v", noteID, filePath, err)
		return "", fmt.Errorf("failed to read note: %w", err)
	}
	log.Printf("[DEBUG] Note %s read successfully from %s (%d bytes)", noteID, filePath, len(content))
	return string(content), nil
}

// Write saves note content to disk
func (ls *LocalStorage) Write(ctx context.Context, noteID string, content string) error {
	filePath := filepath.Join(ls.dir, noteID)
	if err := ioutil.WriteFile(filePath, []byte(content), 0644); err != nil {
		log.Printf("[ERROR] Failed to write note %s to %s: %v (Check directory permissions: %s, Disk space, File permissions)", noteID, filePath, err, ls.dir)
		return fmt.Errorf("failed to write note: %w", err)
	}
	log.Printf("[DEBUG] Note %s written successfully to %s (%d bytes)", noteID, filePath, len(content))
	return nil
}

// Delete removes a note from disk
func (ls *LocalStorage) Delete(ctx context.Context, noteID string) error {
	filePath := filepath.Join(ls.dir, noteID)
	if err := os.Remove(filePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("[INFO] Note %s does not exist at %s, nothing to delete", noteID, filePath)
			return nil // Silently ignore if already deleted
		}
		log.Printf("[ERROR] Failed to delete note %s from %s: %v (Check file permissions)", noteID, filePath, err)
		return fmt.Errorf("failed to delete note: %w", err)
	}
	log.Printf("[DEBUG] Note %s deleted successfully from %s", noteID, filePath)
	return nil
}
