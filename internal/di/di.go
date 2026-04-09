package di

import (
	"github.com/go-kratos/kratos/v2/log"

	"owntracks_server/internal/conf"
	"owntracks_server/internal/logx"
	"owntracks_server/internal/webapp"
)

// LoggerBundle 供 Wire 注入：Kratos Helper + 日志文件关闭回调。
type LoggerBundle struct {
	Helper *log.Helper
	Close  func() error
}

// ProvideLogger 由 logPath 构造日志（控制台 ± 文件）。
func ProvideLogger(logPath string) (LoggerBundle, error) {
	h, c, err := logx.New(logPath)
	if err != nil {
		return LoggerBundle{}, err
	}
	return LoggerBundle{Helper: h, Close: c}, nil
}

// RunWeb 启动 Web 控制台；返回后关闭日志资源。
func RunWeb(cfg *conf.WebConfig, b LoggerBundle) (struct{}, error) {
	defer func() { _ = b.Close() }()
	if err := webapp.Run(cfg, b.Helper); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
}
