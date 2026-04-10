package webapp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"owntracks_server/internal/amap"
	"owntracks_server/internal/conf"
	"owntracks_server/internal/owntracks"
	"owntracks_server/internal/store"
)

const maxPubBody = 1 << 20 // 1 MiB

func registerPubRoutes(mux *http.ServeMux, ch *store.CH, cfg *conf.WebConfig, lg *log.Helper) {
	pub := func(inner http.Handler) http.Handler {
		return httpAuth(cfg, inner)
	}

	mux.Handle("POST /pub/{user}/{device}", pub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := r.PathValue("user")
		device := r.PathValue("device")
		if user == "" || device == "" {
			http.Error(w, "missing user or device", http.StatusBadRequest)
			return
		}
		handlePub(w, r, ch, lg, cfg, user, device)
	})))

	mux.Handle("POST /pub", pub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, device, err := resolveUserDevice(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		handlePub(w, r, ch, lg, cfg, user, device)
	})))
}

func resolveUserDevice(r *http.Request) (user, device string, err error) {
	if t := r.URL.Query().Get("topic"); t != "" {
		u, d, ok := owntracks.SplitTopic(t)
		if ok {
			return u, d, nil
		}
		return "", "", fmt.Errorf("invalid topic query (expected owntracks/user/device)")
	}
	// Booklet: https://owntracks.org/booklet/tech/http/
	if u := r.Header.Get("X-Limit-U"); u != "" {
		if d := r.Header.Get("X-Limit-D"); d != "" {
			return u, d, nil
		}
	}
	if u := r.URL.Query().Get("u"); u != "" {
		if d := r.URL.Query().Get("d"); d != "" {
			return u, d, nil
		}
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, maxPubBody))
	if err != nil {
		return "", "", err
	}
	r.Body = io.NopCloser(strings.NewReader(string(b)))
	if topic, ok := owntracks.TopicFromJSON(b); ok {
		u, d, ok2 := owntracks.SplitTopic(topic)
		if ok2 {
			return u, d, nil
		}
	}
	return "", "", errNoTopic
}

var errNoTopic = errTopic("specify topic query, JSON \"topic\", or use POST /pub/{user}/{device}")

type errTopic string

func (e errTopic) Error() string { return string(e) }

func handlePub(w http.ResponseWriter, r *http.Request, ch *store.CH, lg *log.Helper, cfg *conf.WebConfig, user, device string) {
	b, err := io.ReadAll(io.LimitReader(r.Body, maxPubBody))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]\n"))
		return
	}
	topic := owntracks.Topic(user, device)
	parts, err := owntracks.SplitMessageBodies(b)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	for i, raw := range parts {
		typ, err := owntracks.PayloadType(raw)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if typ != "location" {
			continue
		}
		loc, err := owntracks.ParseLocation(raw)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var district *string
		if cfg.AmapWebKey != "" {
			rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			ad, err := amap.DistrictAdcode(rctx, cfg.AmapWebKey, loc.Lon, loc.Lat)
			cancel()
			if err != nil {
				lg.Debugf("amap regeo: %v", err)
			} else if ad != "" {
				district = &ad
			}
		}
		row := store.LocationRow{
			User:           user,
			Device:         device,
			Topic:          topic,
			EventTime:      loc.Tst,
			IngestSeq:      uint16(i),
			Lat:            loc.Lat,
			Lon:            loc.Lon,
			Acc:            loc.Acc,
			Alt:            loc.Alt,
			Vac:            loc.Vac,
			Vel:            loc.Vel,
			Cog:            loc.Cog,
			Dist:           loc.Dist,
			Tid:            loc.Tid,
			TType:          loc.TType,
			Trigger:        loc.Trigger,
			Battery:        loc.Battery,
			Charging:       loc.Charging,
			DistrictAdcode: district,
			PayloadJSON:    string(raw),
		}
		if err := ch.InsertLocation(ctx, row); err != nil {
			lg.Errorf("insert location: %v", err)
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("[]\n"))
}

func httpAuth(cfg *conf.WebConfig, next http.Handler) http.Handler {
	if cfg.HTTPUser == "" && cfg.HTTPPass == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != cfg.HTTPUser || p != cfg.HTTPPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="owntracks"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
