package factcheck

// WorkflowID is the cache key for a reel: one Workflow ID per reel shortcode,
// so a completed run is found again instead of recomputed.
func WorkflowID(canonicalReelURL string) string {
	return "factcheck-" + shortcode(canonicalReelURL)
}
