package store

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"

	"owntracks_server/internal/conf"
)

// CH 个人实例：默认读取库内全部位置；可选 members 白名单过滤 user。
type CH struct {
	DB        *sql.DB
	Database  string
	Title     string
	Members   []conf.TeamMember
	MaxPoints int
}

// Meta 实现 Store。
func (c *CH) Meta(ctx context.Context) TeamMeta {
	members := c.discoverMembers(ctx)
	return TeamMeta{
		Title:            c.Title,
		Members:          members,
		DefaultFullRange: true,
		DefaultDays:      0,
		MaxIntervalSec:   3600,
	}
}

func (c *CH) discoverMembers(ctx context.Context) []Member {
	if len(c.Members) > 0 {
		out := make([]Member, 0, len(c.Members))
		for i, m := range c.Members {
			d := m.Display
			if d == "" {
				d = m.User
			}
			out = append(out, Member{User: m.User, Display: d, Color: colorForIndex(i)})
		}
		return out
	}
	q := fmt.Sprintf(`
SELECT DISTINCT user
FROM %s.locations
ORDER BY user
`, identDB(c.Database))
	rows, err := c.DB.QueryContext(ctx, q)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var list []Member
	i := 0
	for rows.Next() {
		var u string
		if rows.Scan(&u) != nil {
			continue
		}
		list = append(list, Member{User: u, Display: u, Color: colorForIndex(i)})
		i++
	}
	return list
}

// Bounds 表内最早/最晚事件时间。
func (c *CH) Bounds(ctx context.Context) (minT, maxT time.Time, err error) {
	q := fmt.Sprintf(`
SELECT
  min(event_time),
  max(event_time)
FROM %s.locations
`, identDB(c.Database))
	row := c.DB.QueryRowContext(ctx, q)
	var mn, mx time.Time
	if err = row.Scan(&mn, &mx); err != nil {
		return
	}
	if mn.IsZero() || mx.IsZero() {
		err = fmt.Errorf("无位置数据")
		return
	}
	return mn.UTC(), mx.UTC(), nil
}

// Journey 按时间桶聚合；from/to 任一侧为空则用 Bounds 对应边界补全（默认全表）。
func (c *CH) Journey(ctx context.Context, from, to *time.Time, intervalSec int) (*JourneyResult, error) {
	var f, t time.Time
	bmin, bmax, err := c.Bounds(ctx)
	if err != nil {
		return nil, err
	}
	if from == nil {
		f = bmin
	} else {
		f = from.UTC()
	}
	if to == nil {
		t = bmax
	} else {
		t = to.UTC()
	}
	if !t.After(f) {
		t = f.Add(time.Minute)
	}
	if intervalSec < 30 {
		intervalSec = 30
	}
	maxPts := c.MaxPoints
	if maxPts <= 0 {
		maxPts = 4000
	}
	span := t.Sub(f)
	est := int(math.Ceil(span.Seconds() / float64(intervalSec)))
	if est > maxPts {
		intervalSec = int(math.Ceil(span.Seconds() / float64(maxPts)))
		if intervalSec < 30 {
			intervalSec = 30
		}
	}

	userClause := ""
	args := []any{f, t}
	if len(c.Members) > 0 {
		users := make([]string, 0, len(c.Members))
		for _, m := range c.Members {
			if safeIdentPart(m.User) {
				users = append(users, m.User)
			}
		}
		if len(users) == 0 {
			return nil, fmt.Errorf("成员列表无效")
		}
		userClause = " AND user IN (" + strings.Join(quoteStrings(users), ",") + ")"
	}

	q := fmt.Sprintf(`
SELECT
  user,
  device,
  toStartOfInterval(event_time, INTERVAL %d SECOND) AS bucket,
  argMax(lat, event_time) AS lat,
  argMax(lon, event_time) AS lon,
  max(event_time) AS ts
FROM %s.locations
WHERE event_time >= ? AND event_time <= ?`+userClause+`
GROUP BY user, device, bucket
ORDER BY user, device, bucket
`, intervalSec, identDB(c.Database))

	rows, err := c.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type key struct{ u, d string }
	buckets := make(map[key][]Point)
	for rows.Next() {
		var u, dev string
		var bucket time.Time
		var lat, lon float64
		var ts time.Time
		if err := rows.Scan(&u, &dev, &bucket, &lat, &lon, &ts); err != nil {
			return nil, err
		}
		k := key{u: u, d: dev}
		buckets[k] = append(buckets[k], Point{T: ts.UTC(), Lat: lat, Lon: lon})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	series := make([]Series, 0, len(buckets))
	idx := 0
	for k, pts := range buckets {
		label := fmt.Sprintf("%s / %s", k.u, k.d)
		series = append(series, Series{
			User:   k.u + "/" + k.d,
			Label:  label,
			Color:  colorForIndex(idx),
			Points: pts,
		})
		idx++
	}
	sort.Slice(series, func(i, j int) bool {
		ti, tj := time.Time{}, time.Time{}
		if len(series[i].Points) > 0 {
			ti = series[i].Points[0].T
		}
		if len(series[j].Points) > 0 {
			tj = series[j].Points[0].T
		}
		return ti.Before(tj)
	})

	return &JourneyResult{
		Title:        c.Title,
		From:         f.Format(time.RFC3339Nano),
		To:           t.Format(time.RFC3339Nano),
		IntervalSec:  intervalSec,
		IntervalNote: "按时间桶聚合；间隔会随时间跨度自动上调以控制点数",
		Series:       series,
	}, nil
}

// Stats 汇总。
func (c *CH) Stats(ctx context.Context, from, to *time.Time) (*StatsResult, error) {
	bmin, bmax, err := c.Bounds(ctx)
	if err != nil {
		return nil, err
	}
	var f, t time.Time
	if from == nil {
		f = bmin
	} else {
		f = from.UTC()
	}
	if to == nil {
		t = bmax
	} else {
		t = to.UTC()
	}

	userClause := ""
	args := []any{f, t}
	if len(c.Members) > 0 {
		users := make([]string, 0, len(c.Members))
		for _, m := range c.Members {
			if safeIdentPart(m.User) {
				users = append(users, m.User)
			}
		}
		if len(users) == 0 {
			return nil, fmt.Errorf("成员列表无效")
		}
		userClause = " AND user IN (" + strings.Join(quoteStrings(users), ",") + ")"
	}

	q := fmt.Sprintf(`
SELECT
  user,
  device,
  count() AS cnt,
  sumIf(toFloat64(dist), isNotNull(dist)) AS dist_m,
  max(event_time) AS last_seen
FROM %s.locations
WHERE event_time >= ? AND event_time <= ?`+userClause+`
GROUP BY user, device
ORDER BY user, device
`, identDB(c.Database))

	rows, err := c.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StatRow
	i := 0
	for rows.Next() {
		var u, dev string
		var cnt int64
		var dist sql.NullFloat64
		var last sql.NullTime
		if err := rows.Scan(&u, &dev, &cnt, &dist, &last); err != nil {
			return nil, err
		}
		dkm := 0.0
		if dist.Valid {
			dkm = dist.Float64 / 1000.0
		}
		var ls *time.Time
		if last.Valid {
			x := last.Time.UTC()
			ls = &x
		}
		out = append(out, StatRow{
			User:       u + "/" + dev,
			Display:    fmt.Sprintf("%s · %s", u, dev),
			Color:      colorForIndex(i),
			PointCount: cnt,
			DistanceKm: math.Round(dkm*10) / 10,
			LastSeen:   ls,
		})
		i++
	}
	return &StatsResult{
		Title: c.Title,
		From:  f.Format(time.RFC3339Nano),
		To:    t.Format(time.RFC3339Nano),
		Rows:  out,
	}, rows.Err()
}

func identDB(db string) string {
	if db == "" {
		return "owntracks"
	}
	for _, r := range db {
		if r != '_' && (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return "owntracks"
		}
	}
	return db
}

func safeIdentPart(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func quoteStrings(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	return out
}

// LocationRow HTTP 位置入库（与 locations 表一致）。
type LocationRow struct {
	User, Device, Topic string
	EventTime           time.Time
	IngestSeq           uint16
	Lat, Lon            float64
	Acc, Vel, Cog, Dist *float32
	Alt                 *int32
	Vac                 *uint16
	Tid, TType, Trigger *string
	Battery             *int16
	Charging            *uint8
	PayloadJSON         string
}

// InsertLocation 追加一条位置（HTTP 上报）。
func (c *CH) InsertLocation(ctx context.Context, row LocationRow) error {
	tbl := identDB(c.Database) + ".locations"
	q := `INSERT INTO ` + tbl + ` (
  user, device, topic, event_time, ingest_seq,
  lat, lon, acc, alt, vac, vel, cog, dist,
  tid, t, ` + "`trigger`" + `, battery, charging, payload_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := c.DB.ExecContext(ctx, q,
		row.User, row.Device, row.Topic, row.EventTime, row.IngestSeq,
		row.Lat, row.Lon, row.Acc, row.Alt, row.Vac, row.Vel, row.Cog, row.Dist,
		row.Tid, row.TType, row.Trigger, row.Battery, row.Charging, row.PayloadJSON,
	)
	return err
}
