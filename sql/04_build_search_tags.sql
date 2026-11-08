-- Построение тегов по правилам задания.
--
-- Логика:
--   1) pairs  — для каждой строки берём следующий запрос того же пользователя
--               через leadInFrame с явной рамкой (по умолчанию рамка "до текущей",
--               и leadInFrame вернул бы NULL — поэтому расширяем до UNBOUNDED FOLLOWING).
--   2) Фильтры:
--        - dateDiff <= 1800 секунд (30 минут)
--        - dateDiff >= 0 (отсекает NULL у последней строки пользователя)
--        - query != tag (правило 4)
--   3) agg    — группировка по (query, tag), считаем uniq(user_id) (правило 5),
--               HAVING >= 5 (правило 6).
--   4) Финальный SELECT — row_number по убыванию users внутри каждого query,
--      QUALIFY rank <= 10 (правило 7). QUALIFY — потому что в ClickHouse
--      нельзя фильтровать по оконной функции в WHERE.

INSERT INTO search_tags
WITH pairs AS (
    SELECT
        user_id,
        timestamp,
        search_query AS query,
        leadInFrame(timestamp)    OVER (PARTITION BY user_id ORDER BY timestamp
                                        ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING) AS next_ts,
        leadInFrame(search_query) OVER (PARTITION BY user_id ORDER BY timestamp
                                        ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING) AS tag
    FROM search_logs
),
agg AS (
    SELECT
        query,
        tag,
        uniq(user_id) AS users
    FROM pairs
    WHERE dateDiff('second', timestamp, next_ts) BETWEEN 0 AND 1800
      AND query != tag
    GROUP BY query, tag
    HAVING uniq(user_id) >= 5
)
SELECT
    query,
    tag,
    users,
    row_number() OVER (PARTITION BY query ORDER BY users DESC) AS rank
FROM agg
QUALIFY rank <= 10;
