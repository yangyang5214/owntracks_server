package conf

import (
	"net"
	"net/url"
	"strconv"
	"strings"
)

const defaultClickHouseDB = "owntracks"

// FileClickHouse ClickHouse 连接。
// 优先使用 dsn（整串连接 URL）；否则在 host 非空时由 host/port/user/password/database 拼接原生协议 DSN。
// 仅填 database 而 host 为空时不会生成 DSN（与旧行为一致，避免误连本地）。
type FileClickHouse struct {
	DSN      string `yaml:"dsn"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

// ResolvedDSN 返回 clickhouse-go 连接串；未配置则空字符串。
func (c *FileClickHouse) ResolvedDSN() string {
	if c == nil {
		return ""
	}
	if s := strings.TrimSpace(c.DSN); s != "" {
		return s
	}
	if strings.TrimSpace(c.Host) == "" {
		return ""
	}
	host := strings.TrimSpace(c.Host)
	port := c.Port
	if port <= 0 {
		port = 9000
	}
	user := FirstNonEmpty(strings.TrimSpace(c.User), "default")
	db := FirstNonEmpty(strings.TrimSpace(c.Database), defaultClickHouseDB)

	u := &url.URL{
		Scheme: "clickhouse",
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		Path:   "/" + db,
	}
	if c.Password != "" {
		u.User = url.UserPassword(user, c.Password)
	} else {
		u.User = url.User(user)
	}
	return u.String()
}

// ClickHouseDSNWithDatabase 将 clickhouse://…/db 中的库名替换为 database（非空时）。
// 用于执行含 CREATE DATABASE 的脚本时先连到 default 等已存在的库。
func ClickHouseDSNWithDatabase(dsn, database string) (string, error) {
	database = strings.TrimSpace(database)
	if dsn == "" || database == "" {
		return dsn, nil
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "clickhouse", "http", "https":
		u.Path = "/" + strings.TrimPrefix(database, "/")
		return u.String(), nil
	default:
		return dsn, nil
	}
}
