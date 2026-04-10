(function () {
  "use strict";

  var PIN_TTL = 3600000;
  var PIN_STORAGE_KEY = "_pa";

  var map;
  var AMapRef = null;
  var flatPoints = [];
  var heatCellsCache = null;
  var heatmapLayer = null;
  var cornerPolygons = [];
  var cornerLightVisible = true;
  /** 递增以丢弃过期的异步区县查询结果 */
  var cornerLightGeneration = 0;
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

  function loadScript(src) {
    return new Promise(function (resolve, reject) {
      var s = document.createElement("script");
      s.src = src;
      s.async = true;
      s.onload = function () {
        resolve();
      };
      s.onerror = function () {
        reject(new Error("脚本加载失败: " + src));
      };
      document.head.appendChild(s);
    });
  }

  function showMapError(msg) {
    var el = document.getElementById("map-error");
    if (!el) return;
    el.textContent = msg;
    el.classList.remove("hidden");
  }

  function checkAuth() {
    fetch("/api/pin-status")
      .then(function (r) {
        return r.json();
      })
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
          } catch (e) {
            /* ignore */
          }
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

    setTimeout(function () {
      digits[0].focus();
    }, 100);

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
        for (var j = 0; j < digits.length; j++)
          digits[j].classList.remove("shake");
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
        .then(function (r) {
          return r.json();
        })
        .then(function (data) {
          if (data.ok) {
            pinToken = data.token;
            localStorage.setItem(
              PIN_STORAGE_KEY,
              JSON.stringify({
                fp: data.fingerprint,
                tk: data.token,
                ts: Date.now(),
              })
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
          if (e.key === "Backspace" && !this.value && idx > 0)
            digits[idx - 1].focus();
        });
        digits[idx].addEventListener("paste", function (e) {
          e.preventDefault();
          var text = (e.clipboardData || window.clipboardData)
            .getData("text")
            .replace(/\D/g, "")
            .slice(0, 4);
          for (var k = 0; k < 4; k++) digits[k].value = text[k] || "";
          if (text.length === 4) trySubmit();
          else if (text.length > 0)
            digits[Math.min(text.length, 3)].focus();
        });
      })(d);
    }
  }

  function startApp() {
    document.getElementById("pin-overlay").classList.add("hidden");
    var errEl = document.getElementById("map-error");
    if (errEl) {
      errEl.classList.add("hidden");
      errEl.textContent = "";
    }

    fetchJSON("/api/meta")
      .then(function (meta) {
        if (!meta.amap_key) {
          showMapError(
            "未配置高德地图 Key。请在 configs/config.yaml 最外层填写 amap_key 与 amap_security_js（见高德开放平台）。"
          );
          return null;
        }
        window._AMapSecurityConfig = {
          securityJsCode: meta.amap_security_js || "",
        };
        return loadScript("https://webapi.amap.com/loader.js").then(
          function () {
            return AMapLoader.load({
              key: meta.amap_key,
              version: "2.0",
              plugins: ["AMap.HeatMap", "AMap.DistrictSearch"],
            });
          }
        );
      })
      .then(function (AMap) {
        if (!AMap) return;
        AMapRef = AMap;
        setupControls();
        initMap(AMap);
      })
      .catch(function (e) {
        showMapError(
          e && e.message ? e.message : "地图初始化失败，请检查网络与 Key 配置。"
        );
      });
  }

  /** 与后端一致：经纬度量化到 8 位小数，同坐标保留时间最新的一条 */
  function dedupeFlatPointsByLatLon(points) {
    var m = {};
    for (var i = 0; i < points.length; i++) {
      var p = points[i];
      var key = p.lat.toFixed(8) + "," + p.lon.toFixed(8);
      var prev = m[key];
      if (!prev || p.t > prev.t) m[key] = p;
    }
    var out = [];
    for (var k in m) {
      if (Object.prototype.hasOwnProperty.call(m, k)) out.push(m[k]);
    }
    return out;
  }

  function clearCornerPolygons() {
    if (!map) return;
    for (var i = 0; i < cornerPolygons.length; i++) {
      map.remove(cornerPolygons[i]);
    }
    cornerPolygons = [];
  }

  /** 根据后端入库时写入的区县 adcode 拉边界（无逆地理，仅行政区查询）。 */
  function applyCornerLights(adcodes) {
    var gen = ++cornerLightGeneration;
    clearCornerPolygons();
    if (!map || !AMapRef || !adcodes || adcodes.length === 0) return;

    var districtSearch = new AMapRef.DistrictSearch({
      extensions: "all",
      level: "district",
    });

    function step(i) {
      if (i >= adcodes.length) return Promise.resolve();
      if (cornerLightGeneration !== gen) return Promise.resolve();
      var adcode = adcodes[i];
      return new Promise(function (resolve) {
        districtSearch.search(String(adcode), function (status, result) {
          if (cornerLightGeneration !== gen) return resolve();
          if (
            status === "complete" &&
            result.districtList &&
            result.districtList[0]
          ) {
            var boundaries = result.districtList[0].boundaries;
            if (boundaries && boundaries.length) {
              for (var b = 0; b < boundaries.length; b++) {
                var poly = new AMapRef.Polygon({
                  path: boundaries[b],
                  fillColor: "rgba(77, 184, 255, 0.32)",
                  strokeColor: "rgba(120, 215, 255, 0.65)",
                  strokeWeight: 1,
                  zIndex: 120,
                });
                map.add(poly);
                cornerPolygons.push(poly);
                if (!cornerLightVisible) poly.hide();
              }
            }
          }
          step(i + 1).then(resolve);
        });
      });
    }

    step(0).catch(function () {
      /* 行政区查询失败时静默 */
    });
  }

  function applyCornerVisibility() {
    for (var i = 0; i < cornerPolygons.length; i++) {
      if (cornerLightVisible) cornerPolygons[i].show();
      else cornerPolygons[i].hide();
    }
  }

  function buildHeatmapDataSet() {
    var data = [];
    var maxC = 1;
    if (heatCellsCache && heatCellsCache.length) {
      for (var i = 0; i < heatCellsCache.length; i++) {
        var c = heatCellsCache[i];
        var cnt = Math.max(1, Math.round(c.w));
        if (cnt > maxC) maxC = cnt;
        data.push({ lng: c.lon, lat: c.lat, count: cnt });
      }
    } else {
      for (var j = 0; j < flatPoints.length; j++) {
        var p = flatPoints[j];
        data.push({ lng: p.lon, lat: p.lat, count: 1 });
      }
      maxC = 1;
    }
    return { max: maxC, data: data };
  }

  function refreshHeatmapData() {
    if (!heatmapLayer) return;
    heatmapLayer.setDataSet(buildHeatmapDataSet());
  }

  function initMap(AMap) {
    map = new AMap.Map("map", {
      zoom: 4,
      center: [104.0, 35.0],
      viewMode: "2D",
      mapStyle: "amap://styles/darkblue",
      showLabel: true,
    });

    map.plugin(["AMap.HeatMap", "AMap.DistrictSearch"], function () {
      heatmapLayer = new AMap.HeatMap(map, {
        radius: 38,
        opacity: [0, 0.78],
        gradient: {
          0.25: "rgb(0, 45, 110)",
          0.45: "rgb(0, 120, 220)",
          0.65: "rgb(77, 184, 255)",
          0.85: "rgb(160, 225, 255)",
          1.0: "rgb(255, 255, 255)",
        },
        zIndex: 80,
      });
      heatmapLayer.setDataSet({ max: 1, data: [] });
      loadData();
    });
  }

  function loadData() {
    Promise.all([
      fetchJSON("/api/journey"),
      fetchJSON("/api/heatmap").catch(function () {
        return null;
      }),
    ])
      .then(function (res) {
        processJourney(res[0], res[1]);
      })
      .catch(function () {
        /* ignore */
      });
  }

  function processJourney(j, heatResp) {
    flatPoints = [];
    heatCellsCache =
      heatResp && heatResp.cells && heatResp.cells.length > 0
        ? heatResp.cells
        : null;

    var minLng = Infinity;
    var minLat = Infinity;
    var maxLng = -Infinity;
    var maxLat = -Infinity;
    var has = false;

    var series = j.series || [];
    for (var si = 0; si < series.length; si++) {
      var s = series[si];
      var raw = s.points || [];
      var pts = [];
      for (var pi = 0; pi < raw.length; pi++) {
        pts.push({
          t: new Date(raw[pi].t),
          lat: raw[pi].lat,
          lon: raw[pi].lon,
        });
      }
      if (pts.length === 0) continue;

      for (var k = 0; k < pts.length; k++) {
        var lng = pts[k].lon;
        var la = pts[k].lat;
        if (lng < minLng) minLng = lng;
        if (lng > maxLng) maxLng = lng;
        if (la < minLat) minLat = la;
        if (la > maxLat) maxLat = la;
        flatPoints.push({
          t: pts[k].t,
          lat: la,
          lon: lng,
        });
        has = true;
      }
    }

    flatPoints = dedupeFlatPointsByLatLon(flatPoints);
    flatPoints.sort(function (a, b) {
      return a.t - b.t;
    });

    if (has && map && AMapRef) {
      if (minLng === maxLng && minLat === maxLat) {
        map.setZoomAndCenter(14, [minLng, minLat]);
      } else {
        var sw = new AMapRef.LngLat(minLng, minLat);
        var ne = new AMapRef.LngLat(maxLng, maxLat);
        var bounds = new AMapRef.Bounds(sw, ne);
        map.setBounds(bounds, false, [60, 60, 60, 60]);
      }
    }

    refreshHeatmapData();
    applyCornerLights(j.district_adcodes || []);
  }

  function setupControls() {
    document.getElementById("btn-corner").addEventListener("click", function () {
      cornerLightVisible = !cornerLightVisible;
      this.classList.toggle("active", cornerLightVisible);
      applyCornerVisibility();
    });

    document.getElementById("btn-zin").addEventListener("click", function () {
      if (map) map.zoomIn();
    });
    document.getElementById("btn-zout").addEventListener("click", function () {
      if (map) map.zoomOut();
    });
  }

  checkAuth();
})();
