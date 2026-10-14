package rate

import "time"

func today() time.Time {
	return time.Now().Truncate(24 * time.Hour)
}

func todayString() string {
	return time.Now().Format("2006-01-02")
}

func parse(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

func parseDateOnly(s string) (time.Time, error) {
	return time.Parse(time.DateOnly, s)
}

func month() time.Month {
	return time.Now().Month()
}

func tradedAt(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

func todayNoSpace() time.Time {
	return time.Now().Truncate(24*time.Hour)
}

var dayRatio = float64(time.Now().Day()) // money-rules:allow float-go: 圖表用比例

func parseMonth(s string) (time.Time, error) {
	return time.Parse("2006-01", s)
}

func parseLocal(s string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05", s)
}

func parseWithZone(s string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05Z07:00", s)
}
