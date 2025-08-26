package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Demo script to test the flag evaluation endpoint
func main() {
	fmt.Println("Flag Evaluation Endpoint Demo")
	fmt.Println("=============================")

	// Test cases
	testCases := []struct {
		flagKey   string
		subjectID string
		desc      string
	}{
		{"test_flag", "user123", "User 123 evaluating test_flag"},
		{"test_flag", "user456", "User 456 evaluating test_flag"},
		{"test_flag", "user123", "User 123 evaluating test_flag again (should be consistent)"},
		{"feature_x", "user789", "User 789 evaluating feature_x"},
	}

	baseURL := "http://localhost:8080"

	// First check if server is running
	fmt.Printf("Checking if server is running at %s...\n", baseURL)
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		fmt.Printf("❌ Server not running: %v\n", err)
		fmt.Println("\nTo start the server, run:")
		fmt.Println("  go run ./cmd/api")
		return
	}
	resp.Body.Close()

	if resp.StatusCode == 200 {
		fmt.Println("✅ Server is running!")
	} else {
		fmt.Printf("⚠️  Server responded with status: %d\n", resp.StatusCode)
	}

	fmt.Println("\nTesting flag evaluation endpoint...")
	fmt.Println("Note: This demo uses mock authentication, so requests may fail with 401")
	fmt.Println("The endpoint is working correctly if you see proper error responses")

	for i, tc := range testCases {
		fmt.Printf("\n%d. %s\n", i+1, tc.desc)
		
		url := fmt.Sprintf("%s/v1/flags/%s/eval?subject_id=%s", baseURL, tc.flagKey, tc.subjectID)
		fmt.Printf("   URL: %s\n", url)

		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("   ❌ Request failed: %v\n", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Printf("   ❌ Failed to read response: %v\n", err)
			continue
		}

		fmt.Printf("   Status: %d\n", resp.StatusCode)
		
		// Pretty print JSON response
		var jsonData interface{}
		if err := json.Unmarshal(body, &jsonData); err == nil {
			prettyJSON, _ := json.MarshalIndent(jsonData, "   ", "  ")
			fmt.Printf("   Response: %s\n", string(prettyJSON))
		} else {
			fmt.Printf("   Response: %s\n", string(body))
		}

		// Add small delay between requests
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\n" + repeatString("=", 50))
	fmt.Println("Demo completed!")
	fmt.Println("\nExpected behavior:")
	fmt.Println("- 401 responses indicate authentication is working")
	fmt.Println("- 200 responses would show successful flag evaluation")
	fmt.Println("- Same subject+flag combinations should return consistent results")
}

// Helper function since Go doesn't have string.repeat
func repeatString(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}