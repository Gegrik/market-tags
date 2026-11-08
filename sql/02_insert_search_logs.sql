-- Заливка логов из parquet с нормализацией search_query по правилам задания:
--   1) lowerUTF8 — нижний регистр (учитывает кириллицу)
--   2) trimBoth  — обрезка пробелов по краям
--   3) replaceRegexpAll(..., '\\s+', ' ') — повторные пробелы схлопываются в один
-- WHERE-условия — страховка от NULL и пустых строк.
--
-- ПЕРЕД ЗАПУСКОМ: файл dataset.parquet должен лежать в /var/lib/clickhouse/user_files/
-- (см. README — раздел "Как запустить").

INSERT INTO search_logs
SELECT
    timestamp,
    user_id,
    replaceRegexpAll(trimBoth(lowerUTF8(search_query)), '\\s+', ' ') AS search_query
FROM file('dataset.parquet', Parquet)
WHERE search_query IS NOT NULL
  AND trimBoth(search_query) != '';
