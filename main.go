package main

import (
	"context"
	"database/sql"
	_ "embed"//для директивы go:embed; сам пакет по имени не используется
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	
	_ "github.com/ClickHouse/clickhouse-go/v2"//драйвер регистрирует себя в database/sql под именем "clickhouse"; напрямую не используется
)

//при компиляции вшивает содержимое файла в переменную ниже
//go:embed templates/index.html
var indexHTML string

// Tag - структура одной строки ответа
type Tag struct {
	Tag string `json:"tag"`
	Users uint64 `json:"users"`
}

// PageData - структура для передачи в шаблон
type PageData struct {
	Query string
	Tags []Tag
}

// Server - держит зависимости, которые нужны HTTP-обработчикам
type Server struct {
	db *sql.DB	//пул соединений с ClickHouse
	tmpl *template.Template	//скомпилированный HTML-шаблон
}

func main() {
	// Читаем DSN из окружения
	dsn := os.Getenv("CLICKHOUSE_DSN")
	if dsn == "" {
		dsn = "clickhouse://default:goservice@localhost:9000"
		log.Println("CLICKHOUSE_DSN не задан, использую дефолт для локальной разработки")
	}
	
	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":8080"
	}
	
	// Открываем пул соединений с ClickHouse
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		log.Fatalf("Ошибка открытия соединения: %v", err)
	}
	defer db.Close()// Закроет пул при выходе из main


	// Проверяем, что соединение реально работает (с таймаутом).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Ошибка подключения к ClickHouse: %v", err)
	}

	// Парсим HTML-шаблон один раз при старте; Must паникует, если шаблон сломан	
	tmpl := template.Must(template.New("index").Parse(indexHTML))
	
	// HTTP-сервер: регистрируем обработчики и запускаем прослушивание порта
	srv := &Server{db: db, tmpl: tmpl}
	http.HandleFunc("/", srv.indexHandler)	// главная страница
	http.HandleFunc("/api/tags", srv.tagsHandler)	//JSON API; более специфичный путь перекрывает "/"
	
	log.Printf("Сервер запущен на http://localhost%s", httpAddr)
	// ListenAndServe блокирующий: не возвращается, пока сервер работает
	if err := http.ListenAndServe(httpAddr, nil); err != nil {
		log.Fatalf("Сервер упал: %v", err)
	}
}

// ftechTags делает параметризованный запрос к search_tags и возвращает срез тегов
// Контекст позволяет прервать запрос по таймауту или при отмене HTTP-запроса
func (s *Server) fetchTags(ctx context.Context, q string) ([]Tag, error) {
	rows ,err := s.db.QueryContext(ctx,
		"SELECT tag, users FROM search_tags WHERE query = ? ORDER BY rank",	// "?" - плейсхолдер, защита от SQL-инъекций
		q)
	if err != nil {
		return nil, err
	}
	defer rows.Close() // обязательно: пока rows не закрыт, соединение занято

	//Чтение результата
	tags := []Tag{}// пустой срез (не nil), чтобы в JSON ушло [], а не null
	for rows.Next() { // Переход к следующей строке
		var t Tag
		if err := rows.Scan(&t.Tag, &t.Users); err != nil { // Чтение поля     текущей строки в выбранные переменные
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()// rows.Err() - ошибка итерации (Next вернет false и при конце и при ошибке)
}

// indexHandler отдает HTML-страницу: форма поиска + список тегов для q (если q задан)
func (s *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	q := normalizeQuery(r.URL.Query().Get("q"))

	// Query заполняем всегда (для <input value="...">), Tags - только если q непустой	
	data := PageData{Query: q}
	
	if q != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)//таймаут на запрос к БД
		defer cancel()
		
		tags, err := s.fetchTags(ctx, q)
		if err != nil {
			log.Printf("Ошибка запроса к ClickHouse: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		data.Tags = tags
	}
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8") //Явные обозначения браузеру
	if err := s.tmpl.Execute(w, data); err != nil {// рендер шаблона с данными в ответ
		log.Printf("Ошибка рендера шаблона: %v", err)
	}
}

// tagsHandler - JSON API: GET /api/tags&q=.... -> список {tag, users} в порядке rank
func (s *Server) tagsHandler(w http.ResponseWriter, r *http.Request) {
	// Проверка пустого q
	q := normalizeQuery(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty query"})
		return
	}
	
	// Таймаут для запроса
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	tags, err := s.fetchTags(ctx, q)
	if err != nil {
		log.Printf("Ошибка запроса к ClickHouse: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, tags)
}
// Приводит q к виду в котором он кранится ы search_tags:
// нижний регистр, без пробелов по краям, повторные пробелы схлопываются в один
func normalizeQuery(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	fields := strings.Fields(s)// склеивает по любой последовательности пробельных символов
	return strings.Join(fields, " ")// склеивает через один пробел
}

// Хелпер - ставит Content-Type, статус и сериализует v в JSON в тело ответа
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)	// важно вызвать до записи тела, иначе Go отправит 200
	json.NewEncoder(w).Encode(v)	// сериализует в JSON и пишет в w (с переводом строки
}
