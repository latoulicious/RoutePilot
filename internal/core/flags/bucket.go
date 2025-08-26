package flags

import (
	"hash/fnv"
)

// CalculateBucket calculates a deterministic bucket (0-9999) for a subject
// using FNV-1a hash algorithm. This ensures consistent assignment across
// multiple evaluations for the same subject and flag combination.
func CalculateBucket(salt, subjectID string) int {
	h := fnv.New32a()
	h.Write([]byte(salt + ":" + subjectID))
	return int(h.Sum32() % 10000)
}

// IsInRollout determines if a bucket falls within the rollout percentage
func IsInRollout(bucket, rolloutPercentage int) bool {
	if rolloutPercentage <= 0 {
		return false
	}
	if rolloutPercentage >= 100 {
		return true
	}
	
	// Convert percentage to bucket threshold (0-100% maps to 0-9999 buckets)
	threshold := (rolloutPercentage * 10000) / 100
	return bucket < threshold
}