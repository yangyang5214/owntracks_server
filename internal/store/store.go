package store

import (
	"context"
	"time"
)

// Member 对外 JSON。
type Member struct {
	User    string `json:"user"`
	Display string `json:"display"`
	Color   string `json:"color"`
}

// Point 轨迹点（已降采样）。
type Point struct {
	T   time.Time `json:"t"`
	Lat float64   `json:"lat"`
	Lon float64   `json:"lon"`
}

// Series 单成员轨迹序列。
type Series struct {
	User   string  `json:"user"`
	Label  string  `json:"label"`
	Color  string  `json:"color"`
	Points []Point `json:"points"`
}

// JourneyResult GET /api/journey。
type JourneyResult struct {
	Title        string   `json:"title"`
	From         string   `json:"from"`
	To           string   `json:"to"`
	IntervalSec  int      `json:"interval_sec"`
	IntervalNote string   `json:"interval_note"`
	Series       []Series `json:"series"`
	// DistrictAdcodes 轨迹时间范围内出现过的区县 adcode（依赖入库逆地理写入 locations.district_adcode）。
	DistrictAdcodes []string `json:"district_adcodes,omitempty"`
}

// StatRow 成员统计。
type StatRow struct {
	User       string     `json:"user"`
	Display    string     `json:"display"`
	Color      string     `json:"color"`
	PointCount int64      `json:"point_count"`
	DistanceKm float64    `json:"distance_km"`
	LastSeen   *time.Time `json:"last_seen,omitempty"`
}

// StatsResult GET /api/stats。
type StatsResult struct {
	Title string    `json:"title"`
	From  string    `json:"from"`
	To    string    `json:"to"`
	Rows  []StatRow `json:"rows"`
}

// TeamMeta GET /api/meta。
type TeamMeta struct {
	Title            string   `json:"title"`
	Members          []Member `json:"members"`
	DefaultFullRange bool     `json:"default_full_range"`
	DefaultDays      int      `json:"default_days"`
	MaxIntervalSec   int      `json:"max_interval_sec"`
	// 以下两项由 webapp 从配置注入，非 ClickHouse 查询结果。
	AmapKey        string `json:"amap_key,omitempty"`
	AmapSecurityJs string `json:"amap_security_js,omitempty"`
}

// HeatmapCell 预聚合网格单元（用于前端热力图层）。
type HeatmapCell struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
	W   float64 `json:"w"` // 原始采样点数之和，越大颜色越热
}

// HeatmapResult GET /api/heatmap。
type HeatmapResult struct {
	Title       string        `json:"title"`
	From        string        `json:"from"`
	To          string        `json:"to"`
	GridMeters  float64       `json:"grid_meters_approx"` // 赤道附近约等效边长，米
	CellNote    string        `json:"cell_note"`
	Cells       []HeatmapCell `json:"cells"`
	Precomputed bool          `json:"precomputed"` // true：来自 heatmap_grid 物化汇总
}

// Store 历史数据访问抽象。from/to 为 nil 表示全表时间范围。
type Store interface {
	Meta(ctx context.Context) TeamMeta
	Journey(ctx context.Context, from, to *time.Time, intervalSec int) (*JourneyResult, error)
	Stats(ctx context.Context, from, to *time.Time) (*StatsResult, error)
	Heatmap(ctx context.Context, from, to *time.Time, minCount int) (*HeatmapResult, error)
}
