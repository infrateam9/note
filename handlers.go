package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// NoteRequest represents the JSON payload for saving a note
type NoteRequest struct {
	NoteID  string `json:"noteId"`
	Content string `json:"content"`
}

// NoteResponse represents the JSON response
type NoteResponse struct {
	Success bool   `json:"success"`
	NoteID  string `json:"noteId,omitempty"`
	Error   string `json:"error,omitempty"`
}

// HandleGet handles GET requests to retrieve a note
func HandleGet(storage Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		noteID := r.URL.Query().Get("note")
		clientIP := ClientIP(r)

		if noteID != "" {
			log.Printf("[GET] Retrieving note: %s from %s", noteID, clientIP)
		} else {
			// Don't log for local requests, they are not interesting.
			if clientIP != "127.0.0.1" && clientIP != "::1" {
				log.Printf("[GET] Creating new note from %s", clientIP)
			}
		}

		// Read note content from storage
		content := ""
		if noteID != "" {
			var err error
			content, err = storage.Read(r.Context(), noteID)
			if err != nil {
				log.Printf("[ERROR] Failed to read note %s: %v", noteID, err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			log.Printf("[SUCCESS] Note %s retrieved successfully", noteID)
		}

		// If the client is curl and a note ID was requested, return raw text
		if isCurlRequest(r) && noteID != "" {
			if content == "" {
				http.Error(w, "Note not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = fmt.Fprint(w, content)
			return
		}

		// Render HTML with note content
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		renderHTML(w, noteID, content, r)
	}
}

// HandlePost handles POST requests to save a note
func HandlePost(storage Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientIP := ClientIP(r)
		log.Printf("[POST] Request from %s", clientIP)

		// Set CORS headers to allow requests from any origin
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			log.Printf("[POST] Preflight OPTIONS request from %s", clientIP)
			w.WriteHeader(http.StatusOK)
			return
		}

		// Set response content type
		w.Header().Set("Content-Type", "application/json")

		// Read the entire body first to avoid ParseForm consuming it early
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("[ERROR] Failed to read request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(NoteResponse{
				Success: false,
				Error:   "Read error",
			})
			return
		}

		// Reset body reader for potential future use (though not needed here as we use bodyBytes)
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		var req NoteRequest
		contentType := r.Header.Get("Content-Type")

		if strings.Contains(contentType, "application/json") {
			// Parse JSON request
			if err := json.Unmarshal(bodyBytes, &req); err != nil {
				log.Printf("[ERROR] Failed to parse JSON from %s: %v", clientIP, err)
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(NoteResponse{
					Success: false,
					Error:   "Invalid JSON format",
				})
				return
			}
		} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			// Try to parse as form, fallback to raw if keys are missing
			values, err := url.ParseQuery(string(bodyBytes))
			if err == nil && (values.Has("text") || values.Has("noteId")) {
				req.Content = values.Get("text")
				req.NoteID = values.Get("noteId")
				log.Printf("[INFO] Parsed form data from %s: noteId=%s, content_length=%d", clientIP, req.NoteID, len(req.Content))
			} else {
				req.Content = string(bodyBytes)
				log.Printf("[INFO] Received %d bytes from raw form body from %s", len(bodyBytes), clientIP)
			}
		} else {
			// Plain text or piped binary data
			req.Content = string(bodyBytes)
			req.NoteID = r.URL.Query().Get("noteId")
			log.Printf("[INFO] Received %d bytes from raw request body from %s", len(bodyBytes), clientIP)
		}

		// Auto-generate ID if not provided
		noteID := strings.TrimSpace(req.NoteID)
		if noteID == "" {
			noteID = GenerateNoteID()
			log.Printf("[INFO] Generated new note ID: %s", noteID)
		} else {
			log.Printf("[INFO] Using provided note ID: %s", noteID)
		}

		// Validate note ID
		if !ValidateNoteID(noteID) {
			log.Printf("[ERROR] Invalid note ID format: %s", noteID)
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(NoteResponse{
				Success: false,
				Error:   "Invalid note ID format",
			})
			return
		}

		// If content is empty, delete the note
		if strings.TrimSpace(req.Content) == "" {
			log.Printf("[DELETE] Attempting to delete note: %s (Client: %s)", noteID, clientIP)
			if err := storage.Delete(r.Context(), noteID); err != nil {
				log.Printf("[ERROR] Failed to delete note %s: %v", noteID, err)
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(NoteResponse{
					Success: false,
					Error:   "Failed to delete note",
				})
				return
			}
			log.Printf("[SUCCESS] Note %s deleted successfully", noteID)
		} else {
			// Save the note
			contentSize := len(req.Content)
			log.Printf("[SAVE] Attempting to save note: %s (size: %d bytes, Client: %s)", noteID, contentSize, clientIP)
			if err := storage.Write(r.Context(), noteID, req.Content); err != nil {
				log.Printf("[ERROR] Failed to write note %s: %v", noteID, err)
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(NoteResponse{
					Success: false,
					Error:   "Failed to save note",
				})
				return
			}
			log.Printf("[SUCCESS] Note %s saved successfully (size: %d bytes)", noteID, contentSize)
		}

		// Return success response
		if isCurlRequest(r) {
			// For terminal users, return the full URL for easy piping
			w.Header().Set("Content-Type", "text/plain")
			fullURL := getBaseURL(r) + "?note=" + noteID
			_, _ = fmt.Fprintln(w, fullURL)
		} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprintf(w, "OK: %s\n", noteID)
		} else {
			_ = json.NewEncoder(w).Encode(NoteResponse{
				Success: true,
				NoteID:  noteID,
			})
		}
	}
}

// isCurlRequest checks if the request is from curl
func isCurlRequest(r *http.Request) bool {
	userAgent := r.Header.Get("User-Agent")
	return strings.Contains(strings.ToLower(userAgent), "curl")
}

// renderHTML renders the main HTML template with note content
func renderHTML(w http.ResponseWriter, noteID string, content string, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <link rel="icon" href="favicon.ico" type="image/x-icon">
    <title>Note</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "Roboto", "Oxygen", "Ubuntu", "Cantarell", "Fira Sans", "Droid Sans", "Helvetica Neue", sans-serif;
            background: #f5f5f5;
            overflow: hidden;
        }
        
        .container {
            display: flex;
            flex-direction: column;
            height: 100vh;
            width: 100%;
        }
        
        .header {
            padding: 20px;
            background: #fff;
            border-bottom: 1px solid #ddd;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        
        .header h1 {
            font-size: 24px;
            color: #333;
        }
        
        .note-id {
            font-size: 14px;
            color: #666;
            font-family: monospace;
        }
        
        .controls {
            display: flex;
            gap: 10px;
        }
        
        button {
            padding: 8px 16px;
            border: none;
            background: #007bff;
            color: white;
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
        }
        
        button:hover {
            background: #0056b3;
        }
        
        textarea {
            flex: 1;
            border: none;
            padding: 20px;
            font-family: "Monaco", "Menlo", "Ubuntu Mono", monospace;
            font-size: 14px;
            resize: none;
            overflow: auto;
        }
        
        .status {
            padding: 10px 20px;
            background: #f0f0f0;
            border-top: 1px solid #ddd;
            font-size: 12px;
            color: #666;
        }
        
        #printable {
            display: none;
        }
        
        @media print {
            .header, .controls, .status {
                display: none;
            }
            
            body, .container {
                height: auto;
                background: white;
            }
            
            textarea {
                display: none;
            }
            
            #printable {
                display: block;
                white-space: pre-wrap;
                word-wrap: break-word;
                padding: 20px;
                font-family: "Monaco", "Menlo", "Ubuntu Mono", monospace;
                font-size: 12pt;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div>
                <h1>Note</h1>
                <div class="note-id" id="noteInfo">` + EscapeHTML(noteID) + `</div>
            </div>
            <div class="controls">
                <button onclick="window.location.href=window.location.pathname">New Note</button>
                <button onclick="copyNoteContent()">Copy Content</button>
                <button onclick="copyNoteLink()">Copy Link</button>
                <button onclick="window.print()">Print</button>
            </div>
        </div>
        <textarea id="content" placeholder="Start typing...">` + EscapeHTML(content) + `</textarea>
        <div class="status">
            <span id="status">Ready</span>
        </div>
    </div>
    
    <div id="printable"></div>
    
    <script>
        const appPath = window.location.pathname;
        let lastSaved = ` + "`" + EscapeHTML(content) + "`" + `;
        let currentNoteId = "` + EscapeHTML(noteID) + `";
        const textarea = document.getElementById("content");
        const statusEl = document.getElementById("status");
        const printableEl = document.getElementById("printable");
        
        // Auto-save functionality
        function autoSave() {
            if (textarea.value !== lastSaved) {
                statusEl.textContent = "Saving...";
                
                fetch(appPath + window.location.search, {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json"
                    },
                    body: JSON.stringify({
                        noteId: currentNoteId,
                        content: textarea.value
                    })
                })
                .then(response => {
                    if (!response.ok) {
                        throw new Error("HTTP " + response.status + ": " + response.statusText);
                    }
                    return response.json();
                })
                .then(data => {
                    if (data.success) {
                        lastSaved = textarea.value;
                        currentNoteId = data.noteId;
                        
                        // Update URL if new note was created
                        const newSearch = "?note=" + data.noteId;
                        if (window.location.search !== newSearch && currentNoteId) {
                            window.history.replaceState({}, "", appPath + newSearch);
                            document.getElementById("noteInfo").textContent = data.noteId;
                        }
                        
                        statusEl.textContent = "Saved";
                        setTimeout(() => {
                            if (statusEl.textContent === "Saved") {
                                statusEl.textContent = "Ready";
                            }
                        }, 2000);
                    } else {
                        statusEl.textContent = "Error: " + (data.error || "Save failed");
                    }
                })
                .catch(err => {
                    console.error("Save error:", err);
                    statusEl.textContent = "Error: " + (err.message || "Network error");
                });
            }
        }
        
        // Auto-save every 1 second
        setInterval(autoSave, 1000);
        
        // TAB key handling - insert tab instead of moving focus
        textarea.addEventListener("keydown", function(e) {
            if (e.key === "Tab") {
                e.preventDefault();
                const start = this.selectionStart;
                const end = this.selectionEnd;
                this.value = this.value.substring(0, start) + "\t" + this.value.substring(end);
                this.selectionStart = this.selectionEnd = start + 1;
            }
        });
        
        // Update print preview
        textarea.addEventListener("input", function() {
            printableEl.textContent = this.value;
        });
        
        // Initialize print preview
        printableEl.textContent = textarea.value;
        
        // Copy note link to clipboard
        function copyNoteLink() {
            if (!currentNoteId) {
                return;
            }
            const link = window.location.origin + appPath + "?note=" + currentNoteId;
            
            // Try modern clipboard API first
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(link).then(() => {
                    statusEl.textContent = "Link copied!";
                    setTimeout(() => { statusEl.textContent = "Ready"; }, 2000);
                });
            }
        }

        // Copy note content to clipboard
        function copyNoteContent() {
            const text = textarea.value;
            if (!text) return;

            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(text).then(() => {
                    statusEl.textContent = "Content copied!";
                    setTimeout(() => { statusEl.textContent = "Ready"; }, 2000);
                }).catch(err => {
                    console.error("Failed to copy content:", err);
                });
            }
        }
        
        // Focus textarea
        textarea.focus();
    </script>
</body>
</html>`

	_, _ = fmt.Fprint(w, html)
}

func getBaseURL(r *http.Request) string {
	if urlEnv := os.Getenv("URL"); urlEnv != "" {
		if !strings.HasSuffix(urlEnv, "/") {
			urlEnv += "/"
		}
		return urlEnv
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}
	return scheme + "://" + host + r.URL.Path
}
