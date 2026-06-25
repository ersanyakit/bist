package technical

import (
	"context"
	"strings"
	"testing"

	"hissebot/internal/domain/marketdata"
)

func TestScoreTechnical(t *testing.T) {
	tests := []struct {
		name       string
		prices     []marketdata.OHLCV
		want       float64
		wantNote   string
	}{
		{name: "requires at least two prices", prices: []marketdata.OHLCV{{Close: 10}}, want: 0, wantNote: "en az iki"},
		{name: "rejects non-positive close", prices: []marketdata.OHLCV{{Close: 0}, {Close: 10}}, want: 0, wantNote: "geçerli kapanış"},
		{name: "positive return increases neutral score", prices: []marketdata.OHLCV{{Close: 100}, {Close: 112}}, want: 62},
		{name: "large drawdown floors at zero", prices: []marketdata.OHLCV{{Close: 100}, {Close: 0.1}}, want: 0},
		{name: "large rally caps at one hundred", prices: []marketdata.OHLCV{{Close: 10}, {Close: 100}}, want: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, notes, err := Service{}.ScoreTechnical(context.Background(), tt.prices)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("score = %.2f, want %.2f", got, tt.want)
			}
			if tt.wantNote != "" && (len(notes) == 0 || !strings.Contains(notes[0], tt.wantNote)) {
				t.Fatalf("notes = %+v, want containing %q", notes, tt.wantNote)
			}
		})
	}
}
