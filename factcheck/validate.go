package factcheck

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var reelPath = regexp.MustCompile(`^/(?:[A-Za-z0-9._]+/)?reels?/([A-Za-z0-9_-]{5,20})/?$`)

var allowedHosts = map[string]bool{
	"instagram.com":     true,
	"www.instagram.com": true,
	"instagr.am":        true,
}

// ParseReelURL accepts only https Instagram reel URLs. Nothing is fetched here.
func ParseReelURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("malformed URL: %w", err)
	}
	if u.Scheme != "https" || !allowedHosts[strings.ToLower(u.Hostname())] || u.Port() != "" || u.User != nil {
		return "", fmt.Errorf("not an Instagram URL")
	}
	m := reelPath.FindStringSubmatch(u.Path)
	if m == nil {
		return "", fmt.Errorf("not an Instagram reel path")
	}
	return "https://www.instagram.com/reel/" + m[1] + "/", nil
}

func shortcode(canonical string) string {
	u, _ := url.Parse(canonical)
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	return parts[len(parts)-1]
}
