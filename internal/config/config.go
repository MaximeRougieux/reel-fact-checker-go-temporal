package config

import "os"

func get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Address() string   { return get("TEMPORAL_ADDRESS", "localhost:7233") }
func Namespace() string { return get("TEMPORAL_NAMESPACE", "default") }
func TaskQueue() string { return get("TEMPORAL_TASK_QUEUE", "reel-fact-checker") }
