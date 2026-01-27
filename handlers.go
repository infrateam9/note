package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
		
		if noteID != "" {
			log.Printf("[GET] Retrieving note: %s from %s", noteID, r.RemoteAddr)
		} else {
			log.Printf("[GET] Creating new note from %s", r.RemoteAddr)
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

		// Render HTML with note content
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		renderHTML(w, noteID, content, r)
	}
}

// HandlePost handles POST requests to save a note
func HandlePost(storage Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[POST] Request from %s", r.RemoteAddr)
		
		// Set CORS headers to allow requests from any origin
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		// Handle preflight requests
		if r.Method == http.MethodOptions {
			log.Printf("[POST] Preflight OPTIONS request from %s", r.RemoteAddr)
			w.WriteHeader(http.StatusOK)
			return
		}
		
		// Set response content type
		w.Header().Set("Content-Type", "application/json")
		
		// Parse JSON request
		var req NoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[ERROR] Failed to parse JSON from %s: %v", r.RemoteAddr, err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(NoteResponse{
				Success: false,
				Error:   "Invalid request format",
			})
			return
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
			json.NewEncoder(w).Encode(NoteResponse{
				Success: false,
				Error:   "Invalid note ID format",
			})
			return
		}

		// If content is empty, delete the note
		if strings.TrimSpace(req.Content) == "" {
			log.Printf("[DELETE] Attempting to delete note: %s", noteID)
			if err := storage.Delete(r.Context(), noteID); err != nil {
				log.Printf("[ERROR] Failed to delete note %s: %v (Path may not exist)", noteID, err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(NoteResponse{
					Success: false,
					Error:   "Failed to delete note",
				})
				return
			}
			log.Printf("[SUCCESS] Note %s deleted successfully", noteID)
		} else {
			// Save the note
			contentSize := len(req.Content)
			log.Printf("[SAVE] Attempting to save note: %s (size: %d bytes)", noteID, contentSize)
			if err := storage.Write(r.Context(), noteID, req.Content); err != nil {
				log.Printf("[ERROR] Failed to write note %s: %v (Check directory permissions and disk space)", noteID, err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(NoteResponse{
					Success: false,
					Error:   "Failed to save note",
				})
				return
			}
			log.Printf("[SUCCESS] Note %s saved successfully (size: %d bytes)", noteID, contentSize)
		}

		// Return success response
		// Check if this is a curl request (plain text response)
		if isCurlRequest(r) {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "OK: %s\n", noteID)
		} else {
			json.NewEncoder(w).Encode(NoteResponse{
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
                <button onclick="location.href='/'">New Note</button>
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
    <input type="hidden" id="baseUrl" value="` + getBaseURL(r) + `">
    
    <script>
        let baseUrl = document.getElementById('baseUrl').value;
        let lastSaved = "` + EscapeHTML(content) + `";
        let currentNoteId = "` + EscapeHTML(noteID) + `";
        let textarea = document.getElementById("content");
        let statusEl = document.getElementById("status");
        let printableEl = document.getElementById("printable");
        
        // Auto-save functionality
        function autoSave() {
            if (textarea.value !== lastSaved) {
                statusEl.textContent = "Saving...";
                
                fetch("/", {
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
                        if (location.search !== "?note=" + data.noteId && currentNoteId) {
                            window.history.replaceState({}, "", "/?note=" + data.noteId);
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
                alert("Please save the note first");
                return;
            }
            const link = baseUrl + "?note=" + currentNoteId;
            
            // Try modern clipboard API first
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(link).then(() => {
                    statusEl.textContent = "Link copied!";
                    setTimeout(() => { statusEl.textContent = "Ready"; }, 2000);
                }).catch(err => {
                    // Fallback if clipboard API fails
                    copyToClipboardFallback(link);
                });
            } else {
                // Fallback for older browsers or non-secure contexts
                copyToClipboardFallback(link);
            }
        }
        
        // Fallback method to copy text
        function copyToClipboardFallback(text) {
            const textarea = document.createElement("textarea");
            textarea.value = text;
            textarea.style.position = "fixed";
            textarea.style.opacity = "0";
            document.body.appendChild(textarea);
            textarea.select();
            try {
                document.execCommand("copy");
                statusEl.textContent = "Link copied!";
                setTimeout(() => { statusEl.textContent = "Ready"; }, 2000);
            } catch (err) {
                console.error("Failed to copy:", err);
                alert("Failed to copy link. Link: " + text);
            }
            document.body.removeChild(textarea);
        }
        
        // Focus textarea
        textarea.focus();
    </script>
</body>
</html>`

	fmt.Fprint(w, html)
}

// getBaseURL returns the base URL from environment or request
func getBaseURL(r *http.Request) string {
	// Check URL environment variable first
	if url := os.Getenv("URL"); url != "" {
		// Ensure it ends with /
		if url[len(url)-1] != '/' {
			url += "/"
		}
		return url
	}

	// Auto-detect from request
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// Check X-Forwarded-Proto header (for reverse proxies)
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	host := r.Host
	// Check X-Forwarded-Host header (for reverse proxies)
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}

	return scheme + "://" + host + "/"
}
