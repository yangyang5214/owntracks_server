-- 丢弃 owntracks 库及全部数据；随后请执行同目录 init.sql 重建表结构。
-- 例：clickhouse-client ... --multiquery < fresh_clickhouse.sql && clickhouse-client ... --multiquery < init.sql

DROP DATABASE IF EXISTS owntracks;
