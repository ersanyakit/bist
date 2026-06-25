package confidence

const (
	BaseScore                     = 0.20
	StructuredSourceWeight        = 0.35
	PDFTextWeight                 = 0.18
	DocumentIDWeight              = 0.10
	PageWeight                    = 0.08
	TableWeight                   = 0.10
	RowColumnWeight               = 0.08
	ValidationPassedWeight        = 0.18
	ValidationWarningPenalty      = 0.08
	InferredPenalty               = 0.18
	OCRPenalty                    = 0.25
	ContradictionPenalty          = 0.35
	ReviewRequiredThreshold       = 0.75
)

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
	score := BaseScore
	if signals.StructuredSource {
		score += StructuredSourceWeight
	}
	if signals.PDFText {
		score += PDFTextWeight
	}
	if signals.HasDocumentID {
		score += DocumentIDWeight
	}
	if signals.HasPage {
		score += PageWeight
	}
	if signals.HasTable {
		score += TableWeight
	}
	if signals.HasRowColumn {
		score += RowColumnWeight
	}
	if signals.ValidationPassed {
		score += ValidationPassedWeight
	}
	if signals.ValidationWarning {
		score -= ValidationWarningPenalty
	}
	if signals.Inferred {
		score -= InferredPenalty
	}
	if signals.OCR {
		score -= OCRPenalty
	}
	if signals.Contradiction {
		score -= ContradictionPenalty
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
	return score < ReviewRequiredThreshold
}
