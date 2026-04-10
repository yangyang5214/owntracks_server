package main

import (
	"os"

	"github.com/spf13/cobra"

	"owntracks_server/internal/conf"
	"owntracks_server/internal/di"
)

var (
	webConfigPath string
	webListen     string
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "启动 HTTP 服务（OwnTracks 上报 + 轨迹控制台）",
	RunE:  runWeb,
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "同 web：HTTP 接收 OwnTracks 位置并服务 Web 控制台",
	RunE:  runWeb,
}

func init() {
	for _, c := range []*cobra.Command{webCmd, serverCmd} {
		rootCmd.AddCommand(c)
		c.Flags().StringVar(&webConfigPath, "config", conf.DefaultConfigPath, "YAML 配置文件路径")
		c.Flags().StringVar(&webListen, "listen", "", "HTTP 监听（默认来自配置 :8080）")
	}
}

func runWeb(_ *cobra.Command, _ []string) error {
	f, err := conf.LoadFile(webConfigPath)
	if err != nil {
		return err
	}
	w := conf.FileToWeb(f)
	if v := os.Getenv("WEB_ADDR"); v != "" {
		w.Listen = v
	}
	w.HTTPUser = conf.FirstNonEmpty(os.Getenv("HTTP_USER"), w.HTTPUser)
	w.HTTPPass = conf.FirstNonEmpty(os.Getenv("HTTP_PASS"), w.HTTPPass)
	w.AmapWebKey = conf.FirstNonEmpty(os.Getenv("AMAP_WEB_KEY"), w.AmapWebKey)
	if webListen != "" {
		w.Listen = webListen
	}
	logPath := conf.ResolveLogFile(os.Getenv("LOG_FILE"), f.Log.File)
	_, err = di.InitializeWeb(w, logPath)
	return err
}
