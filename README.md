# market-tags
 
Учебный проект: строим поисковые теги (топ-10 следующих запросов) для маркетплейса по логам поиска в ClickHouse и отдаем из через Go-сервис на простую веб-страницу.

## Стек

- **ClickHouse** - хранение логов и агрегатов.
- **GO 1.22+** (стандартная библиотека + `clickhouse-go/v2`) - HTTP-сервис
- **html/template** - рендер страницы.
- **Docker** - запуск ClickHouse локально.

## Структура
.
├── main.go                    # HTTP-сервис (API + HTML)
├── normalize_test.go          # unit-тест нормализации q
├── handler_test.go            # тест эндпоинта /api/tags (пустой q → 400)
├── templates/index.html       # HTML-шаблон (вшит через //go:embed)
├── sql/
│   ├── 01_create_search_logs.sql   # CREATE TABLE search_logs
│   ├── 02_insert_search_logs.sql   # INSERT из parquet с нормализацией
│   ├── 03_create_search_tags.sql   # CREATE TABLE search_tags
│   ├── 04_build_search_tags.sql    # INSERT — построение топ-10 тегов
│   └── 05_checks.sql               # SQL-проверки из задания
├── go.mod
├── go.sum
└── README.md

## Как запустить

### 1. ClickHouse в Docker

```bash
docker run -d \
  --name clickhouse-server \
  --ulimit nofile=262144:262144 \
  -p 8123:8123 \
  -p 9000:9000 \
  -v clickhouse_data:/var/lib/clickhouse \
  -e CLICKHOUSE_PASSWORD=goservice \
  clickhouse/clickhouse-server
```

### 2. Загрузить датасет в контейнер

```bash
docker cp dataset.parquet clickhouse-server:/var/lib/clickhouse/user_files/dataset.parquet
```

### 3. Прогнать SQL-скрипты (по порядку)

```bash
docker exec -i clickhouse-server clickhouse-client --password goservice < sql/01_create_search_logs.sql
docker exec -i clickhouse-server clickhouse-client --password goservice < sql/02_insert_search_logs.sql
docker exec -i clickhouse-server clickhouse-client --password goservice < sql/03_create_search_tags.sql
docker exec -i clickhouse-server clickhouse-client --password goservice < sql/04_build_search_tags.sql
docker exec -i clickhouse-server clickhouse-client --password goservice < sql/05_checks.sql
```

Шаг 5 должен вывести `0`, `0`, `10`, `0  0` — это значит, что все SQL-проверки пройдены.

### 4. Запустить Go-сервис

```bash
go run main.go
```

По умолчанию:
- ClickHouse: `clickhouse://default:goservice@localhost:9000`
- HTTP-сервер: `:8080`

Оба можно переопределить через переменные окружения:

```bash
CLICKHOUSE_DSN="clickhouse://default:goservice@localhost:9000" \
HTTP_ADDR=":9090" \
go run main.go
```

Открыть в браузере: <http://localhost:8080/>.

### 5. Тесты

```bash
go test -v ./...
```

Должны пройти `TestNormalizeQuery` (7 кейсов) и `TestTagsHandler_EmptyQuery` (3 кейса).

## API

### `GET /api/tags?q=<текст>`

Возвращает список тегов для нормализованного `q` в порядке `rank`.

**Успех (200):**

```json
[{"tag": "красное платье", "users": 9264}, {"tag": "вечернее платье", "users": 6609}]
```

**Пустой `q` (400):**

```json
{"error": "empty query"}
```

**Нет тегов (200):**

```json
[]
```

### `GET /`

HTML-страница: поле поиска, кнопка «Найти», список тегов под строкой. Теги кликабельны — клик подставляет тег в поле поиска и показывает теги для него.

## Решения по схеме и почему

### `search_logs`

```sql
ENGINE = MergeTree
ORDER BY (user_id, timestamp)
```

- **`MergeTree`** — стандартный движок для логов: append-only, хорошо сжимает, поддерживает индексы по `ORDER BY`.
- **`ORDER BY (user_id, timestamp)`** — весь алгоритм построения тегов читает «все запросы одного пользователя по порядку времени». При такой сортировке эти данные физически лежат рядом — чтение последовательное, без случайных прыжков по диску. Альтернатива `(timestamp, user_id)` была бы хуже: у нас нет запросов «все действия за период», у нас есть «все действия одного пользователя».
- **Без `PARTITION BY`** — для ~1 млн строк одна партиция оптимальна. В продакшене сделали бы `PARTITION BY toYYYYMM(timestamp)`, чтобы удобно удалять старые данные.

### `search_tags`

```sql
ENGINE = MergeTree
ORDER BY (query, rank)
```

Go-сервис читает её так: `SELECT tag, users FROM search_tags WHERE query = ? ORDER BY rank`. При такой сортировке все теги для одного `query` лежат рядом и уже отсортированы по `rank` — чтение 10 строк происходит мгновенно.

### Нормализация

Применяется **в двух местах** — по одному и тому же правилу (нижний регистр, обрезка пробелов по краям, схлопывание повторных пробелов):

1. **При вставке в `search_logs`** (SQL): `replaceRegexpAll(trimBoth(lowerUTF8(search_query)), '\\s+', ' ')`. Дальше вся аналитика работает с чистыми данными — проще отлаживать запросы.

2. **В Go, при каждом запросе** (`normalizeQuery` в `main.go`): входной параметр `q` приводится к тому же виду, прежде чем уйти в `search_tags`. Без этого запрос «  ПЛАТЬЕ  » не нашёл бы ничего, потому что в `search_tags` всё хранится как «платье». На это есть unit-тест `TestNormalizeQuery` (7 кейсов, включая «Платье » и «платье»).

## Допущения

- **Пароль `goservice` в коде.** Используется как дефолт для локальной разработки, чтобы `go run main.go` работал из коробки. В продакшене задаётся через `CLICKHOUSE_DSN`. В реальном проекте пароль в исходниках недопустим.
- **`leadInFrame` с явной рамкой.** Используем `leadInFrame` (а не `lead`) с `ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING`, потому что по умолчанию рамка окна — «до текущей строки включительно», и `leadInFrame` вернул бы NULL для всех строк.
- **`QUALIFY` вместо внешнего подзапроса.** В ClickHouse нельзя фильтровать по оконной функции в `WHERE` (ошибка `ILLEGAL_AGGREGATION`). Используем `QUALIFY rank <= 10` — ClickHouse-специфичное расширение, аналог `WHERE` после оконных функций.
- **`row_number` вместо `LIMIT 10 BY query`.** Задание предлагает оба. Выбрали `row_number`, потому что результат требует колонку `rank` (1–10). `LIMIT BY` не даёт номер строки.

## Проверки

- `sql/05_checks.sql` — 4 SQL-проверки из задания.
- `go test -v ./...` — 2 Go-теста (нормализация + пустой `q` → 400).

## Бонус: инкрементальный пересчёт

Если бы нужно было пересчитывать теги каждый день на новых данных, не пересобирая всё с нуля:

1. **Хранить пары с разбивкой по дням.** Таблица `pairs_daily(day, query, tag, users)` — агрегат за каждый день. Заполняется из `search_logs` за вчерашний день.
2. **Итоговая `search_tags`** пересобирается из `pairs_daily` за последние N дней. При этом `users` считать как `uniq` по пользователям за окно — но `uniq` не аддитивен (нельзя сложить два дня), поэтому хранить либо `uniqExact` с состоянием (агрегатная функция `uniqState`/`uniqMerge` в ClickHouse), либо сырые пары `(day, user_id, query, tag)` и пересчитывать `uniq` по ним за окно.
3. **Старые дни** (старше окна) можно удалять из `pairs_daily`, чтобы таблица не росла бесконечно.

В ClickHouse для этого есть `AggregatingMergeTree` + `uniqState`/`uniqMerge` — специально под такие инкрементальные агрегаты.
