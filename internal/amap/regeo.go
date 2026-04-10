package amap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const regeoEndpoint = "https://restapi.amap.com/v3/geocode/regeo"

var regeoHTTPClient = &http.Client{Timeout: 4 * time.Second}

type regeoResponse struct {
	Status    string `json:"status"`
	Info      string `json:"info"`
	Regeocode *struct {
		AddressComponent *struct {
			Adcode string `json:"adcode"`
		} `json:"addressComponent"`
	} `json:"regeocode"`
}

// DistrictAdcode 调用高德 Web 服务逆地理，返回区县六位 adcode（失败或境外等返回空字符串）。
func DistrictAdcode(ctx context.Context, apiKey string, lon, lat float64) (string, error) {
	if apiKey == "" {
		return "", nil
	}
	key := coordKey(lon, lat)
	if v, ok := regeoCacheLoad(key); ok {
		return v, nil
	}

	u, err := url.Parse(regeoEndpoint)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("key", apiKey)
	q.Set("location", fmt.Sprintf("%.6f,%.6f", lon, lat))
	q.Set("extensions", "base")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := regeoHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var out regeoResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("regeo json: %w", err)
	}
	if out.Status != "1" {
		return "", fmt.Errorf("regeo status=%s info=%s", out.Status, out.Info)
	}
	if out.Regeocode == nil || out.Regeocode.AddressComponent == nil {
		return "", nil
	}
	ad := strings.TrimSpace(out.Regeocode.AddressComponent.Adcode)
	if len(ad) < 6 {
		return "", nil
	}
	regeoCacheStore(key, ad)
	return ad, nil
}

func coordKey(lon, lat float64) string {
	return fmt.Sprintf("%.4f,%.4f", lon, lat)
}

var (
	regeoMu    sync.Mutex
	regeoCache map[string]string // 约百米级量化，减轻重复坐标配额消耗
)

const regeoCacheMax = 2048

func regeoCacheLoad(k string) (string, bool) {
	regeoMu.Lock()
	defer regeoMu.Unlock()
	if regeoCache == nil {
		return "", false
	}
	v, ok := regeoCache[k]
	return v, ok
}

func regeoCacheStore(k, ad string) {
	regeoMu.Lock()
	defer regeoMu.Unlock()
	if regeoCache == nil {
		regeoCache = make(map[string]string, 256)
	}
	if len(regeoCache) >= regeoCacheMax {
		regeoCache = make(map[string]string, 256)
	}
	regeoCache[k] = ad
}
