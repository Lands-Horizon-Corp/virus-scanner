package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	htmlContent        []byte
	maxConcurrentScans = runtime.NumCPU() * 2 // Double CPU count for better throughput
	scanSemaphore      = make(chan struct{}, maxConcurrentScans)

	// Allowed CORS origins
	allowedOrigins = map[string]bool{
		// Primary production domains
		"https://ecoop-suite.netlify.app": true,
		"https://ecoop-suite.com":         true,
		"https://www.ecoop-suite.com":     true,

		// Development and staging environments
		"https://development.ecoop-suite.com":     true,
		"https://www.development.ecoop-suite.com": true,
		"https://staging.ecoop-suite.com":         true,
		"https://www.staging.ecoop-suite.com":     true,

		// Fly.io deployment domains
		"https://cooperatives-development.fly.dev": true,
		"https://cooperatives-staging.fly.dev":     true,
		"https://cooperatives-production.fly.dev":  true,

		// Local development
		"http://localhost:3000": true,
		"http://localhost:3001": true,
		"http://localhost:8080": true,
		"http://localhost:8081": true,
	}

	// Object pools for memory efficiency
	bufferPool = sync.Pool{
		New: func() any {
			buf := make([]byte, 64*1024) // 64KB buffers
			return &buf
		},
	}

	bytesReaderPool = sync.Pool{
		New: func() any {
			return &bytes.Reader{}
		},
	}

	stringsBuilderPool = sync.Pool{
		New: func() any {
			return &strings.Builder{}
		},
	}
)

func uiHandler(ctx *fasthttp.RequestCtx) {
	setCORSHeaders(ctx)
	ctx.SetContentType("text/html")
	ctx.SetBody(htmlContent)
}

// CORS middleware
func setCORSHeaders(ctx *fasthttp.RequestCtx) {
	origin := string(ctx.Request.Header.Peek("Origin"))

	// Check if origin is allowed
	if allowedOrigins[origin] {
		ctx.Response.Header.Set("Access-Control-Allow-Origin", origin)
	}

	// Set other CORS headers
	ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	ctx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
	ctx.Response.Header.Set("Access-Control-Max-Age", "86400") // 24 hours
}

func handleOptions(ctx *fasthttp.RequestCtx) {
	setCORSHeaders(ctx)
	ctx.SetStatusCode(fasthttp.StatusOK)
}

func scanHandler(ctx *fasthttp.RequestCtx) {
	setCORSHeaders(ctx)

	if !ctx.IsPost() {
		ctx.Error("Only POST allowed", fasthttp.StatusMethodNotAllowed)
		return
	}
	contentType := string(ctx.Request.Header.ContentType())
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		ctx.Error("Invalid multipart request", fasthttp.StatusBadRequest)
		return
	}
	boundary := params["boundary"]
	if boundary == "" {
		ctx.Error("Missing boundary", fasthttp.StatusBadRequest)
		return
	}
	body := ctx.PostBody()
	filename, fileReader, err := extractFileStreamOptimized(body, boundary)
	if err != nil {
		ctx.Error("Failed to parse file: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}
	result, err := scanStreamOptimized(fileReader, filename)
	if err != nil {
		ctx.Error("Scan error: "+err.Error(), fasthttp.StatusInternalServerError)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	ctx.SetBodyString(result)
}

// Optimized multipart parsing with streaming
func extractFileStreamOptimized(body []byte, boundary string) (filename string, reader io.Reader, err error) {
	bytesReader := bytesReaderPool.Get().(*bytes.Reader)
	defer bytesReaderPool.Put(bytesReader)
	bytesReader.Reset(body)
	multipartReader := multipart.NewReader(bytesReader, boundary)
	for {
		part, err := multipartReader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, err
		}
		if part.FormName() == "file" {
			filename = part.FileName()
			if filename == "" {
				filename = "unknown"
			}
			return filename, part, nil
		}
		part.Close()
	}
	return "", nil, fmt.Errorf("no file found in multipart data")
}

func scanStreamOptimized(r io.Reader, filename string) (string, error) {
	scanSemaphore <- struct{}{}
	defer func() { <-scanSemaphore }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	status, threat, err := scanWithClamscanOptimized(ctx, r)
	if err != nil {
		return "", err
	}
	sb := stringsBuilderPool.Get().(*strings.Builder)
	defer func() {
		sb.Reset()
		stringsBuilderPool.Put(sb)
	}()
	sb.WriteString(`{"filename":"`)
	sb.WriteString(filename)
	sb.WriteString(`","status":"`)
	sb.WriteString(status)
	sb.WriteString(`","threat":"`)
	sb.WriteString(threat)
	sb.WriteString(`","scan_time":"`)
	sb.WriteString(time.Now().Format(time.RFC3339))
	sb.WriteString(`","engine":"ClamAV-Optimized"}`)
	return sb.String(), nil
}

func scanWithClamscanOptimized(ctx context.Context, r io.Reader) (status string, threat string, err error) {
	// Read the entire stream into memory first (we need this for clamscan stdin)
	bufPtr := bufferPool.Get().(*[]byte)
	defer bufferPool.Put(bufPtr)
	buf := *bufPtr

	var data bytes.Buffer
	_, err = io.CopyBuffer(&data, r, buf)
	if err != nil {
		return "", "", fmt.Errorf("failed to read input stream: %w", err)
	}

	// Create clamscan command
	cmd := exec.CommandContext(ctx, "clamscan",
		"--no-summary",        // No summary
		"--infected",          // Only show infected files
		"--stdout",            // Output to stdout
		"--max-filesize=500M", // Large file support
		"--max-scansize=500M", // Max scan size
		"-")                   // Read from stdin

	// Set the input data
	cmd.Stdin = bytes.NewReader(data.Bytes())

	// Run command and get output
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			switch exitErr.ExitCode() {
			case 1:
				// Virus found - optimized parsing
				if idx := strings.Index(outputStr, "FOUND"); idx != -1 {
					// Fast threat name extraction
					if colonIdx := strings.LastIndex(outputStr[:idx], ":"); colonIdx != -1 {
						threat = strings.TrimSpace(outputStr[colonIdx+1 : idx])
					}
				}
				return "infected", threat, nil
			case 2:
				return "", "", fmt.Errorf("clamscan configuration error: %s", outputStr)
			default:
				return "", "", fmt.Errorf("clamscan failed with exit code %d: %s", exitErr.ExitCode(), outputStr)
			}
		}
		return "", "", fmt.Errorf("clamscan execution error: %v - output: %s", err, outputStr)
	}
	return "clean", "", nil
}
func requestHandler(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())

	// Handle preflight OPTIONS requests
	if ctx.IsOptions() {
		handleOptions(ctx)
		return
	}

	switch path {
	case "/":
		uiHandler(ctx)
	case "/scan":
		scanHandler(ctx)
	default:
		setCORSHeaders(ctx)
		ctx.Error("Not found", fasthttp.StatusNotFound)
	}
}

func main() {
	var err error
	htmlContent, err = os.ReadFile("index.html")
	if err != nil {
		panic("Unable to load index.html: " + err.Error())
	}

	fmt.Println("Starting ULTRA-OPTIMIZED virus scanner server on :8081")
	fmt.Println("Web UI: http://localhost:8081")
	fmt.Println("API: POST http://localhost:8081/scan")
	fmt.Println("⚡ High-performance streaming scanner with CORS support!")
	fmt.Printf("🚀 Max concurrent scans: %d (2x CPU cores)\n", maxConcurrentScans)
	fmt.Printf("💾 Buffer pool size: 64KB per connection\n")
	fmt.Printf("🌐 CORS enabled for %d domains\n", len(allowedOrigins))

	// Ultra-high performance server settings
	server := &fasthttp.Server{
		Handler:                       requestHandler,
		MaxRequestBodySize:            500 * 1024 * 1024,      // 500MB
		ReadBufferSize:                256 * 1024,             // 256KB for large files
		WriteBufferSize:               256 * 1024,             // 256KB write buffer
		MaxConnsPerIP:                 50,                     // Higher connection limit
		MaxRequestsPerConn:            10000,                  // More connection reuse
		ReadTimeout:                   15 * time.Minute,       // Extended for large files
		WriteTimeout:                  15 * time.Minute,       // Extended write timeout
		IdleTimeout:                   10 * time.Minute,       // Longer idle timeout
		ReduceMemoryUsage:             false,                  // Prioritize speed
		StreamRequestBody:             true,                   // Essential for streaming
		DisableKeepalive:              false,                  // Keep connections alive
		TCPKeepalive:                  true,                   // TCP keepalive
		Concurrency:                   runtime.NumCPU() * 512, // Massive concurrency
		DisableHeaderNamesNormalizing: true,                   // Skip header normalization
		NoDefaultServerHeader:         true,                   // Skip default headers
		NoDefaultDate:                 true,                   // Skip date header
	}

	if err := server.ListenAndServe(":8081"); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
	}
}
