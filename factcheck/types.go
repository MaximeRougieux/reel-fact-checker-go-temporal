package factcheck

type Claim struct {
	Text         string   `json:"text"`
	Kind         string   `json:"kind"` // "fact" or "implication"
	Premises     []string `json:"premises"`
	TimestampSec int      `json:"timestampSec"`
	// Verifiable is false for purely private matters (what someone privately
	// thought, felt or did) that no public source could confirm. Those are not searched.
	Verifiable bool `json:"verifiable"`
}

type Evidence struct {
	Source  string  `json:"source"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"` // relevance from the search provider, 0..1
}

const (
	Supported    = "supported"
	Refuted      = "refuted"
	Insufficient = "insufficient_evidence"
	NotCheckable = "not_checkable"
	Contested    = "contested" // supported or refuted, but below ContestedBelow
)

// ContestedBelow is the top-label probability under which a supported/refuted
// verdict is reported as contested.
const ContestedBelow = 0.6

type Verdict struct {
	Claim         Claim              `json:"claim"`
	Label         string             `json:"label"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
	Evidence      []Evidence         `json:"evidence"`
}

type Report struct {
	ReelURL  string    `json:"reelUrl"`
	Verdicts []Verdict `json:"verdicts"`
	Summary  string    `json:"summary"`
}
