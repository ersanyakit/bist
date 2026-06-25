package confidence

import "testing"

func TestScoreEvidenceSignals(t *testing.T) {
	tests := []struct {
		name string
		in   EvidenceSignals
		want float64
	}{
		{name: "base only", want: BaseScore},
		{
			name: "structured validated table evidence caps at one",
			in: EvidenceSignals{
				StructuredSource: true,
				HasDocumentID:    true,
				HasPage:          true,
				HasTable:         true,
				HasRowColumn:     true,
				ValidationPassed: true,
			},
			want: 1,
		},
		{
			name: "ocr inferred contradiction floors at zero",
			in: EvidenceSignals{
				OCR:           true,
				Inferred:      true,
				Contradiction: true,
			},
			want: 0,
		},
		{
			name: "pdf document page remains review grade",
			in: EvidenceSignals{
				PDFText:       true,
				HasDocumentID: true,
				HasPage:       true,
			},
			want: BaseScore + PDFTextWeight + DocumentIDWeight + PageWeight,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Score(tt.in); got != tt.want {
				t.Fatalf("Score() = %.4f, want %.4f", got, tt.want)
			}
		})
	}
}

func TestReviewRequiredThreshold(t *testing.T) {
	tests := []struct {
		score float64
		want  bool
	}{
		{score: ReviewRequiredThreshold - 0.01, want: true},
		{score: ReviewRequiredThreshold, want: false},
		{score: ReviewRequiredThreshold + 0.01, want: false},
	}
	for _, tt := range tests {
		if got := ReviewRequired(tt.score); got != tt.want {
			t.Fatalf("ReviewRequired(%.2f) = %t, want %t", tt.score, got, tt.want)
		}
	}
}
