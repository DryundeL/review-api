package tz

import (
	"strings"
	"time"
)

// Moscow — часовой пояс приложения (Europe/Moscow).
// Если в образе нет zoneinfo, используется фиксированный MSK (UTC+3).
var Moscow *time.Location

func init() {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		Moscow = time.FixedZone("MSK", 3*3600)
		return
	}
	Moscow = loc
}

// Now возвращает текущий момент с локацией Moscow (удобно для «офисных» меток и периодов).
func Now() time.Time {
	return time.Now().In(Moscow)
}

// FormatRFC3339 — RFC3339 со смещением Москвы.
func FormatRFC3339(t time.Time) string {
	return t.In(Moscow).Format(time.RFC3339)
}

// FormatDateTime — «2006-01-02 15:04:05» по Москве.
func FormatDateTime(t time.Time) string {
	return t.In(Moscow).Format("2006-01-02 15:04:05")
}

// FormatDate — дата для имён файлов и отображения только даты.
func FormatDate(t time.Time) string {
	return t.In(Moscow).Format("2006-01-02")
}

// FormatWithLayout форматирует время в заданном layout по часам Москвы.
func FormatWithLayout(t time.Time, layout string) string {
	return t.In(Moscow).Format(layout)
}

// ParseDateStart — начало календарного дня в Москве (00:00:00 по MSK).
func ParseDateStart(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", strings.TrimSpace(s), Moscow)
}

// ParseDateEndInclusive — конец календарного дня в Москве для условий вида created_at <= t.
func ParseDateEndInclusive(s string) (time.Time, error) {
	start, err := ParseDateStart(s)
	if err != nil {
		return time.Time{}, err
	}
	return start.Add(24*time.Hour - time.Nanosecond), nil
}
