package domain

import "context"

type FinancialFactRepository interface {
	UpsertStatementVersions(ctx context.Context, ticker string, versions []FinancialStatementVersion) (FinancialStatementVersionStore, error)
	LoadStatementVersions(ctx context.Context, ticker string) (FinancialStatementVersionStore, error)
	AppendAuditEvent(ctx context.Context, event DataLineageEvent) error
}
