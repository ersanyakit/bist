package confidence

type EvidenceSignals struct {
	StructuredSource  bool
	PDFText           bool
	OCR               bool
	HasDocumentID     bool
	HasPage           bool
	HasTable          bool
	HasRowColumn      bool
	ValidationPassed  bool
	ValidationWarning bool
	Inferred          bool
	Contradiction     bool
}

func Score(signals EvidenceSignals) float64 {
	score := 0.20
	if signals.StructuredSource {
		score += 0.35
	}
	if signals.PDFText {
		score += 0.18
	}
	if signals.HasDocumentID {
		score += 0.10
	}
	if signals.HasPage {
		score += 0.08
	}
	if signals.HasTable {
		score += 0.10
	}
	if signals.HasRowColumn {
		score += 0.08
	}
	if signals.ValidationPassed {
		score += 0.18
	}
	if signals.ValidationWarning {
		score -= 0.08
	}
	if signals.Inferred {
		score -= 0.18
	}
	if signals.OCR {
		score -= 0.25
	}
	if signals.Contradiction {
		score -= 0.35
	}
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func ReviewRequired(score float64) bool {
	return score < 0.75
}
