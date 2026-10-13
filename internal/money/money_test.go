package money

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want Satang
		err  error
	}{
		{"500", 50000, nil},
		{"1,250", 125000, nil},
		{"1,250,000", 125000000, nil},
		{"1250.5", 125050, nil},
		{"1250.50", 125050, nil},
		{"0.75", 75, nil},
		{"๑๒๐", 12000, nil},
		{"99 บาท", 9900, nil},
		{"99บาท", 9900, nil},
		{"฿99", 9900, nil},
		{"  42  ", 4200, nil},

		{"", 0, ErrEmpty},
		{"บาท", 0, ErrEmpty},
		{"0", 0, ErrNotPositive},
		{"0.00", 0, ErrNotPositive},
		{"12,50", 0, ErrInvalid},
		{",500", 0, ErrInvalid},
		{"1,2500", 0, ErrInvalid},
		{"1.234", 0, ErrInvalid},
		{"1.", 0, ErrInvalid},
		{".5", 0, ErrInvalid},
		{"abc", 0, ErrInvalid},
		{"-5", 0, ErrInvalid},
		{"100000001", 0, ErrTooLarge},
		{"0812345678901", 0, ErrTooLarge},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if !errors.Is(err, tt.err) {
			t.Errorf("Parse(%q) error = %v, want %v", tt.in, err, tt.err)
			continue
		}
		if got != tt.want {
			t.Errorf("Parse(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		in   Satang
		want string
	}{
		{0, "0"},
		{5, "0.05"},
		{50000, "500"},
		{125050, "1,250.50"},
		{100000000, "1,000,000"},
		{-12000, "-120"},
		{-125050, "-1,250.50"},
	}
	for _, tt := range tests {
		if got := tt.in.String(); got != tt.want {
			t.Errorf("Satang(%d).String() = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSigned(t *testing.T) {
	for in, want := range map[Satang]string{430000: "+4,300", 0: "0", -12000: "-120"} {
		if got := in.Signed(); got != want {
			t.Errorf("Satang(%d).Signed() = %q, want %q", in, got, want)
		}
	}
}
