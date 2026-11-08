-- Таблица для сырых логов поиска.
-- ORDER BY (user_id, timestamp) выбран потому, что основной алгоритм
-- построения тегов читает "все запросы одного пользователя по порядку времени" —
-- такая сортировка делает это чтение последовательным и быстрым.

CREATE TABLE IF NOT EXISTS search_logs
(
    timestamp    DateTime64(3, 'UTC'),
    user_id      UInt64,
    search_query String
)
ENGINE = MergeTree
ORDER BY (user_id, timestamp);
