package gormx

import "testing"

func TestExtractWhere_LimitAndNestedTokens(t *testing.T) {
	stmt := "SELECT * FROM invoices WHERE status='paid' AND amount>=1000 AND id=10 ORDER BY created_at DESC"
	cols, vals, conds := extractWhere(stmt, 2, false)
	if len(cols) != 2 || len(conds) != 2 {
		t.Fatalf("expected bounded extraction to 2 conditions, got cols=%d conds=%d", len(cols), len(conds))
	}
	if vals["status"] != "paid" {
		t.Fatalf("expected status=paid, got=%v", vals["status"])
	}
}
