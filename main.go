package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Build variables - injected at build time
var (
	Version    = "vDev"
	BuildTime  = "timeless"
	CommitHash = "sha-unknown"
)

// Global storage instance
var globalStorage Storage

func init() {
	// Print build information
	log.Printf("Note App - Version: %s, BuildTime: %s, CommitHash: %s", Version, BuildTime, CommitHash)
}

func main() {
	// Detect runtime environment
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		// Lambda mode
		initLambda()
	} else {
		// HTTP server mode
		initHTTPServer()
	}
}

// initLambda initializes Lambda mode with S3 storage
func initLambda() {
	log.Println("Initializing Lambda mode with S3 storage")

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	// Get S3 configuration
	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		log.Fatal("S3_BUCKET environment variable is required")
	}

	s3Prefix := os.Getenv("S3_PREFIX")
	if s3Prefix == "" {
		s3Prefix = "note"
	}

	// Create S3 storage
	s3Client := s3.NewFromConfig(cfg)
	globalStorage = NewS3Storage(s3Client, s3Bucket, s3Prefix)

	log.Printf("S3 storage configured: bucket=%s, prefix=%s", s3Bucket, s3Prefix)

	// Start Lambda handler
	lambda.Start(LambdaHandler)
}

// initHTTPServer initializes HTTP server mode with local storage
func initHTTPServer() {
	log.Println("Initializing HTTP server mode with local disk storage")

	// Get configuration
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	noteDir := os.Getenv("NOTE_DIR")
	if noteDir == "" {
		noteDir = "/note"
	}

	// Create local storage
	var err error
	globalStorage, err = NewLocalStorage(noteDir)
	if err != nil {
		log.Fatalf("Failed to initialize local storage: %v", err)
	}

	log.Printf("Local storage configured: directory=%s", noteDir)

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/favicon.ico", serveFavicon)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			HandleGet(globalStorage)(w, r)
		} else if r.Method == http.MethodPost {
			HandlePost(globalStorage)(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Create server
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Setup graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
	}()

	// Start server
	log.Printf("Starting HTTP server on port %s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
