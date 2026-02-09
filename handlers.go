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
		noteID := extractNoteID(r)
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

// HandlePost handles POST requests to save a note (refactored)
func HandlePost(storage Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientIP := ClientIP(r)
		log.Printf("[POST] Request from %s", clientIP)

		// CORS and preflight handling
		setCORSHeaders(w)
		if r.Method == http.MethodOptions {
			log.Printf("[POST] Preflight OPTIONS request from %s", clientIP)
			w.WriteHeader(http.StatusOK)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// Read body
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("[ERROR] Failed to read request body: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "Read error")
			return
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Parse request
		req, contentType, err := parseNoteRequest(r, bodyBytes, clientIP)
		if err != nil {
			log.Printf("[ERROR] Failed to parse request from %s: %v", clientIP, err)
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Ensure note ID
		noteID := strings.TrimSpace(req.NoteID)
		if noteID == "" {
			noteID = GenerateNoteID()
			log.Printf("[INFO] Generated new note ID: %s", noteID)
		} else {
			log.Printf("[INFO] Using provided note ID: %s", noteID)
		}

		if !ValidateNoteID(noteID) {
			log.Printf("[ERROR] Invalid note ID format: %s", noteID)
			writeJSONError(w, http.StatusBadRequest, "Invalid note ID format")
			return
		}

		// Save or delete
		if strings.TrimSpace(req.Content) == "" {
			log.Printf("[DELETE] Attempting to delete note: %s (Client: %s)", noteID, clientIP)
			if err := storage.Delete(r.Context(), noteID); err != nil {
				log.Printf("[ERROR] Failed to delete note %s: %v", noteID, err)
				writeJSONError(w, http.StatusInternalServerError, "Failed to delete note")
				return
			}
			log.Printf("[SUCCESS] Note %s deleted successfully", noteID)
		} else {
			contentSize := len(req.Content)
			log.Printf("[SAVE] Attempting to save note: %s (size: %d bytes, Client: %s)", noteID, contentSize, clientIP)
			if err := storage.Write(r.Context(), noteID, req.Content); err != nil {
				log.Printf("[ERROR] Failed to write note %s: %v", noteID, err)
				writeJSONError(w, http.StatusInternalServerError, "Failed to save note")
				return
			}
			log.Printf("[SUCCESS] Note %s saved successfully (size: %d bytes)", noteID, contentSize)
		}

		// Return success response
		if isCurlRequest(r) {
			w.Header().Set("Content-Type", "text/plain")
			fullURL := getBaseURL(r) + "noteid/" + noteID
			_, _ = fmt.Fprintln(w, fullURL)
			return
		}

		if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprintf(w, "OK: %s\n", noteID)
			return
		}

		_ = json.NewEncoder(w).Encode(NoteResponse{Success: true, NoteID: noteID})
	}
}

// setCORSHeaders sets common CORS response headers
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// writeJSONError writes a JSON error response
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(NoteResponse{Success: false, Error: message})
}

// parseNoteRequest parses the request body into NoteRequest and returns the content type
func parseNoteRequest(r *http.Request, bodyBytes []byte, clientIP string) (NoteRequest, string, error) {
	var req NoteRequest
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			return req, contentType, fmt.Errorf("invalid JSON format")
		}
		// If JSON didn't include a note ID, try to pick it from the path
		if req.NoteID == "" {
			req.NoteID = extractPathNoteID(r)
		}
		return req, contentType, nil
	}

	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		values, err := url.ParseQuery(string(bodyBytes))
		if err == nil && (values.Has("text") || values.Has("noteId")) {
			req.Content = values.Get("text")
			req.NoteID = values.Get("noteId")
			log.Printf("[INFO] Parsed form data from %s: noteId=%s, content_length=%d", clientIP, req.NoteID, len(req.Content))
			return req, contentType, nil
		}
		req.Content = string(bodyBytes)
		log.Printf("[INFO] Received %d bytes from raw form body from %s", len(bodyBytes), clientIP)
		return req, contentType, nil
	}

	// Plain text or piped binary data
	req.Content = string(bodyBytes)
	// Prefer noteId from query, but fall back to path-based ID (e.g., /noteid/ABCDE)
	req.NoteID = r.URL.Query().Get("noteId")
	if req.NoteID == "" {
		req.NoteID = extractPathNoteID(r)
	}
	log.Printf("[INFO] Received %d bytes from raw request body from %s", len(bodyBytes), clientIP)
	return req, contentType, nil
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
    <link rel="icon" href="/favicon.ico" type="image/x-icon">
    <title>Note</title>
    <style>
        *, *::before, *::after {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        :root {
            --blue-600: #2563EB;
            --blue-700: #1D4ED8;
            --blue-50: #EFF6FF;
            --blue-100: #DBEAFE;
            --white: #FFFFFF;
            --surface: #F8FAFC;
            --border: #E2E8F0;
            --text-primary: #1E293B;
            --text-secondary: #64748B;
            --text-muted: #94A3B8;
            --green-500: #22C55E;
            --red-500: #EF4444;
            --shadow-sm: 0 1px 2px rgba(0,0,0,0.05);
            --shadow-md: 0 4px 6px -1px rgba(0,0,0,0.07), 0 2px 4px -2px rgba(0,0,0,0.05);
            --radius: 8px;
            --radius-lg: 12px;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "Inter", "Roboto", "Helvetica Neue", sans-serif;
            background: var(--white);
            overflow: hidden;
            color: var(--text-primary);
            -webkit-font-smoothing: antialiased;
        }

        .container {
            display: flex;
            flex-direction: column;
            height: 100vh;
            height: 100dvh;
            width: 100%;
        }

        /* ---- Header ---- */
        .header {
            padding: 12px 20px;
            background: var(--white);
            border-bottom: 1px solid var(--border);
            box-shadow: var(--shadow-sm);
            display: flex;
            justify-content: space-between;
            align-items: center;
            flex-shrink: 0;
            z-index: 10;
            gap: 12px;
        }

        .header-left {
            display: flex;
            align-items: center;
            gap: 10px;
            min-width: 0;
        }

        .header h1 {
            font-size: 20px;
            font-weight: 700;
            color: var(--text-primary);
            letter-spacing: -0.025em;
            white-space: nowrap;
        }

        .header h1 .logo-icon {
            color: var(--blue-600);
        }

        .note-id {
            font-size: 12px;
            color: var(--text-muted);
            font-family: "SF Mono", "Monaco", "Menlo", "Consolas", monospace;
            background: var(--blue-50);
            padding: 2px 8px;
            border-radius: 4px;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }

        .note-id:empty {
            display: none;
        }

        /* ---- Buttons ---- */
        .controls {
            display: flex;
            gap: 6px;
            flex-shrink: 0;
        }

        .btn {
            display: inline-flex;
            align-items: center;
            gap: 6px;
            padding: 7px 14px;
            border: 1px solid var(--border);
            background: var(--white);
            color: var(--text-secondary);
            border-radius: 6px;
            cursor: pointer;
            font-size: 13px;
            font-weight: 500;
            white-space: nowrap;
            transition: all 0.15s ease;
            font-family: inherit;
            line-height: 1;
        }

        .btn:hover {
            background: var(--blue-50);
            color: var(--blue-600);
            border-color: var(--blue-600);
        }

        .btn:active {
            transform: scale(0.97);
        }

        .btn-primary {
            background: var(--blue-600);
            color: var(--white);
            border-color: var(--blue-600);
        }

        .btn-primary:hover {
            background: var(--blue-700);
            border-color: var(--blue-700);
            color: var(--white);
        }

        .btn svg {
            width: 14px;
            height: 14px;
            flex-shrink: 0;
        }

        .btn-label {
            display: inline;
        }

        /* ---- Textarea ---- */
        .editor-wrap {
            flex: 1;
            display: flex;
            padding: 12px;
            min-height: 0;
        }

        textarea {
            flex: 1;
            border: 1px solid var(--border);
            border-radius: var(--radius-lg);
            padding: 20px;
            font-family: "SF Mono", "Monaco", "Menlo", "Consolas", "Ubuntu Mono", monospace;
            font-size: 14px;
            line-height: 1.6;
            resize: none;
            overflow: auto;
            background: var(--surface);
            color: var(--text-primary);
            outline: none;
            transition: border-color 0.2s ease, box-shadow 0.2s ease;
        }

        textarea::placeholder {
            color: var(--text-muted);
        }

        textarea:focus {
            border-color: var(--blue-600);
            box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12);
        }

        /* ---- Status Bar ---- */
        .status-bar {
            padding: 8px 20px;
            background: var(--blue-50);
            border-top: 1px solid var(--border);
            font-size: 12px;
            color: var(--text-secondary);
            display: flex;
            justify-content: space-between;
            align-items: center;
            flex-shrink: 0;
            gap: 12px;
        }

        .status-left {
            display: flex;
            align-items: center;
            gap: 6px;
            min-width: 0;
        }

        .status-dot {
            width: 6px;
            height: 6px;
            border-radius: 50%;
            background: var(--text-muted);
            flex-shrink: 0;
            transition: background-color 0.3s ease;
        }

        .status-dot.ready    { background: var(--text-muted); }
        .status-dot.saving   { background: var(--blue-600); }
        .status-dot.saved    { background: var(--green-500); }
        .status-dot.error    { background: var(--red-500); }

        #statusText {
            transition: opacity 0.2s ease;
        }

        .status-right {
            color: var(--text-muted);
            font-variant-numeric: tabular-nums;
            white-space: nowrap;
        }

        /* ---- Printable ---- */
        #printable {
            display: none;
        }

        /* ---- Toast ---- */
        .toast {
            position: fixed;
            bottom: 60px;
            left: 50%;
            transform: translateX(-50%) translateY(20px);
            background: var(--text-primary);
            color: var(--white);
            padding: 8px 18px;
            border-radius: 8px;
            font-size: 13px;
            font-weight: 500;
            pointer-events: none;
            opacity: 0;
            transition: opacity 0.25s ease, transform 0.25s ease;
            z-index: 100;
            box-shadow: var(--shadow-md);
        }

        .toast.show {
            opacity: 1;
            transform: translateX(-50%) translateY(0);
        }

        /* ---- Responsive: Mobile ---- */
        @media (max-width: 640px) {
            .header {
                padding: 10px 14px;
                gap: 8px;
            }

            .header h1 {
                font-size: 17px;
            }

            .note-id {
                display: none;
            }

            .controls {
                gap: 4px;
            }

            .btn {
                padding: 7px 8px;
                border: none;
                background: transparent;
                color: var(--text-secondary);
                border-radius: 6px;
            }

            .btn:hover {
                background: var(--blue-50);
                border: none;
            }

            .btn-primary {
                background: var(--blue-600);
                color: var(--white);
                padding: 7px 10px;
                border-radius: 6px;
            }

            .btn-primary:hover {
                background: var(--blue-700);
            }

            .btn-label {
                display: none;
            }

            .btn svg {
                width: 16px;
                height: 16px;
            }

            .editor-wrap {
                padding: 8px;
            }

            textarea {
                padding: 14px;
                font-size: 15px;
                border-radius: var(--radius);
            }

            .status-bar {
                padding: 7px 14px;
            }
        }

        /* ---- Print ---- */
        @media print {
            .header, .controls, .status-bar, .editor-wrap, .toast {
                display: none !important;
            }

            body, .container {
                height: auto;
                background: white;
                overflow: visible;
            }

            #printable {
                display: block;
                white-space: pre-wrap;
                word-wrap: break-word;
                padding: 0.5in;
                font-family: "Monaco", "Menlo", "Ubuntu Mono", monospace;
                font-size: 11pt;
                line-height: 1.5;
                color: #000;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="header-left">
                <h1><span class="logo-icon">✎</span> Note</h1>
                <span class="note-id" id="noteInfo">` + EscapeHTML(noteID) + `</span>
            </div>
            <div class="controls">
                <button class="btn btn-primary" onclick="newNote()" title="New Note">
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15"/></svg>
                    <span class="btn-label">New</span>
                </button>
                <button class="btn" onclick="copyNoteContent()" title="Copy Content">
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3a2.25 2.25 0 0 0-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9.334a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.416a2.25 2.25 0 0 1-2.25-2.25V6.846c0-1.108.806-2.057 1.907-2.185a48.507 48.507 0 0 1 1.927-.184"/></svg>
                    <span class="btn-label">Copy</span>
                </button>
                <button class="btn" onclick="copyNoteLink()" title="Copy Link">
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m9.86-2.54a4.5 4.5 0 0 0-1.242-7.244l-4.5-4.5a4.5 4.5 0 0 0-6.364 6.364L4.34 8.374" transform="translate(1,1) scale(0.92)"/></svg>
                    <span class="btn-label">Link</span>
                </button>
                <button class="btn" onclick="window.print()" title="Print">
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M6.72 13.829c-.24.03-.48.062-.72.096m.72-.096a42.415 42.415 0 0 1 10.56 0m-10.56 0L6.34 18m10.94-4.171c.24.03.48.062.72.096m-.72-.096L17.66 18m0 0 .229 2.523a1.125 1.125 0 0 1-1.12 1.227H7.231c-.662 0-1.18-.568-1.12-1.227L6.34 18m11.318 0h1.091A2.25 2.25 0 0 0 21 15.75V9.456c0-1.081-.768-2.015-1.837-2.175a48.055 48.055 0 0 0-1.913-.247M6.34 18H5.25A2.25 2.25 0 0 1 3 15.75V9.456c0-1.081.768-2.015 1.837-2.175a48.041 48.041 0 0 1 1.913-.247m10.5 0a48.536 48.536 0 0 0-10.5 0m10.5 0V3.375c0-.621-.504-1.125-1.125-1.125h-8.25c-.621 0-1.125.504-1.125 1.125v3.659M18.75 7.281H5.25"/></svg>
                    <span class="btn-label">Print</span>
                </button>
            </div>
        </div>

        <div class="editor-wrap">
            <textarea id="content" placeholder="Start typing your note...">` + EscapeHTML(content) + `</textarea>
        </div>

        <div class="status-bar">
            <div class="status-left">
                <span class="status-dot ready" id="statusDot"></span>
                <span id="statusText">Ready</span>
            </div>
            <div class="status-right" id="charCount"></div>
        </div>
    </div>

    <div id="printable"></div>
    <div class="toast" id="toast"></div>

    <script>
        const basePath = window.location.pathname.replace(/\/noteid\/.*$/, '');
        const appBase = basePath.endsWith('/') ? basePath : basePath + '/';
        let lastSaved = ` + "`" + EscapeHTML(content) + "`" + `;
        let currentNoteId = "` + EscapeHTML(noteID) + `";
        const textarea = document.getElementById("content");
        const statusText = document.getElementById("statusText");
        const statusDot = document.getElementById("statusDot");
        const charCountEl = document.getElementById("charCount");
        const printableEl = document.getElementById("printable");
        const toastEl = document.getElementById("toast");

        function setStatus(text, state) {
            statusText.textContent = text;
            statusDot.className = 'status-dot ' + (state || 'ready');
        }

        function updateCharCount() {
            const len = textarea.value.length;
            const words = textarea.value.trim() ? textarea.value.trim().split(/\s+/).length : 0;
            charCountEl.textContent = len > 0 ? words + ' word' + (words !== 1 ? 's' : '') + ' \u00b7 ' + len.toLocaleString() + ' char' + (len !== 1 ? 's' : '') : '';
        }

        function showToast(msg) {
            toastEl.textContent = msg;
            toastEl.classList.add('show');
            setTimeout(function() { toastEl.classList.remove('show'); }, 2000);
        }

        function newNote() {
            window.location.href = appBase;
        }

        // Auto-save
        function autoSave() {
            if (textarea.value !== lastSaved) {
                setStatus('Saving...', 'saving');

                const saveUrl = currentNoteId ? appBase + 'noteid/' + currentNoteId : appBase;
                fetch(saveUrl, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ noteId: currentNoteId, content: textarea.value })
                })
                .then(function(response) {
                    if (!response.ok) throw new Error('HTTP ' + response.status + ': ' + response.statusText);
                    return response.json();
                })
                .then(function(data) {
                    if (data.success) {
                        lastSaved = textarea.value;
                        currentNoteId = data.noteId;

                        var newPath = appBase + 'noteid/' + data.noteId;
                        if (window.location.pathname !== newPath && currentNoteId) {
                            window.history.replaceState({}, '', newPath);
                            document.getElementById('noteInfo').textContent = data.noteId;
                        }

                        setStatus('Saved', 'saved');
                        setTimeout(function() {
                            if (statusText.textContent === 'Saved') setStatus('Ready', 'ready');
                        }, 2000);
                    } else {
                        setStatus('Error: ' + (data.error || 'Save failed'), 'error');
                    }
                })
                .catch(function(err) {
                    console.error('Save error:', err);
                    setStatus('Error: ' + (err.message || 'Network error'), 'error');
                });
            }
        }

        setInterval(autoSave, 1000);

        // TAB key
        textarea.addEventListener('keydown', function(e) {
            if (e.key === 'Tab') {
                e.preventDefault();
                var start = this.selectionStart;
                var end = this.selectionEnd;
                this.value = this.value.substring(0, start) + '\t' + this.value.substring(end);
                this.selectionStart = this.selectionEnd = start + 1;
            }
        });

        textarea.addEventListener('input', function() {
            printableEl.textContent = this.value;
            updateCharCount();
        });

        printableEl.textContent = textarea.value;
        updateCharCount();

        function copyToClipboard(text) {
            if (navigator.clipboard && navigator.clipboard.writeText) {
                return navigator.clipboard.writeText(text).then(function() {
                    return true;
                }).catch(function() {
                    return fallbackCopy(text);
                });
            }
            return Promise.resolve(fallbackCopy(text));
        }

        function fallbackCopy(text) {
            var ta = document.createElement('textarea');
            ta.value = text;
            ta.style.position = 'fixed';
            ta.style.left = '-9999px';
            document.body.appendChild(ta);
            ta.select();
            var ok = false;
            try { ok = document.execCommand('copy'); } catch(e) {}
            document.body.removeChild(ta);
            return ok;
        }

        function copyNoteLink() {
            if (!currentNoteId) { showToast('Save a note first'); return; }
            var link = window.location.origin + appBase + 'noteid/' + currentNoteId;
            copyToClipboard(link).then(function(ok) {
                showToast(ok ? 'Link copied!' : 'Could not copy link');
            });
        }

        function copyNoteContent() {
            var text = textarea.value;
            if (!text) { showToast('Nothing to copy'); return; }
            copyToClipboard(text).then(function(ok) {
                showToast(ok ? 'Content copied!' : 'Could not copy content');
            });
        }

        textarea.focus();
    </script>
</body>
</html>`

	_, _ = fmt.Fprint(w, html)
}

// extractPathNoteID extracts a note ID from a path of the form /.../noteid/{id}
func extractPathNoteID(r *http.Request) string {
	path := r.URL.Path
	if idx := strings.Index(path, "/noteid/"); idx != -1 {
		id := path[idx+len("/noteid/"):]
		// strip any trailing slash
		id = strings.Trim(id, "/")
		return id
	}
	return ""
}

// extractNoteID returns the note id either from query (?note=) or from /noteid/{id}
func extractNoteID(r *http.Request) string {
	if id := r.URL.Query().Get("note"); id != "" {
		return id
	}
	return extractPathNoteID(r)
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
	// Remove any trailing /noteid/{id} from the path to get the app root (supports reverse proxy subpaths)
	path := r.URL.Path
	if idx := strings.Index(path, "/noteid/"); idx != -1 {
		path = path[:idx]
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return scheme + "://" + host + path
}
