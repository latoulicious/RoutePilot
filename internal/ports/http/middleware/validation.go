package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode"
)

const (
	// MaxRequestSize limits request body size to 1MB
	MaxRequestSize = 1024 * 1024
	// MaxHeaderSize limits individual header size
	MaxHeaderSize = 8192
)

// ValidationMiddleware provides request validation and sanitization
type ValidationMiddleware struct {
	// Add any configuration if needed
}

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware() *ValidationMiddleware {
	return &ValidationMiddleware{}
}

// Middleware returns the HTTP middleware function
func (m *ValidationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate request size
		if r.ContentLength > MaxRequestSize {
			writeErrorResponse(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE",
				fmt.Sprintf("Request body too large, maximum %d bytes allowed", MaxRequestSize))
			return
		}

		// Validate headers
		if err := m.validateHeaders(r); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "INVALID_HEADERS", err.Error())
			return
		}

		// Sanitize and validate request path
		if err := m.validatePath(r); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "INVALID_PATH", err.Error())
			return
		}

		// For requests with body, validate and add body hash
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
			if err := m.processRequestBody(w, r); err != nil {
				writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", err.Error())
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// validateHeaders checks header values for security issues
func (m *ValidationMiddleware) validateHeaders(r *http.Request) error {
	for name, values := range r.Header {
		// Check header name
		if !isValidHeaderName(name) {
			return fmt.Errorf("invalid header name: %s", name)
		}

		// Check header values
		for _, value := range values {
			if len(value) > MaxHeaderSize {
				return fmt.Errorf("header %s too large", name)
			}

			if !isValidHeaderValue(value) {
				return fmt.Errorf("invalid header value for %s", name)
			}
		}
	}

	return nil
}

// validatePath validates and sanitizes the request path
func (m *ValidationMiddleware) validatePath(r *http.Request) error {
	path := r.URL.Path

	// Check for path traversal attempts
	if strings.Contains(path, "..") || strings.Contains(path, "//") {
		return fmt.Errorf("invalid path: contains path traversal")
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("invalid path: contains null bytes")
	}

	// Validate path characters (allow alphanumeric, hyphens, underscores, slashes)
	validPath := regexp.MustCompile(`^[a-zA-Z0-9/_-]+$`)
	if !validPath.MatchString(path) {
		return fmt.Errorf("invalid path: contains invalid characters")
	}

	return nil
}

// processRequestBody validates and processes the request body
func (m *ValidationMiddleware) processRequestBody(_ http.ResponseWriter, r *http.Request) error {
	if r.Body == nil {
		return nil
	}

	// Read the body
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxRequestSize))
	if err != nil {
		return fmt.Errorf("failed to read request body: %v", err)
	}
	r.Body.Close()

	// Validate JSON if Content-Type is application/json
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") {
		if err := m.validateJSON(body); err != nil {
			return fmt.Errorf("invalid JSON: %v", err)
		}
	}

	// Create body hash for signature verification
	hash := sha256.Sum256(body)
	r.Header.Set("X-Body-Hash", hex.EncodeToString(hash[:]))

	// Restore the body for downstream handlers
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))

	return nil
}

// validateJSON validates that the body contains valid JSON
func (m *ValidationMiddleware) validateJSON(body []byte) error {
	if len(body) == 0 {
		return nil
	}

	var js json.RawMessage
	if err := json.Unmarshal(body, &js); err != nil {
		return err
	}

	// Additional JSON security checks
	bodyStr := string(body)

	// Check for potential JSON injection patterns
	if strings.Contains(bodyStr, "__proto__") || strings.Contains(bodyStr, "constructor") {
		return fmt.Errorf("potentially malicious JSON content")
	}

	return nil
}

// isValidHeaderName checks if a header name is valid
func isValidHeaderName(name string) bool {
	if name == "" {
		return false
	}

	for _, r := range name {
		if !isTokenChar(r) {
			return false
		}
	}

	return true
}

// isValidHeaderValue checks if a header value is valid
func isValidHeaderValue(value string) bool {
	for _, r := range value {
		// Allow printable ASCII and tabs, but not control characters
		if r < 32 && r != 9 || r == 127 {
			return false
		}
	}

	return true
}

// isTokenChar checks if a character is valid in an HTTP token
func isTokenChar(r rune) bool {
	return r > 32 && r < 127 && !strings.ContainsRune("()<>@,;:\\\"/[]?={} \t", r)
}

// SanitizeString removes potentially dangerous characters from strings
func SanitizeString(s string) string {
	// Remove control characters except tab, newline, and carriage return
	var result strings.Builder
	for _, r := range s {
		if unicode.IsPrint(r) || r == '\t' || r == '\n' || r == '\r' {
			result.WriteRune(r)
		}
	}

	return strings.TrimSpace(result.String())
}

// ValidateUUID checks if a string is a valid UUID format
func ValidateUUID(s string) bool {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	return uuidRegex.MatchString(strings.ToLower(s))
}

// ValidateFlagKey checks if a flag key is valid (alphanumeric with underscores)
func ValidateFlagKey(key string) bool {
	if key == "" || len(key) > 100 {
		return false
	}

	validKey := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	return validKey.MatchString(key)
}
