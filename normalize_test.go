package main

import "testing"

func TestNormalizeQuery(t *testing.T) {
	tests := []struct {
		name string
		input string
		want string
	}{
		{"нижний регистр", "ПЛАТЬЕ", "платье"},
		{"пробелы по краям", "  платье  ", "платье"},
		{"повторные пробелы внутри", "красное  платье", "красное платье"},
		{"все вместе", "  КРАСНОЕ  ПЛАТЬЕ  ", "красное платье"},
		{"уже нормализовано", "платье", "платье"},
		{"пустая строка", "", ""},
		{"только пробелы", "  ", ""},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeQuery(tc.input)
			if got != tc.want {
				t.Errorf("normalizeQuery(%q) = %q, хотим %q", tc.input, got, tc.want)
			}
		})
	}
}
