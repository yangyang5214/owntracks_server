package conf

// FileWeb Web 控制台（个人默认展示库内全部历史）。
type FileWeb struct {
	// Port HTTP 监听端口（默认 8080）；listen 非空时优先。
	Port      int            `yaml:"port"`
	Listen    string         `yaml:"listen"`
	Title     string         `yaml:"title"`
	TeamName  string         `yaml:"team_name"` // 兼容旧字段，与 title 二选一
	Members   []FileTeamUser `yaml:"members"`   // 可选：非空则只展示这些 user；空=全部
	StaticDir string         `yaml:"static_dir"`
	Pin       string         `yaml:"pin"` // 4 位数字访问密码；留空则不需要认证
}

// FileTeamUser 团队成员（对应路径 /pub/{user}/… 中的 user）。
type FileTeamUser struct {
	User    string `yaml:"user"`
	Display string `yaml:"display"`
}

// WebConfig 合并后的 Web + 存储配置。
type WebConfig struct {
	Listen     string
	Title      string
	Members    []TeamMember
	StaticDir  string
	CHDSN      string
	CHDatabase string
	// HTTP 上报 /pub 的 Basic Auth；与 auth.http_user / http_pass 一致；皆空则匿名。
	HTTPUser string
	HTTPPass string
	// Pin 4 位数字访问密码；空则无需认证。
	Pin string
	// 高德地图 Web Key与安全密钥（注入 /api/meta 供前端加载 JS API）。
	AmapKey        string
	AmapSecurityJs string
	// 高德 Web 服务 Key（服务端逆地理写入 district_adcode）；须为控制台「Web 服务」Key，勿填 JS 端 amap_key。
	AmapWebKey string
}

// TeamMember 运行时成员（含展示名）。
type TeamMember struct {
	User    string
	Display string
}

// File 扩展字段（与 config.yaml 对应）。
// 嵌入在 File 主结构体中。
