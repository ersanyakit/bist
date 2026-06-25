package repositories_test

import (
	"testing"

	"hissebot/internal/repositories"
	"hissebot/internal/repositories/filedocuments"
	"hissebot/internal/repositories/memory"
	"hissebot/internal/storage"
)

var (
	_ repositories.EquityRepository             = (*storage.EquityStore)(nil)
	_ repositories.PriceRepository              = (*memory.Store)(nil)
	_ repositories.FinancialStatementRepository = (*memory.Store)(nil)
	_ repositories.DisclosureRepository         = (*memory.Store)(nil)
	_ repositories.MacroRepository              = (*memory.Store)(nil)
	_ repositories.StockRepository              = (*memory.Store)(nil)
	_ repositories.AnalysisRepository           = (*memory.Store)(nil)
	_ repositories.DocumentRepository           = (*memory.Store)(nil)
	_ repositories.DocumentRepository           = (*filedocuments.Store)(nil)
)

func TestRepositoryAdaptersSatisfyPorts(t *testing.T) {
}
