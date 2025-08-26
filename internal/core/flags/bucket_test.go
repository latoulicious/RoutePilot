package flags

import (
	"fmt"
	"testing"
)

func TestCalculateBucket(t *testing.T) {
	tests := []struct {
		name      string
		salt      string
		subjectID string
		expected  int
	}{
		{
			name:      "known input 1",
			salt:      "test-salt",
			subjectID: "user-123",
			expected:  6268, // Calculated using FNV-1a hash
		},
		{
			name:      "known input 2",
			salt:      "flag-abc",
			subjectID: "user-456",
			expected:  6785, // Calculated using FNV-1a hash
		},
		{
			name:      "empty subject",
			salt:      "test-salt",
			subjectID: "",
			expected:  8110, // Calculated using FNV-1a hash
		},
		{
			name:      "empty salt",
			salt:      "",
			subjectID: "user-123",
			expected:  1035, // Calculated using FNV-1a hash
		},
		{
			name:      "special characters",
			salt:      "salt-with-special!@#",
			subjectID: "user:with:colons",
			expected:  2485, // Calculated using FNV-1a hash
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateBucket(tt.salt, tt.subjectID)
			if result != tt.expected {
				t.Errorf("CalculateBucket(%q, %q) = %d, expected %d", tt.salt, tt.subjectID, result, tt.expected)
			}
		})
	}
}

func TestCalculateBucket_Deterministic(t *testing.T) {
	salt := "test-salt"
	subjectID := "user-123"
	
	// Call multiple times and ensure we get the same result
	first := CalculateBucket(salt, subjectID)
	for i := 0; i < 100; i++ {
		result := CalculateBucket(salt, subjectID)
		if result != first {
			t.Errorf("CalculateBucket is not deterministic: first=%d, iteration %d=%d", first, i, result)
		}
	}
}

func TestCalculateBucket_Range(t *testing.T) {
	salt := "test-salt"
	
	// Test with various subject IDs to ensure results are in valid range
	for i := 0; i < 1000; i++ {
		subjectID := fmt.Sprintf("user-%d", i)
		bucket := CalculateBucket(salt, subjectID)
		
		if bucket < 0 || bucket >= 10000 {
			t.Errorf("CalculateBucket(%q, %q) = %d, expected range [0, 9999]", salt, subjectID, bucket)
		}
	}
}

func TestCalculateBucket_Distribution(t *testing.T) {
	salt := "test-salt"
	buckets := make([]int, 10) // Track distribution across 10 segments
	
	// Generate 10000 buckets and check distribution
	for i := 0; i < 10000; i++ {
		subjectID := fmt.Sprintf("user-%d", i)
		bucket := CalculateBucket(salt, subjectID)
		segment := bucket / 1000 // 0-999 -> 0, 1000-1999 -> 1, etc.
		if segment >= 10 {
			segment = 9 // Handle edge case for bucket 9999
		}
		buckets[segment]++
	}
	
	// Each segment should have roughly 1000 entries (±20% tolerance)
	for i, count := range buckets {
		if count < 800 || count > 1200 {
			t.Errorf("Bucket segment %d has %d entries, expected ~1000 (±200)", i, count)
		}
	}
}

func TestIsInRollout(t *testing.T) {
	tests := []struct {
		name             string
		bucket           int
		rolloutPercentage int
		expected         bool
	}{
		{
			name:             "0% rollout",
			bucket:           0,
			rolloutPercentage: 0,
			expected:         false,
		},
		{
			name:             "100% rollout",
			bucket:           9999,
			rolloutPercentage: 100,
			expected:         true,
		},
		{
			name:             "50% rollout - included",
			bucket:           4999,
			rolloutPercentage: 50,
			expected:         true,
		},
		{
			name:             "50% rollout - excluded",
			bucket:           5000,
			rolloutPercentage: 50,
			expected:         false,
		},
		{
			name:             "10% rollout - included",
			bucket:           999,
			rolloutPercentage: 10,
			expected:         true,
		},
		{
			name:             "10% rollout - excluded",
			bucket:           1000,
			rolloutPercentage: 10,
			expected:         false,
		},
		{
			name:             "1% rollout - included",
			bucket:           99,
			rolloutPercentage: 1,
			expected:         true,
		},
		{
			name:             "1% rollout - excluded",
			bucket:           100,
			rolloutPercentage: 1,
			expected:         false,
		},
		{
			name:             "negative rollout",
			bucket:           0,
			rolloutPercentage: -1,
			expected:         false,
		},
		{
			name:             "over 100% rollout",
			bucket:           9999,
			rolloutPercentage: 150,
			expected:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsInRollout(tt.bucket, tt.rolloutPercentage)
			if result != tt.expected {
				t.Errorf("IsInRollout(%d, %d) = %v, expected %v", tt.bucket, tt.rolloutPercentage, result, tt.expected)
			}
		})
	}
}

func TestIsInRollout_EdgeCases(t *testing.T) {
	// Test boundary conditions for different rollout percentages
	testCases := []struct {
		rollout   int
		threshold int
	}{
		{1, 100},   // 1% -> threshold 100
		{5, 500},   // 5% -> threshold 500
		{10, 1000}, // 10% -> threshold 1000
		{25, 2500}, // 25% -> threshold 2500
		{50, 5000}, // 50% -> threshold 5000
		{75, 7500}, // 75% -> threshold 7500
		{99, 9900}, // 99% -> threshold 9900
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d%% rollout", tc.rollout), func(t *testing.T) {
			// Bucket just below threshold should be included
			if !IsInRollout(tc.threshold-1, tc.rollout) {
				t.Errorf("Bucket %d should be included in %d%% rollout", tc.threshold-1, tc.rollout)
			}
			
			// Bucket at threshold should be excluded
			if IsInRollout(tc.threshold, tc.rollout) {
				t.Errorf("Bucket %d should be excluded from %d%% rollout", tc.threshold, tc.rollout)
			}
		})
	}
}