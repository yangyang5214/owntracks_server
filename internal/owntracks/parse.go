package owntracks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SplitTopic 解析 owntracks/<user>/<device>。
func SplitTopic(topic string) (user, device string, ok bool) {
	topic = strings.TrimSpace(topic)
	parts := strings.Split(topic, "/")
	if len(parts) != 3 || parts[0] != "owntracks" {
		return "", "", false
	}
	if parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

// Topic 生成标准 topic。
func Topic(user, device string) string {
	return "owntracks/" + user + "/" + device
}

// SplitMessageBodies 支持单条 JSON 或 JSON 数组。
func SplitMessageBodies(raw []byte) ([][]byte, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty body")
	}
	if raw[0] == '[' {
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, err
		}
		out := make([][]byte, len(arr))
		for i, r := range arr {
			out[i] = []byte(r)
		}
		return out, nil
	}
	return [][]byte{raw}, nil
}

// LocationFields 从 OwnTracks location JSON 提取字段。
type LocationFields struct {
	Lat, Lon float64
	Tst      time.Time
	Acc      *float32
	Alt      *int32
	Vac      *uint16
	Vel      *float32
	Cog      *float32
	Dist     *float32
	Tid      *string
	TType    *string // JSON "t"
	Trigger  *string
	Battery  *int16
	Charging *uint8
}

type rawLoc struct {
	Type string  `json:"_type"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Tst  json.RawMessage `json:"tst"`
	Acc  *float64        `json:"acc"`
	Alt  *float64        `json:"alt"`
	Vac  *float64        `json:"vac"`
	Vel  *float64        `json:"vel"`
	Cog  *float64        `json:"cog"`
	Dist *float64        `json:"dist"`
	Tid     *string  `json:"tid"`
	T       *string  `json:"t"`
	Trigger *string  `json:"trigger"`
	Batt    *float64 `json:"batt"`
	Conn    connField `json:"conn"`
}

// connField 兼容 OwnTracks 扩展字段：字符串 "w"/"m"/"o"（Wi‑Fi / 移动数据 / 离线），
// 以及个别客户端曾使用的布尔值（仅布尔 true 时映射为 charging）。
type connField struct {
	Str string
	Bool *bool
}

func (c *connField) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		c.Str = s
		return nil
	}
	var b bool
	if err := json.Unmarshal(data, &b); err != nil {
		return err
	}
	c.Bool = &b
	return nil
}

// PayloadType 读取 JSON 的 _type 字段；用于在 HTTP 入站时区分 location 与其它类型。
func PayloadType(raw []byte) (string, error) {
	var m struct {
		Type string `json:"_type"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", err
	}
	return m.Type, nil
}

// ParseLocation 解析单条 location 消息；_type 须为 location。
func ParseLocation(raw []byte) (LocationFields, error) {
	var z LocationFields
	var m rawLoc
	if err := json.Unmarshal(raw, &m); err != nil {
		return z, err
	}
	if m.Type != "" && m.Type != "location" {
		return z, fmt.Errorf("unsupported _type %q", m.Type)
	}
	ts, err := parseTST(m.Tst)
	if err != nil {
		return z, err
	}
	z.Lat, z.Lon = m.Lat, m.Lon
	z.Tst = ts.UTC()
	if m.Acc != nil {
		v := float32(*m.Acc)
		z.Acc = &v
	}
	if m.Alt != nil {
		v := int32(*m.Alt)
		z.Alt = &v
	}
	if m.Vac != nil {
		v := uint16(*m.Vac)
		z.Vac = &v
	}
	if m.Vel != nil {
		v := float32(*m.Vel)
		z.Vel = &v
	}
	if m.Cog != nil {
		v := float32(*m.Cog)
		z.Cog = &v
	}
	if m.Dist != nil {
		v := float32(*m.Dist)
		z.Dist = &v
	}
	z.Tid = m.Tid
	z.TType = m.T
	z.Trigger = m.Trigger
	if m.Batt != nil {
		v := int16(*m.Batt)
		z.Battery = &v
	}
	if m.Conn.Bool != nil && *m.Conn.Bool {
		u := uint8(1)
		z.Charging = &u
	}
	return z, nil
}

func parseTST(raw json.RawMessage) (time.Time, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return time.Time{}, fmt.Errorf("missing tst")
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return time.Time{}, err
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return time.Time{}, err
		}
		return unixTST(f), nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return time.Time{}, err
	}
	return unixTST(f), nil
}

func unixTST(sec float64) time.Time {
	if sec > 1e12 {
		return time.UnixMilli(int64(sec)).UTC()
	}
	s := int64(sec)
	frac := sec - float64(s)
	return time.Unix(s, int64(frac*1e9)).UTC()
}
