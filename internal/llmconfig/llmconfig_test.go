package llmconfig

import "testing"

func clearEnv(t *testing.T) {
	for _, k := range []string{"LLM_PROVIDER", "ANTHROPIC_API_KEY", "MISTRAL_API_KEY", "ANTHROPIC_MODEL",
		"ANTHROPIC_JUDGE_MODEL", "MISTRAL_MODEL", "MISTRAL_JUDGE_MODEL"} {
		t.Setenv(k, "")
	}
}

func TestProvider(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"nothing set", nil, ""},
		{"anthropic key", map[string]string{"ANTHROPIC_API_KEY": "k"}, "claude"},
		{"mistral key", map[string]string{"MISTRAL_API_KEY": "k"}, "mistral"},
		{"both keys prefer claude", map[string]string{"ANTHROPIC_API_KEY": "k", "MISTRAL_API_KEY": "k"}, "claude"},
		{"explicit wins over keys", map[string]string{"LLM_PROVIDER": "Mistral", "ANTHROPIC_API_KEY": "k"}, "mistral"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clearEnv(t)
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			if got := Provider(); got != c.want {
				t.Errorf("Provider() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestModels(t *testing.T) {
	clearEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "k")
	if ExtractorModel() != "claude-haiku-5-5" || JudgeModel() != "claude-haiku-5-5" {
		t.Errorf("claude defaults: %q %q", ExtractorModel(), JudgeModel())
	}
	t.Setenv("ANTHROPIC_JUDGE_MODEL", "claude-judge")
	if JudgeModel() != "claude-judge" {
		t.Errorf("claude judge override: %q", JudgeModel())
	}

	clearEnv(t)
	t.Setenv("MISTRAL_API_KEY", "k")
	if ExtractorModel() != "mistral-small-latest" || JudgeModel() != "mistral-small-latest" {
		t.Errorf("mistral defaults: %q %q", ExtractorModel(), JudgeModel())
	}
	t.Setenv("MISTRAL_MODEL", "mistral-x")
	t.Setenv("ANTHROPIC_JUDGE_MODEL", "must-be-ignored")
	if ExtractorModel() != "mistral-x" || JudgeModel() != "mistral-x" {
		t.Errorf("mistral model: %q %q", ExtractorModel(), JudgeModel())
	}
	t.Setenv("MISTRAL_JUDGE_MODEL", "mistral-judge")
	if JudgeModel() != "mistral-judge" {
		t.Errorf("mistral judge override: %q", JudgeModel())
	}
}

func TestNewLLM(t *testing.T) {
	for _, p := range []string{"claude", "mistral"} {
		if l, err := NewLLM(p, "m"); err != nil || l == nil {
			t.Errorf("NewLLM(%q) = %v, %v", p, l, err)
		}
	}
	if _, err := NewLLM("nope", "m"); err == nil {
		t.Fatal("want error for unknown provider")
	}
}
