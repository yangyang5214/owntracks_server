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
}

// Store 历史数据访问抽象。from/to 为 nil 表示全表时间范围。
type Store interface {
	Meta(ctx context.Context) TeamMeta
	Journey(ctx context.Context, from, to *time.Time, intervalSec int) (*JourneyResult, error)
	Stats(ctx context.Context, from, to *time.Time) (*StatsResult, error)
}
