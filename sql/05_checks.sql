-- Проверки корректности search_tags (Часть 4 задания).
-- Ожидаемые результаты указаны в комментариях.

-- 1) Нет ни одной строки, где query = tag
--    Ожидается: 0
SELECT countIf(query = tag) AS same_query_tag
FROM search_tags;

-- 2) Нет строк, где users < 5
--    Ожидается: 0
SELECT countIf(users < 5) AS low_users
FROM search_tags;

-- 3) Для каждого query не больше 10 тегов
--    Ожидается: 10 (или меньше, если у какого-то query мало пар)
SELECT max(cnt) AS max_tags_per_query
FROM (
    SELECT query, count() AS cnt
    FROM search_tags
    GROUP BY query
);

-- 4) Все query и tag уже в нормализованном виде
--    (нижний регистр, без пробелов по краям)
--    Ожидается: 0 и 0
SELECT
    countIf(query != lowerUTF8(trimBoth(query))) AS non_normalized_query,
    countIf(tag   != lowerUTF8(trimBoth(tag)))   AS non_normalized_tag
FROM search_tags;
