package factcheck

import "testing"

func TestParseReelURL(t *testing.T) {
	ok := map[string]string{
		"https://www.instagram.com/reel/Cabc123xyz/":          "https://www.instagram.com/reel/Cabc123xyz/",
		"https://instagram.com/reels/Cabc123xyz":              "https://www.instagram.com/reel/Cabc123xyz/",
		"https://www.instagram.com/someuser/reel/Cabc123xyz/": "https://www.instagram.com/reel/Cabc123xyz/",
	}
	for in, want := range ok {
		if got, err := ParseReelURL(in); err != nil || got != want {
			t.Errorf("%s: got %q, %v", in, got, err)
		}
	}
	bad := []string{
		"http://www.instagram.com/reel/Cabc123xyz/",
		"https://evil.com/reel/Cabc123xyz/",
		"https://www.instagram.com.evil.com/reel/Cabc123xyz/",
		"https://instagram.com@evil.com/reel/Cabc123xyz/",
		"https://www.instagram.com/p/Cabc123xyz/",
		"https://www.instagram.com:8443/reel/Cabc123xyz/",
		"not a url",
	}
	for _, in := range bad {
		if _, err := ParseReelURL(in); err == nil {
			t.Errorf("%s: expected error", in)
		}
	}
}
