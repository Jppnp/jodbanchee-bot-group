package ledger

import "time"

// Period is a named time window for summaries.
type Period string

const (
	Today     Period = "today"
	Yesterday Period = "yesterday"
	ThisMonth Period = "this_month"
	LastMonth Period = "last_month"
)

// Range returns [from, to) for the period relative to now.
// now's location decides where days and months begin.
func (p Period) Range(now time.Time) (from, to time.Time) {
	loc := now.Location()
	y, m, d := now.Date()
	startOfDay := time.Date(y, m, d, 0, 0, 0, 0, loc)
	startOfMonth := time.Date(y, m, 1, 0, 0, 0, 0, loc)

	switch p {
	case Today:
		return startOfDay, startOfDay.AddDate(0, 0, 1)
	case Yesterday:
		return startOfDay.AddDate(0, 0, -1), startOfDay
	case LastMonth:
		return startOfMonth.AddDate(0, -1, 0), startOfMonth
	default: // ThisMonth
		return startOfMonth, startOfMonth.AddDate(0, 1, 0)
	}
}
