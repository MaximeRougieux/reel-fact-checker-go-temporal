package config

import "os"

// Get returns the environment variable key, or def when it is unset or empty.
func Get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Address() string   { return Get("TEMPORAL_ADDRESS", "localhost:7233") }
func Namespace() string { return Get("TEMPORAL_NAMESPACE", "default") }
func TaskQueue() string { return Get("TEMPORAL_TASK_QUEUE", "reel-fact-checker") }
