/* global L */

async function fetchJSON(path) {
  const r = await fetch(path);
  if (!r.ok) {
    const t = await r.text();
    throw new Error(t || r.statusText);
  }
  return r.json();
}

function parseTime(s) {
  const d = Date.parse(s);
  if (Number.isNaN(d)) throw new Error("bad time");
  return new Date(d);
}

function main() {
  const map = L.map("map", { zoomControl: true, attributionControl: false });
  L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
    maxZoom: 19,
    subdomains: "abc",
  }).addTo(map);

  const layers = [];
  let flatPoints = [];
  let playTimer = null;

  const tl = document.getElementById("tl");
  const tlRange = document.getElementById("tl-range");

  function renderJourney(j) {
    layers.forEach((l) => map.removeLayer(l));
    layers.length = 0;
    flatPoints = [];

    document.getElementById("title").textContent = j.title || "足迹放映机";
    document.getElementById("subtitle").textContent =
      `区间 ${new Date(j.from).toLocaleString()} — ${new Date(j.to).toLocaleString()} · 间隔 ${j.interval_sec}s`;

    const bounds = [];
    let maxLen = 0;
    for (const s of j.series || []) {
      const latlngs = (s.points || []).map((p) => [p.lat, p.lon]);
      if (latlngs.length > maxLen) maxLen = latlngs.length;
      for (const p of s.points || []) {
        flatPoints.push({
          t: parseTime(p.t),
          lat: p.lat,
          lon: p.lon,
          color: s.color,
          label: s.label,
        });
      }
      if (latlngs.length === 0) continue;
      const poly = L.polyline(latlngs, {
        color: s.color,
        weight: 3,
        opacity: 0.85,
      }).addTo(map);
      layers.push(poly);
      bounds.push(...latlngs);
    }

    flatPoints.sort((a, b) => a.t - b.t);

    if (bounds.length) {
      map.fitBounds(bounds, { padding: [24, 24] });
    } else {
      map.setView([20, 0], 2);
    }

    if (flatPoints.length) {
      const t0 = flatPoints[0].t.getTime();
      const t1 = flatPoints[flatPoints.length - 1].t.getTime();
      tl.max = Math.max(1, flatPoints.length - 1);
      tl.value = 0;
      tlRange.textContent = `${flatPoints.length} 点 · ${maxLen ? maxLen + " 点/序列" : ""}`;
      updateMarker(t0, t1, 0);
    } else {
      tl.max = 1;
      tl.value = 0;
      tlRange.textContent = "无点";
    }
  }

  let marker = null;
  function updateMarker(t0, t1, idx) {
    if (!flatPoints.length) return;
    const p = flatPoints[Math.min(idx, flatPoints.length - 1)];
    const frac = (p.t.getTime() - t0) / Math.max(1, t1 - t0);
    if (marker) map.removeLayer(marker);
    marker = L.circleMarker([p.lat, p.lon], {
      radius: 9,
      color: "#0c0f14",
      weight: 2,
      fillColor: p.color,
      fillOpacity: 0.95,
    }).addTo(map);
    marker.bindPopup(
      `<div style="font-size:12px;">${p.label}<br/>${p.t.toLocaleString()}</div>`,
    );
    marker.openPopup();
    tlRange.textContent = `${Math.round(frac * 100)}% · ${p.t.toLocaleString()}`;
  }

  tl.addEventListener("input", () => {
    if (!flatPoints.length) return;
    const t0 = flatPoints[0].t.getTime();
    const t1 = flatPoints[flatPoints.length - 1].t.getTime();
    const i = Number(tl.value);
    updateMarker(t0, t1, i);
  });

  document.getElementById("reset").addEventListener("click", () => {
    if (playTimer) {
      clearInterval(playTimer);
      playTimer = null;
    }
    tl.value = 0;
    tl.dispatchEvent(new Event("input"));
  });

  document.getElementById("play").addEventListener("click", () => {
    if (playTimer) {
      clearInterval(playTimer);
      playTimer = null;
      return;
    }
    let i = Number(tl.value);
    playTimer = setInterval(() => {
      i += 1;
      if (i >= tl.max) i = 0;
      tl.value = String(i);
      tl.dispatchEvent(new Event("input"));
    }, 120);
  });

  /** 默认：不传 from/to，后端返回全库时间范围 + 降采样轨迹 */
  fetchJSON("/api/journey")
    .then(renderJourney)
    .catch((e) => {
      document.getElementById("subtitle").textContent = String(e.message || e);
    });

  fetchJSON("/api/stats")
    .then((s) => {
      const el = document.getElementById("stats");
      el.innerHTML = "";
      for (const r of s.rows || []) {
        const card = document.createElement("div");
        card.className = "card";
        card.innerHTML = `<small>${r.display}</small><strong>${r.point_count} 点</strong><small style="display:block;margin-top:0.2rem;">≈ ${r.distance_km} km · 最近 ${r.last_seen ? new Date(r.last_seen).toLocaleString() : "-"}</small>`;
        el.appendChild(card);
      }
    })
    .catch(() => {});
}

main();
