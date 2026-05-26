// Package datetypes hosts wire-format primitives that don't have a clean home
// inside any one domain module. The first inhabitant is Date — a calendar
// date (no time, no zone) backed by time.Time, used for SQL DATE columns
// (check_in_date, check_out_date, ...) whose default time.Time JSON
// representation as a full RFC3339 timestamp confuses the frontend.
package datetypes

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

const layout = "2006-01-02"

// Date is a calendar date with no time-of-day. JSON wire form is "YYYY-MM-DD";
// Postgres DATE columns scan/round-trip correctly through pgx.
type Date time.Time

func (d Date) Time() time.Time { return time.Time(d) }

func (d Date) Format(l string) string { return time.Time(d).Format(l) }

func (d Date) IsZero() bool { return time.Time(d).IsZero() }

func (d Date) String() string { return time.Time(d).Format(layout) }

func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte(`null`), nil
	}
	return []byte(`"` + d.Format(layout) + `"`), nil
}

func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*d = Date{}
		return nil
	}
	// Tolerate full RFC3339 from older callers — trim to the date portion.
	if len(s) > 10 {
		s = s[:10]
	}
	t, err := time.Parse(layout, s)
	if err != nil {
		return fmt.Errorf("invalid date %q: %w", s, err)
	}
	*d = Date(t)
	return nil
}

// Scan implements sql.Scanner so pgx can read a Postgres DATE column. pgx
// hands us a time.Time at UTC midnight.
func (d *Date) Scan(src any) error {
	if src == nil {
		*d = Date{}
		return nil
	}
	switch v := src.(type) {
	case time.Time:
		*d = Date(v)
		return nil
	case string:
		t, err := time.Parse(layout, v)
		if err != nil {
			return fmt.Errorf("date scan %q: %w", v, err)
		}
		*d = Date(t)
		return nil
	case []byte:
		t, err := time.Parse(layout, string(v))
		if err != nil {
			return fmt.Errorf("date scan %q: %w", v, err)
		}
		*d = Date(t)
		return nil
	default:
		return fmt.Errorf("date scan: unsupported type %T", src)
	}
}

// Value lets pgx serialize a Date back into a DATE column.
func (d Date) Value() (driver.Value, error) {
	if d.IsZero() {
		return nil, nil
	}
	return d.Time(), nil
}
