// Package money handles Thai baht amounts.
//
// Amounts are stored as integer satang (1 baht = 100 satang) so that sums are
// always exact. Never use float64 for money.
package money

import (
	"errors"
	"strconv"
	"strings"
)

// Satang is an amount of Thai baht expressed in satang (1/100 baht).
type Satang int64

// Max is the largest single amount we accept (100 million baht).
// It guards against typos such as pasting a phone number as an amount.
const Max Satang = 100_000_000 * 100

var (
	ErrEmpty       = errors.New("money: empty amount")
	ErrInvalid     = errors.New("money: invalid amount")
	ErrTooLarge    = errors.New("money: amount too large")
	ErrNotPositive = errors.New("money: amount must be greater than zero")
)

var thaiDigits = strings.NewReplacer(
	"๐", "0", "๑", "1", "๒", "2", "๓", "3", "๔", "4",
	"๕", "5", "๖", "6", "๗", "7", "๘", "8", "๙", "9",
)

// NormalizeDigits converts Thai numerals (๐-๙) to ASCII digits.
func NormalizeDigits(s string) string {
	return thaiDigits.Replace(s)
}

// Parse reads a positive baht amount such as "500", "1,250", "1250.5",
// "๑๒๕๐" or "99.75 บาท" and returns it in satang.
func Parse(s string) (Satang, error) {
	s = NormalizeDigits(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, "บาท")
	s = strings.TrimPrefix(s, "฿")
	s = strings.TrimSuffix(s, "฿")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ErrEmpty
	}

	whole, frac, hasFrac := strings.Cut(s, ".")
	if !validGrouping(whole) {
		return 0, ErrInvalid
	}
	whole = strings.ReplaceAll(whole, ",", "")
	if hasFrac && (len(frac) == 0 || len(frac) > 2 || !allDigits(frac)) {
		return 0, ErrInvalid
	}

	// Limit length before converting so huge inputs cannot overflow int64.
	if len(whole) > 12 {
		return 0, ErrTooLarge
	}
	baht, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, ErrInvalid
	}

	var satang int64
	if hasFrac {
		if len(frac) == 1 {
			frac += "0"
		}
		satang, _ = strconv.ParseInt(frac, 10, 64)
	}

	total := Satang(baht*100 + satang)
	switch {
	case total <= 0:
		return 0, ErrNotPositive
	case total > Max:
		return 0, ErrTooLarge
	}
	return total, nil
}

// validGrouping accepts "1250" or a correctly comma-grouped "1,250,000".
func validGrouping(s string) bool {
	if s == "" {
		return false
	}
	if !strings.Contains(s, ",") {
		return allDigits(s)
	}
	parts := strings.Split(s, ",")
	if len(parts[0]) == 0 || len(parts[0]) > 3 || !allDigits(parts[0]) {
		return false
	}
	for _, p := range parts[1:] {
		if len(p) != 3 || !allDigits(p) {
			return false
		}
	}
	return true
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// String formats the amount like "1,250" or "1,250.50" (no currency unit).
func (m Satang) String() string {
	neg := m < 0
	if neg {
		m = -m
	}
	baht := int64(m) / 100
	satang := int64(m) % 100

	s := groupThousands(strconv.FormatInt(baht, 10))
	if satang != 0 {
		s += "." + twoDigits(satang)
	}
	if neg {
		s = "-" + s
	}
	return s
}

// Signed formats the amount with an explicit sign, e.g. "+4,300" or "-120".
func (m Satang) Signed() string {
	if m > 0 {
		return "+" + m.String()
	}
	return m.String()
}

func groupThousands(digits string) string {
	if len(digits) <= 3 {
		return digits
	}
	var b strings.Builder
	head := len(digits) % 3
	if head > 0 {
		b.WriteString(digits[:head])
	}
	for i := head; i < len(digits); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}

func twoDigits(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}
