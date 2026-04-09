package owntracks

import "encoding/json"

// TopicFromJSON 读取 JSON 中的 topic 字段（部分客户端在 body 中带 topic）。
func TopicFromJSON(raw []byte) (topic string, ok bool) {
	var m struct {
		Topic string `json:"topic"`
	}
	if err := json.Unmarshal(raw, &m); err != nil || m.Topic == "" {
		return "", false
	}
	return m.Topic, true
}
