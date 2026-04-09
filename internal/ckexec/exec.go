package ckexec

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// SplitStatements 将 SQL 文本拆成多条语句（按 `;` 分割，并去掉行注释与块注释）。
// 适用于 DDL/迁移脚本；若语句中含未转义分号可能误拆分。
func SplitStatements(sql string) []string {
	sql = stripBlockComments(sql)
	lineComment := regexp.MustCompile(`(?m)^\s*--.*$`)
	sql = lineComment.ReplaceAllString(sql, "")
	var out []string
	for _, part := range strings.Split(sql, ";") {
		s := strings.TrimSpace(part)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func stripBlockComments(s string) string {
	for {
		i := strings.Index(s, "/*")
		if i < 0 {
			return s
		}
		j := strings.Index(s[i+2:], "*/")
		if j < 0 {
			return s[:i]
		}
		j = i + 2 + j
		s = s[:i] + s[j+2:]
	}
}

// ExecSQLFile 读取并执行 SQL 文件中的全部语句。
func ExecSQLFile(ctx context.Context, db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %q: %w", path, err)
	}
	stmts := SplitStatements(string(data))
	for i, q := range stmts {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("%s: statement %d/%d: %w", path, i+1, len(stmts), err)
		}
	}
	return nil
}
