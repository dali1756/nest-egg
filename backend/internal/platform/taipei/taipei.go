package taipei

import (
	"time"
	_ "time/tzdata"
)

var Location = mustLoad()

func mustLoad() *time.Location {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		panic(err)
	}
	return loc
}

func Today(now time.Time) time.Time {
	y, m, d := now.In(Location).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, Location)
}

func ParseDate(s string) (time.Time, error) {
	return time.ParseInLocation(time.DateOnly, s, Location)
}

func FromDBDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, Location)
}
