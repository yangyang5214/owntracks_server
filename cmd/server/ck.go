package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/spf13/cobra"

	"owntracks_server/internal/ckexec"
	"owntracks_server/internal/conf"
)

var ckDatabase string

var ckCmd = &cobra.Command{
	Use:          "ck [flags] <sql-file>...",
	Short:        "在 ClickHouse 上执行 SQL 文件（按分号分隔多条语句）",
	Args:         cobra.MinimumNArgs(1),
	SilenceUsage: true,
	RunE:         runCK,
}

func init() {
	ckCmd.Flags().StringVar(&webConfigPath, "config", conf.DefaultConfigPath, "YAML 配置文件路径")
	ckCmd.Flags().StringVar(&ckDatabase, "database", "", "覆盖连接中的库名（执行建库脚本时可设为 default）")
	rootCmd.AddCommand(ckCmd)
}

func runCK(_ *cobra.Command, args []string) error {
	f, err := conf.LoadFile(webConfigPath)
	if err != nil {
		return err
	}
	dsn := f.ClickHouse.ResolvedDSN()
	if v := os.Getenv("CLICKHOUSE_DSN"); v != "" {
		dsn = v
	}
	if dsn == "" {
		return fmt.Errorf("clickhouse: 请在配置中填写 clickhouse.dsn 或 host，或设置环境变量 CLICKHOUSE_DSN")
	}
	if ckDatabase != "" {
		dsn, err = conf.ClickHouseDSNWithDatabase(dsn, ckDatabase)
		if err != nil {
			return err
		}
	}

	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("clickhouse ping: %w", err)
	}

	for _, path := range args {
		if err := ckexec.ExecSQLFile(ctx, db, path); err != nil {
			return err
		}
	}
	return nil
}
