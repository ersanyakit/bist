package technical

import (
	"context"
	"fmt"

	"hissebot/internal/domain/marketdata"
)

type Service struct{}

func (Service) ScoreTechnical(_ context.Context, prices []marketdata.OHLCV) (float64, []string, error) {
	if len(prices) < 2 {
		return 0, []string{"teknik skor için en az iki fiyat barı gerekir"}, nil
	}
	first := prices[0].Close
	last := prices[len(prices)-1].Close
	if first <= 0 || last <= 0 {
		return 0, []string{"geçerli kapanış fiyatı yok"}, nil
	}
	returnPct := (last - first) / first * 100
	score := 50 + returnPct
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score, []string{fmt.Sprintf("seçili pencerede fiyat getirisi %.2f%%", returnPct)}, nil
}
