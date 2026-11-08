-- Результирующая таблица с тегами (топ-10 следующих запросов для каждого query).
-- ORDER BY (query, rank) выбран потому, что Go-сервис читает её так:
--   SELECT tag, users FROM search_tags WHERE query = ? ORDER BY rank
-- При такой сортировке все теги для одного query лежат рядом и уже упорядочены
-- по rank — чтение 10 строк происходит мгновенно.

CREATE TABLE IF NOT EXISTS search_tags
(
    query String,
    tag   String,
    users UInt64,
    rank  UInt8
)
ENGINE = MergeTree
ORDER BY (query, rank);
