-- OwnTracks + ClickHouse
-- 约定: user/device 对应逻辑 topic owntracks/<user>/<device>（HTTP 路径或 topic 参数）；时间统一 UTC。

CREATE DATABASE IF NOT EXISTS owntracks;

-- 注意：若库表已存在且仍为按月分区，ClickHouse 无法原地改分区键；需建新表迁数据或保持原表。

-- ---------------------------------------------------------------------------
-- 1) 位置历史（主表）
-- 引擎: MergeTree — 追加型时序，按用户/设备/时间范围扫描最优。
-- 分区: 按年 — 分区数少，适合多年长期保留；单年数据量极大时可再改为按月。
-- 排序: (user, device, event_time, ingest_seq) — 同一秒内多点用 ingest_seq 保序。
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS owntracks.locations
(
    `user` LowCardinality(String) COMMENT 'topic 中的用户名',
    `device` LowCardinality(String) COMMENT 'topic 中的设备名',
    `topic` String COMMENT '完整逻辑 topic（owntracks/user/device）',
    `event_time` DateTime64(3, 'UTC') COMMENT 'OwnTracks tst（设备事件时间）',
    `ingested_at` DateTime64(3, 'UTC') DEFAULT now64(3) COMMENT '入库时间',
    `ingest_seq` UInt16 DEFAULT 0 COMMENT '同一 event_time 下区分多点',

    `lat` Float64 COMMENT '纬度 WGS84',
    `lon` Float64 COMMENT '经度 WGS84',
    `acc` Nullable(Float32) COMMENT '水平精度 m',
    `alt` Nullable(Int32) COMMENT '海拔 m',
    `vac` Nullable(UInt16) COMMENT '垂直精度 m',
    `vel` Nullable(Float32) COMMENT '速度 m/s',
    `cog` Nullable(Float32) COMMENT '航向 °',
    `dist` Nullable(Float32) COMMENT '相对上次距离 m',
    `tid` LowCardinality(Nullable(String)) COMMENT 'tracker id',
    `t` LowCardinality(Nullable(String)) COMMENT '附加类型等',
    `trigger` LowCardinality(Nullable(String)) COMMENT '触发: p/u/c/r/v 等',

    `battery` Nullable(Int16) COMMENT '电量 %',
    `charging` Nullable(UInt8) COMMENT '是否充电 0/1',

    `payload_json` String COMMENT '完整 JSON，便于扩展字段' CODEC(ZSTD(3))
)
ENGINE = MergeTree
PARTITION BY toYear(event_time)
ORDER BY (user, device, event_time, ingest_seq)
SETTINGS index_granularity = 8192;

-- 未设置 TTL：默认长期保留全部数据；若需按时间清理再自行加 TTL。

-- ---------------------------------------------------------------------------
-- 2) 每设备「最新一条」位置（从 locations 同步）
-- 引擎: ReplacingMergeTree(ver) — 同 (user,device) 合并时保留 ver 最大行，适合「最新状态」。
-- 查询未合并数据时需 FINAL 或聚合；高 QPS 读最新可配合物化视图或应用层缓存。
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS owntracks.latest_locations
(
    `user` LowCardinality(String),
    `device` LowCardinality(String),
    `topic` String,
    `event_time` DateTime64(3, 'UTC'),
    `ingested_at` DateTime64(3, 'UTC'),

    `lat` Float64,
    `lon` Float64,
    `acc` Nullable(Float32),
    `alt` Nullable(Int32),
    `vel` Nullable(Float32),
    `cog` Nullable(Float32),
    `tid` LowCardinality(Nullable(String)),
    `trigger` LowCardinality(Nullable(String)),
    `battery` Nullable(Int16),

    `ver` UInt64 COMMENT '单调版本，用于 Replacing；通常取毫秒时间戳',
    `payload_json` String CODEC(ZSTD(3))
)
ENGINE = ReplacingMergeTree(ver)
PARTITION BY toYear(event_time)
ORDER BY (user, device)
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS owntracks.locations_to_latest_mv
TO owntracks.latest_locations
AS
SELECT
    user,
    device,
    topic,
    event_time,
    ingested_at,
    lat,
    lon,
    acc,
    alt,
    vel,
    cog,
    tid,
    trigger,
    battery,
    toUInt64(toUnixTimestamp64Milli(event_time)) AS ver,
    payload_json
FROM owntracks.locations;

-- ---------------------------------------------------------------------------
-- 3) 非 location 消息（可选，如 waypoint / card / configuration）
-- 引擎: MergeTree — 与 locations 分离，避免稀疏列；JSON 全量存 payload。
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS owntracks.events
(
    `user` LowCardinality(String),
    `device` LowCardinality(String),
    `topic` String,
    `message_type` LowCardinality(String) COMMENT '_type 字段，如 waypoint',
    `event_time` DateTime64(3, 'UTC'),
    `ingested_at` DateTime64(3, 'UTC') DEFAULT now64(3),

    `payload_json` String CODEC(ZSTD(3))
)
ENGINE = MergeTree
PARTITION BY toYear(event_time)
ORDER BY (user, device, event_time, message_type)
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------------
-- 4) 热力图预聚合网格（物化视图增量维护，读图走本表避免全表扫 locations）
-- 说明：
--   - 新写入 locations 时 MV 自动按日 + 用户 + 设备 + 网格累加，非「每天批跑」；
--   - 若希望合并分区可定期 OPTIMIZE TABLE owntracks.heatmap_grid FINAL；
--   - 上线前历史数据须执行 resource/heatmap_backfill.sql（仅一次，勿重复执行以免重复计数）。
-- 网格：g_lat/g_lon = round(lat/lon * 10000)，赤道附近约 11m。
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS owntracks.heatmap_grid
(
    `day` Date COMMENT 'UTC 日历日',
    `user` LowCardinality(String),
    `device` LowCardinality(String),
    `g_lat` Int32 COMMENT 'round(lat * 10000)',
    `g_lon` Int32 COMMENT 'round(lon * 10000)',
    `cnt` UInt64 COMMENT '落入该格子的点数',
    `sum_lat` Float64 COMMENT '纬度之和，用于加权中心',
    `sum_lon` Float64 COMMENT '经度之和'
)
ENGINE = SummingMergeTree()
PARTITION BY toYear(day)
ORDER BY (day, user, device, g_lat, g_lon)
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS owntracks.locations_to_heatmap_mv
TO owntracks.heatmap_grid
AS
SELECT
    toDate(event_time) AS day,
    user,
    device,
    toInt32(round(lat * 10000)) AS g_lat,
    toInt32(round(lon * 10000)) AS g_lon,
    toUInt64(1) AS cnt,
    lat AS sum_lat,
    lon AS sum_lon
FROM owntracks.locations;
