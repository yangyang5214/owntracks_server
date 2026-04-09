package main

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:          "owntracks_server",
	Short:        "OwnTracks HTTP 上报 + 轨迹 Web 控制台（ClickHouse）",
	SilenceUsage: true,
}
