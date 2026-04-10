package conf

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// File 与 configs/config.yaml 结构对应。
type File struct {
	Auth           FileAuth       `yaml:"auth"`
	Web            FileWeb        `yaml:"web"`
	ClickHouse     FileClickHouse `yaml:"clickhouse"`
	Log            FileLog        `yaml:"log"`
	AmapKey        string `yaml:"amap_key"`
	AmapSecurityJs string `yaml:"amap_security_js"`
	// AmapWebKey 高德 Web 服务 Key（逆地理入库 district_adcode）；与 amap_key（JS）分离，不可混用。
	AmapWebKey string `yaml:"amap_web_key"`
}

// FileLog 日志输出（Kratos log）：默认文件路径见 DefaultLogFile；设为 "-" 或 "none" 则仅控制台。
type FileLog struct {
	File string `yaml:"file"`
}

// FileAuth HTTP /pub 上报的 Basic Auth（可选）。
type FileAuth struct {
	HTTPUser string `yaml:"http_user"`
	HTTPPass string `yaml:"http_pass"`
}

// LoadFile 读取 YAML；path 为空则尝试 defaultConfigPath。
func LoadFile(path string) (*File, error) {
	if path == "" {
		path = DefaultConfigPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	return &f, nil
}

// FileToWeb 将 YAML 转为 WebConfig；members 为空表示展示库内全部轨迹（个人自用）。
func FileToWeb(f *File) *WebConfig {
	w := &WebConfig{
		Listen:     ":8080",
		Title:      "",
		CHDatabase: defaultClickHouseDB,
	}
	if f == nil {
		return w
	}
	if f.Web.Listen != "" {
		w.Listen = f.Web.Listen
	} else if f.Web.Port > 0 {
		w.Listen = fmt.Sprintf(":%d", f.Web.Port)
	}
	if f.Web.Title != "" {
		w.Title = f.Web.Title
	} else if f.Web.TeamName != "" {
		w.Title = f.Web.TeamName
	}
	w.StaticDir = f.Web.StaticDir
	w.CHDSN = f.ClickHouse.ResolvedDSN()
	if f.ClickHouse.Database != "" {
		w.CHDatabase = f.ClickHouse.Database
	}
	for _, m := range f.Web.Members {
		disp := m.Display
		if disp == "" {
			disp = m.User
		}
		if m.User == "" {
			continue
		}
		w.Members = append(w.Members, TeamMember{User: m.User, Display: disp})
	}
	w.HTTPUser = f.Auth.HTTPUser
	w.HTTPPass = f.Auth.HTTPPass
	w.Pin = f.Web.Pin
	w.AmapKey = f.AmapKey
	w.AmapSecurityJs = f.AmapSecurityJs
	w.AmapWebKey = f.AmapWebKey
	return w
}

// DefaultConfigPath 默认配置文件路径（相对工作目录）。
const DefaultConfigPath = "configs/config.yaml"

// DefaultLogFile 默认日志文件路径（与控制台同时输出）；ResolveLogFile 在配置与环境均未指定时使用。
const DefaultLogFile = "logs/owntracks.log"

// ResolveLogFile 合并 LOG_FILE 与 YAML 的 log.file；未配置时使用 DefaultLogFile；"-" / "none" 表示仅控制台。
func ResolveLogFile(env, yamlPath string) string {
	v := FirstNonEmpty(env, yamlPath)
	if v == "" {
		return DefaultLogFile
	}
	if v == "-" || v == "none" {
		return ""
	}
	return v
}

// EnvOr 返回环境变量 key 的非空值，否则返回 fallback。
func EnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// FirstNonEmpty 返回第一个非空字符串。
func FirstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
