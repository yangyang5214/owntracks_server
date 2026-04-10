-- 已有库：为 locations / latest_locations 增加区县 adcode，并重建 latest物化视图。
-- 若可清空数据，更简单做法：fresh_clickhouse.sql + init.sql（见 resource/fresh_clickhouse.sql）。
-- 执行前请确认库名（默认 owntracks）。

DROP VIEW IF EXISTS owntracks.locations_to_latest_mv;

ALTER TABLE owntracks.locations
    ADD COLUMN IF NOT EXISTS `district_adcode` LowCardinality(Nullable(String))
    COMMENT '区县 adcode（入库时高德逆地理）' AFTER `charging`;

ALTER TABLE owntracks.latest_locations
    ADD COLUMN IF NOT EXISTS `district_adcode` LowCardinality(Nullable(String)) AFTER `battery`;

CREATE MATERIALIZED VIEW owntracks.locations_to_latest_mv
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
    district_adcode,
    toUInt64(toUnixTimestamp64Milli(event_time)) AS ver,
    payload_json
FROM owntracks.locations;
