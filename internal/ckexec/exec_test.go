package ckexec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitStatements_initSQL(t *testing.T) {
	root := filepath.Join("..", "..", "resource", "init.sql")
	b, err := os.ReadFile(root)
	if err != nil {
		t.Skipf("skip: %v", err)
	}
	stmts := SplitStatements(string(b))
	if len(stmts) != 5 {
		t.Fatalf("expected 5 statements, got %d", len(stmts))
	}
}
