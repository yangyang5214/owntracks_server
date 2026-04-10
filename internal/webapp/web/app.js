(function () {
  "use strict";

  var PIN_TTL = 3600000;
  var PIN_STORAGE_KEY = "_pa";

  var HEAT_SOURCE = "tracks-heat";
  var LINE_SOURCE = "tracks-line";
  /** 双层热力：外圈光晕 + 内圈高亮，接近 Strava 蓝色热力 */
  var HEAT_LAYERS = ["tracks-heat-glow", "tracks-heat-core"];
  var LINE_LAYER = "tracks-line-layer";

  /**
   * Strava maps ?style=dark：深色底图 + 道路/地名标注（Carto dark_all，OSM 数据）
   */
  var DARK_STYLE = {
    version: 8,
    sources: {
      carto_dark: {
        type: "raster",
        tiles: [
          "https://a.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}.png",
          "https://b.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}.png",
          "https://c.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}.png",
        ],
        tileSize: 256,
        maxzoom: 19,
        attribution:
          '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/attributions">CARTO</a>',
      },
    },
    layers: [{ id: "carto_dark", type: "raster", source: "carto_dark" }],
  };

  var SAT_STYLE = {
    version: 8,
    sources: {
      sat: {
        type: "raster",
        tiles: [
          "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}",
        ],
        tileSize: 256,
        maxzoom: 18,
      },
    },
    layers: [{ id: "sat", type: "raster", source: "sat" }],
  };

  /** Strava gColor=blue 风格：低密深蓝 → 高密电蓝 → 白芯 */
  var STRAVA_BLUE_HEAT = [
    "interpolate",
    ["linear"],
    ["heatmap-density"],
    0,
    "rgba(0, 12, 32, 0)",
    0.06,
    "rgba(0, 45, 110, 0.18)",
    0.18,
    "rgba(0, 85, 190, 0.5)",
    0.35,
    "rgba(0, 140, 245, 0.78)",
    0.52,
    "rgba(40, 180, 255, 0.9)",
    0.7,
    "rgba(120, 215, 255, 0.96)",
    0.88,
    "rgba(210, 240, 255, 0.99)",
    1,
    "rgba(255, 255, 255, 1)",
  ];

  /** CK 预聚合 w 为点数，sqrt 后作为热力权重，避免单格过大压满 */
  var HEAT_WEIGHT_EXPR = [
    "min",
    5,
    ["+", 0.2, ["*", 0.5, ["sqrt", ["max", ["get", "w"], 0]]]],
  ];

  var map;
  var flatPoints = [];
  /** 非空时热力图层使用 /api/heatmap 预计算格网，时间轴只驱动轨迹线 */
  var heatCellsCache = null;
  var heatEnabled = true;
  var satMode = false;
  var playTimer = null;
  var curMarker = null;
  var pinToken = "";

  function fetchJSON(url) {
    var finalUrl = url;
    if (pinToken && url.indexOf("/api/pin") === -1) {
      var sep = url.indexOf("?") >= 0 ? "&" : "?";
      finalUrl = url + sep + "_t=" + pinToken;
    }
    return fetch(finalUrl).then(function (r) {
      if (r.status === 401) {
        localStorage.removeItem(PIN_STORAGE_KEY);
        location.reload();
        return Promise.reject(new Error("认证已过期"));
      }
      if (!r.ok)
        return r.text().then(function (t) {
          throw new Error(t || r.statusText);
        });
      return r.json();
    });
  }

  function timeAgo(d) {
    var ms = Date.now() - d.getTime();
    var m = Math.floor(ms / 60000);
    if (m < 1) return "刚刚";
    if (m < 60) return m + " 分钟前";
    var h = Math.floor(m / 60);
    if (h < 24) return h + " 小时前";
    var dd = Math.floor(h / 24);
    if (dd < 30) return dd + " 天前";
    return d.toLocaleDateString();
  }

  function fmtDist(km) {
    if (km >= 10000) return (km / 10000).toFixed(1) + " 万 km";
    if (km >= 1000) return (km / 1000).toFixed(1) + "k km";
    return Math.round(km) + " km";
  }

  function fmtNum(n) {
    if (n >= 100000) return (n / 10000).toFixed(1) + " 万";
    if (n >= 10000) return (n / 1000).toFixed(1) + "k";
    return n.toLocaleString();
  }

  function checkAuth() {
    fetch("/api/pin-status")
      .then(function (r) { return r.json(); })
      .then(function (status) {
        if (!status.required) {
          startApp();
          return;
        }
        var raw = localStorage.getItem(PIN_STORAGE_KEY);
        if (raw) {
          try {
            var cached = JSON.parse(raw);
            if (
              cached.fp === status.fingerprint &&
              Date.now() - cached.ts < PIN_TTL
            ) {
              pinToken = cached.tk;
              startApp();
              return;
            }
          } catch (e) { /* ignore */ }
        }
        localStorage.removeItem(PIN_STORAGE_KEY);
        showPinOverlay();
      })
      .catch(function () {
        startApp();
      });
  }

  function showPinOverlay() {
    var overlay = document.getElementById("pin-overlay");
    overlay.classList.remove("hidden");
    var digits = overlay.querySelectorAll(".pin-digit");
    var errorEl = document.getElementById("pin-error");

    for (var i = 0; i < digits.length; i++) digits[i].value = "";
    errorEl.textContent = "";

    setTimeout(function () { digits[0].focus(); }, 100);

    function getPin() {
      var v = "";
      for (var j = 0; j < digits.length; j++) v += digits[j].value;
      return v;
    }

    function shakeAndClear(msg) {
      errorEl.textContent = msg;
      for (var j = 0; j < digits.length; j++) {
        digits[j].classList.add("shake");
        digits[j].value = "";
      }
      setTimeout(function () {
        for (var j = 0; j < digits.length; j++) digits[j].classList.remove("shake");
        digits[0].focus();
      }, 450);
    }

    function trySubmit() {
      var pin = getPin();
      if (pin.length !== 4) return;

      fetch("/api/verify-pin", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ pin: pin }),
      })
        .then(function (r) { return r.json(); })
        .then(function (data) {
          if (data.ok) {
            pinToken = data.token;
            localStorage.setItem(
              PIN_STORAGE_KEY,
              JSON.stringify({ fp: data.fingerprint, tk: data.token, ts: Date.now() })
            );
            startApp();
          } else {
            shakeAndClear("密码错误，请重试");
          }
        })
        .catch(function () {
          shakeAndClear("网络错误");
        });
    }

    for (var d = 0; d < digits.length; d++) {
      (function (idx) {
        digits[idx].addEventListener("input", function () {
          this.value = this.value.replace(/\D/g, "").slice(0, 1);
          if (this.value && idx < 3) digits[idx + 1].focus();
          if (getPin().length === 4) trySubmit();
        });
        digits[idx].addEventListener("keydown", function (e) {
          if (e.key === "Backspace" && !this.value && idx > 0) digits[idx - 1].focus();
        });
        digits[idx].addEventListener("paste", function (e) {
          e.preventDefault();
          var text = (e.clipboardData || window.clipboardData).getData("text").replace(/\D/g, "").slice(0, 4);
          for (var k = 0; k < 4; k++) digits[k].value = text[k] || "";
          if (text.length === 4) trySubmit();
          else if (text.length > 0) digits[Math.min(text.length, 3)].focus();
        });
      })(d);
    }
  }

  function startApp() {
    document.getElementById("pin-overlay").classList.add("hidden");
    initMap();
    setupControls();
  }

  function getTimelineEndIndex() {
    if (!flatPoints.length) return -1;
    var tl = document.getElementById("tl");
    return Math.min(Number(tl.value), flatPoints.length - 1);
  }

  function buildPointFeatures(upToIndex) {
    var features = [];
    var n = flatPoints.length;
    if (n === 0 || upToIndex < 0) return features;
    var end = Math.min(upToIndex, n - 1);
    for (var i = 0; i <= end; i++) {
      var p = flatPoints[i];
      features.push({
        type: "Feature",
        geometry: { type: "Point", coordinates: [p.lon, p.lat] },
        properties: { w: 1 },
      });
    }
    return features;
  }

  function buildLineFeature(upToIndex) {
    var n = flatPoints.length;
    if (n === 0 || upToIndex < 0) return null;
    var end = Math.min(upToIndex, n - 1);
    if (end < 1) return null;
    var coords = [];
    for (var i = 0; i <= end; i++) {
      coords.push([flatPoints[i].lon, flatPoints[i].lat]);
    }
    return {
      type: "Feature",
      geometry: { type: "LineString", coordinates: coords },
      properties: {},
    };
  }

  function buildHeatFeaturesFromCells(cells) {
    var f = [];
    for (var i = 0; i < cells.length; i++) {
      var c = cells[i];
      f.push({
        type: "Feature",
        geometry: { type: "Point", coordinates: [c.lon, c.lat] },
        properties: { w: c.w },
      });
    }
    return f;
  }

  function setTracksData(upToIndex) {
    if (!map) return;
    if (!heatCellsCache || !heatCellsCache.length) {
      if (map.getSource(HEAT_SOURCE)) {
        map.getSource(HEAT_SOURCE).setData({
          type: "FeatureCollection",
          features: buildPointFeatures(upToIndex),
        });
      }
    }
    if (map.getSource(LINE_SOURCE)) {
      var line = buildLineFeature(upToIndex);
      map.getSource(LINE_SOURCE).setData({
        type: "FeatureCollection",
        features: line ? [line] : [],
      });
    }
  }

  function setHeatLayersVisibility(visible) {
    var vis = visible ? "visible" : "none";
    for (var i = 0; i < HEAT_LAYERS.length; i++) {
      if (map.getLayer(HEAT_LAYERS[i])) {
        map.setLayoutProperty(HEAT_LAYERS[i], "visibility", vis);
      }
    }
    if (map.getLayer(LINE_LAYER)) {
      map.setLayoutProperty(LINE_LAYER, "visibility", vis);
    }
  }

  function ensureTrackLayers() {
    if (map.getSource(HEAT_SOURCE)) return;

    map.addSource(HEAT_SOURCE, {
      type: "geojson",
      data: { type: "FeatureCollection", features: [] },
    });

    map.addSource(LINE_SOURCE, {
      type: "geojson",
      data: { type: "FeatureCollection", features: [] },
    });

    map.addLayer({
      id: HEAT_LAYERS[0],
      type: "heatmap",
      source: HEAT_SOURCE,
      paint: {
        "heatmap-weight": HEAT_WEIGHT_EXPR,
        "heatmap-intensity": [
          "interpolate",
          ["linear"],
          ["zoom"],
          0,
          0.45,
          4,
          0.75,
          8,
          1.1,
          12,
          1.45,
          16,
          1.75,
        ],
        "heatmap-radius": [
          "interpolate",
          ["linear"],
          ["zoom"],
          0,
          5,
          4,
          14,
          8,
          32,
          12,
          52,
          16,
          78,
        ],
        "heatmap-opacity": [
          "interpolate",
          ["linear"],
          ["zoom"],
          10,
          0.62,
          14,
          0.52,
          18,
          0.42,
        ],
        "heatmap-color": STRAVA_BLUE_HEAT,
      },
    });

    map.addLayer({
      id: HEAT_LAYERS[1],
      type: "heatmap",
      source: HEAT_SOURCE,
      paint: {
        "heatmap-weight": HEAT_WEIGHT_EXPR,
        "heatmap-intensity": [
          "interpolate",
          ["linear"],
          ["zoom"],
          0,
          0.85,
          4,
          1.15,
          8,
          1.75,
          12,
          2.35,
          16,
          2.85,
        ],
        "heatmap-radius": [
          "interpolate",
          ["linear"],
          ["zoom"],
          0,
          2,
          4,
          6,
          8,
          14,
          12,
          24,
          16,
          34,
        ],
        "heatmap-opacity": [
          "interpolate",
          ["linear"],
          ["zoom"],
          10,
          0.95,
          14,
          0.92,
          18,
          0.88,
        ],
        "heatmap-color": STRAVA_BLUE_HEAT,
      },
    });

    map.addLayer({
      id: LINE_LAYER,
      type: "line",
      source: LINE_SOURCE,
      minzoom: 9,
      layout: {
        "line-join": "round",
        "line-cap": "round",
      },
      paint: {
        "line-color": "rgba(100, 200, 255, 0.92)",
        "line-width": [
          "interpolate",
          ["linear"],
          ["zoom"],
          9,
          1.2,
          12,
          2.5,
          14,
          4,
          18,
          7,
        ],
        "line-opacity": [
          "interpolate",
          ["linear"],
          ["zoom"],
          9,
          0.12,
          11,
          0.35,
          14,
          0.55,
          18,
          0.7,
        ],
        "line-blur": [
          "interpolate",
          ["linear"],
          ["zoom"],
          9,
          1.2,
          14,
          0.55,
          18,
          0.35,
        ],
      },
    });

    setHeatLayersVisibility(heatEnabled);
  }

  function syncTracksFromTimeline() {
    setTracksData(getTimelineEndIndex());
  }

  function initMap() {
    map = new maplibregl.Map({
      container: "map",
      style: DARK_STYLE,
      center: [104, 35],
      zoom: 3,
      attributionControl: false,
      maxPitch: 0,
    });

    map.on("load", function () {
      ensureTrackLayers();
      loadData();
    });
  }

  function loadData() {
    Promise.all([
      fetchJSON("/api/journey"),
      fetchJSON("/api/heatmap").catch(function () { return null; }),
      fetchJSON("/api/stats").catch(function () { return null; }),
    ])
      .then(function (res) {
        processJourney(res[0], res[1]);
        if (res[2]) processStats(res[2]);
        document.body.classList.remove("loading");
      })
      .catch(function (e) {
        document.getElementById("subtitle").textContent =
          "加载失败: " + (e.message || e);
        document.body.classList.remove("loading");
      });
  }

  function processJourney(j, heatResp) {
    flatPoints = [];
    heatCellsCache =
      heatResp && heatResp.cells && heatResp.cells.length > 0 ? heatResp.cells : null;

    document.getElementById("title").textContent = j.title || "轨迹热力图";

    var bounds = new maplibregl.LngLatBounds();
    var has = false;

    var series = j.series || [];
    for (var si = 0; si < series.length; si++) {
      var s = series[si];
      var raw = s.points || [];
      var pts = [];
      for (var pi = 0; pi < raw.length; pi++) {
        pts.push({ t: new Date(raw[pi].t), lat: raw[pi].lat, lon: raw[pi].lon });
      }
      if (pts.length === 0) continue;

      for (var k = 0; k < pts.length; k++) {
        bounds.extend([pts[k].lon, pts[k].lat]);
        flatPoints.push({
          t: pts[k].t,
          lat: pts[k].lat,
          lon: pts[k].lon,
          color: s.color,
          label: s.label,
        });
        has = true;
      }
    }

    flatPoints.sort(function (a, b) { return a.t - b.t; });

    if (has) {
      map.fitBounds(bounds, { padding: 60, maxZoom: 14, duration: 1500 });
      var from = new Date(j.from);
      var to = new Date(j.to);
      var sub =
        from.toLocaleDateString() + " — " + to.toLocaleDateString() +
        " · " + fmtNum(flatPoints.length) + " 点";
      if (heatCellsCache) {
        sub += " · 热力CK预聚合";
        if (heatResp.grid_meters_approx) {
          sub += " ~" + heatResp.grid_meters_approx + "m格";
        }
      }
      document.getElementById("subtitle").textContent = sub;
    } else {
      document.getElementById("subtitle").textContent = "暂无轨迹数据";
    }

    ensureTrackLayers();
    setupTimeline();
    if (heatCellsCache && map.getSource(HEAT_SOURCE)) {
      map.getSource(HEAT_SOURCE).setData({
        type: "FeatureCollection",
        features: buildHeatFeaturesFromCells(heatCellsCache),
      });
    }
  }

  function processStats(s) {
    var dist = 0, pts = 0, last = null;
    var rows = s.rows || [];
    for (var i = 0; i < rows.length; i++) {
      var r = rows[i];
      dist += r.distance_km || 0;
      pts += r.point_count || 0;
      if (r.last_seen) {
        var d = new Date(r.last_seen);
        if (!last || d > last) last = d;
      }
    }
    document.getElementById("s-dist").textContent = fmtDist(dist);
    document.getElementById("s-pts").textContent = fmtNum(pts);
    document.getElementById("s-last").textContent = last ? timeAgo(last) : "--";
  }

  function setupTimeline() {
    var tl = document.getElementById("tl");
    if (!flatPoints.length) {
      tl.max = 1;
      tl.value = 1;
      return;
    }
    tl.max = flatPoints.length - 1;
    tl.value = tl.max;

    tl.removeEventListener("input", onTimelineInput);
    tl.addEventListener("input", onTimelineInput);
    onTimelineInput();
  }

  function onTimelineInput() {
    var idx = getTimelineEndIndex();
    if (idx < 0 || !flatPoints.length) return;

    var p = flatPoints[idx];
    var pct = Math.round((idx / Math.max(1, flatPoints.length - 1)) * 100);

    document.getElementById("tl-time").textContent = p.t.toLocaleString();
    document.getElementById("tl-pct").textContent = pct + "%";

    updateMarker(p);
    setTracksData(idx);
  }

  function updateMarker(p) {
    if (curMarker) curMarker.remove();

    var el = document.createElement("div");
    el.style.position = "relative";
    el.innerHTML = '<div class="marker-pulse"></div><div class="marker-dot"></div>';

    curMarker = new maplibregl.Marker({ element: el, anchor: "center" })
      .setLngLat([p.lon, p.lat])
      .setPopup(
        new maplibregl.Popup({ offset: 14, closeButton: false, closeOnClick: false }).setHTML(
          '<div class="popup-label">' + p.label + "</div>" +
          '<div class="popup-time">' + p.t.toLocaleString() + "</div>"
        )
      )
      .addTo(map);

    curMarker.togglePopup();
  }

  function setupControls() {
    var btnPlay = document.getElementById("btn-play");
    var pauseSVG = '<svg viewBox="0 0 24 24"><path d="M6 19h4V5H6zm8-14v14h4V5z"/></svg>';
    var playSVG = '<svg viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>';

    btnPlay.addEventListener("click", function () {
      if (playTimer) {
        clearInterval(playTimer);
        playTimer = null;
        btnPlay.classList.remove("active");
        btnPlay.innerHTML = playSVG;
        return;
      }
      btnPlay.classList.add("active");
      btnPlay.innerHTML = pauseSVG;
      var tl = document.getElementById("tl");
      var i = Number(tl.value);
      playTimer = setInterval(function () {
        i += 1;
        if (i > Number(tl.max)) i = 0;
        tl.value = i;
        onTimelineInput();
      }, 60);
    });

    document.getElementById("btn-reset").addEventListener("click", function () {
      stopPlay();
      document.getElementById("tl").value = 0;
      onTimelineInput();
    });

    document.getElementById("btn-end").addEventListener("click", function () {
      stopPlay();
      var tl = document.getElementById("tl");
      tl.value = tl.max;
      onTimelineInput();
    });

    document.getElementById("btn-heat").addEventListener("click", function () {
      heatEnabled = !heatEnabled;
      this.classList.toggle("active", heatEnabled);
      if (map) setHeatLayersVisibility(heatEnabled);
    });

    document.getElementById("btn-sat").addEventListener("click", function () {
      satMode = !satMode;
      this.classList.toggle("active", satMode);
      map.setStyle(satMode ? SAT_STYLE : DARK_STYLE);
      map.once("style.load", function () {
        ensureTrackLayers();
        if (heatCellsCache && heatCellsCache.length) {
          map.getSource(HEAT_SOURCE).setData({
            type: "FeatureCollection",
            features: buildHeatFeaturesFromCells(heatCellsCache),
          });
        }
        syncTracksFromTimeline();
        setHeatLayersVisibility(heatEnabled);
      });
    });

    document.getElementById("btn-zin").addEventListener("click", function () {
      map.zoomIn({ duration: 300 });
    });
    document.getElementById("btn-zout").addEventListener("click", function () {
      map.zoomOut({ duration: 300 });
    });
  }

  function stopPlay() {
    if (playTimer) {
      clearInterval(playTimer);
      playTimer = null;
      var btn = document.getElementById("btn-play");
      btn.classList.remove("active");
      btn.innerHTML = '<svg viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>';
    }
  }

  checkAuth();
})();
