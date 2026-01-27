package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
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
		return nil, fmt.Errorf("failed to create notes directory: %w", err)
	}
	return &LocalStorage{dir: dir}, nil
}

// Read retrieves note content from disk
func (ls *LocalStorage) Read(ctx context.Context, noteID string) (string, error) {
	filePath := filepath.Join(ls.dir, noteID)
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil // Return empty string for missing notes
		}
		return "", fmt.Errorf("failed to read note: %w", err)
	}
	return string(content), nil
}

// Write saves note content to disk
func (ls *LocalStorage) Write(ctx context.Context, noteID string, content string) error {
	filePath := filepath.Join(ls.dir, noteID)
	if err := ioutil.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write note: %w", err)
	}
	return nil
}

// Delete removes a note from disk
func (ls *LocalStorage) Delete(ctx context.Context, noteID string) error {
	filePath := filepath.Join(ls.dir, noteID)
	if err := os.Remove(filePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // Silently ignore if already deleted
		}
		return fmt.Errorf("failed to delete note: %w", err)
	}
	return nil
}
