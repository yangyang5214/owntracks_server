-- OwnTracks + ClickHouse
-- 约定: user/device 对应逻辑 topic owntracks/<user>/<device>（HTTP 路径或 topic 参数）；时间统一 UTC。

CREATE DATABASE IF NOT EXISTS owntracks;

-- ---------------------------------------------------------------------------
-- 1) 位置历史（主表）
-- 引擎: MergeTree — 追加型时序，按用户/设备/时间范围扫描最优。
-- 分区: 按月 — 分区数适中，便于按时间裁剪。
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
PARTITION BY toYYYYMM(event_time)
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
PARTITION BY toYYYYMM(event_time)
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
PARTITION BY toYYYYMM(event_time)
ORDER BY (user, device, event_time, message_type)
SETTINGS index_granularity = 8192;
