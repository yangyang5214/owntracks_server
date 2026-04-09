//go:build wireinject
// +build wireinject

package di

import (
	"github.com/google/wire"

	"owntracks_server/internal/conf"
)

// InitializeWeb 组装 Web 控制台依赖并运行（阻塞至退出）。
func InitializeWeb(cfg *conf.WebConfig, logPath string) (struct{}, error) {
	panic(wire.Build(
		RunWeb,
		ProvideLogger,
	))
}
