package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTagsHandler_EmptyQuery(t *testing.T) {
	tests := []struct {
		name string
		url string
	}{
		{"q отсутствует", "/api/tags"},
		{"q пустой", "/api/tags?q="},
		{"q из одних пробелов", "/api/tags?q=%20%20%20"}, //%20 необходим так как мы напрямую задаем URL без curl, который автоматически все перекодирует
	}
	
	srv := &Server{db: nil} //db не нужен, до БД не дойдем
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil) //Фейковый *http.Request с аргументами: метод(GET), URL(задаем в структуре tests, тело(nil). Возвращает запрос для передачи в tagsHandler
			rec := httptest.NewRecorder() //Фейковый http.ResponseWriter, записывает все от handler в память
			
			srv.tagsHandler(rec, req) //Вызываем метод на прямую, как функцию
			
			if rec.Code != http.StatusBadRequest { //итоговый статус-код
				t.Errorf("статус = %d, хотим %d. Тело: %s",
					rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"error"`){ //тело ответа как строка
				t.Errorf("в теле нет поля error. Тело: %s", rec.Body.String())
			}
		})
	}
}
