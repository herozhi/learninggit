package gorm

import (
	"context"
	"database/sql"
	"strings"
)

const (
	StressTestTablePostfix   = "_stress"
	ContextStressKey         = "K_STRESS"
	ContextSkipStressForRead = "K_SKIP_STRESS"
	ContextStressSwitch      = "K_STRESS_SWITCH"

	SwitchOn = "on"
	SwitchOff = "off"
)

const (
	//scopeKindCreate = "creates"
	//scopeKindUpdate = "updates"
	//scopeKindDelete = "deletes"
	//scopeKindQueries = "queries"
	//scopeKindRowQueries = "rowQueries"

	ScopeWrite  = "W"
	ScopeRead   = "R"
	ScopeUnkown = "Ukn"
)

func getPostfixedTableName(tableName string) string {
	if strings.HasSuffix(tableName, StressTestTablePostfix) {
		return tableName
	}
	return tableName + StressTestTablePostfix
}

func logRejectionExecutions(scope *Scope, execType string) {
	scope.db.log("[Reject] table:", scope.TableName(), " execType:", execType, " conditions:", scope.Search, " value:", scope.Value)
}

func isTestRequest(ctx context.Context) bool {
	if stressTag, ok := ctx.Value(ContextStressKey).(string); ok {
		return stressTag != ""
	}
	return false
}

func shouldSkipTest(ctx context.Context) bool {
	if skip, ok := ctx.Value(ContextSkipStressForRead).(bool); ok {
		return skip
	}
	return false
}

// Context set context
func (s *DB) Context(ctx context.Context) *DB {
	clone := s.clone()
	clone.Ctx = ctx
	clone.isTestRequest = isTestRequest(ctx)
	clone.shouldSkipTest = shouldSkipTest(ctx)
	return clone
}

type sqlCtxExecer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

type sqlCtxQuerier interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

type sqlCtxPreparer interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}
