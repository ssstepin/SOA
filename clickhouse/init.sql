CREATE DATABASE IF NOT EXISTS stats;
  USE stats;

  CREATE TABLE IF NOT EXISTS events (
    event_date Date,
    event_time DateTime,
    event_type Enum8('like' = 1, 'view' = 2, 'comment' = 3),
    user_id UInt32,
    post_id UInt32,
    comment_text Nullable(String),
    INDEX post_idx post_id TYPE bloom_filter GRANULARITY 3
  ) ENGINE = MergeTree()
  PARTITION BY toYYYYMM(event_date)
  ORDER BY (post_id, event_date, event_type)
  TTL event_date + INTERVAL 6 MONTH;

  CREATE MATERIALIZED VIEW IF NOT EXISTS events_daily_mv
  ENGINE = SummingMergeTree()
  PARTITION BY toYYYYMM(event_date)
  ORDER BY (post_id, event_date, event_type)
  AS SELECT
    event_date,
    post_id,
    event_type,
    count() as count
  FROM events
  GROUP BY event_date, post_id, event_type;
