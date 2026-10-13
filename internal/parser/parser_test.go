package parser

import (
	"reflect"
	"testing"

	"github.com/jodbanchee/jodbanchee/internal/ledger"
	"github.com/jodbanchee/jodbanchee/internal/money"
)

func baht(n int64) money.Satang { return money.Satang(n * 100) }

func in(amount money.Satang, note string) ledger.NewEntry {
	return ledger.NewEntry{Kind: ledger.Income, Amount: amount, Note: note}
}

func out(amount money.Satang, note string) ledger.NewEntry {
	return ledger.NewEntry{Kind: ledger.Expense, Amount: amount, Note: note}
}

func TestParseEntries(t *testing.T) {
	tests := []struct {
		text string
		want []ledger.NewEntry
	}{
		// amount first
		{"+500 ขายผัก", []ledger.NewEntry{in(baht(500), "ขายผัก")}},
		{"-120 ค่ากับข้าว", []ledger.NewEntry{out(baht(120), "ค่ากับข้าว")}},
		{"-120ค่ากับข้าว", []ledger.NewEntry{out(baht(120), "ค่ากับข้าว")}},
		{"+ 500", []ledger.NewEntry{in(baht(500), "")}},
		{"+1,250.50 บาท ค่าขนม", []ledger.NewEntry{in(125050, "ค่าขนม")}},
		{"รับ 500 ขายผัก", []ledger.NewEntry{in(baht(500), "ขายผัก")}},
		{"รับ500", []ledger.NewEntry{in(baht(500), "")}},
		{"รายรับ 500บาท เงินเดือน", []ledger.NewEntry{in(baht(500), "เงินเดือน")}},
		{"จ่าย 80 ค่าน้ำ", []ledger.NewEntry{out(baht(80), "ค่าน้ำ")}},
		{"รายจ่าย ๑๒๐ ค่ารถ", []ledger.NewEntry{out(baht(120), "ค่ารถ")}},
		{"−120 ค่ากับข้าว", []ledger.NewEntry{out(baht(120), "ค่ากับข้าว")}}, // U+2212 minus
		{"＋500 ขายผัก", []ledger.NewEntry{in(baht(500), "ขายผัก")}},          // full-width plus

		// keyword, note, amount
		{"จ่ายค่าไฟ 1,200", []ledger.NewEntry{out(baht(1200), "ค่าไฟ")}},
		{"จ่ายค่าไฟ 1,200 บาท", []ledger.NewEntry{out(baht(1200), "ค่าไฟ")}},
		{"รับเงินค่าผัก 500", []ledger.NewEntry{in(baht(500), "เงินค่าผัก")}},
		{"จ่าย ค่าเทอมลูก 15000", []ledger.NewEntry{out(baht(15000), "ค่าเทอมลูก")}},

		// note, sign, amount
		{"ขายผัก +500", []ledger.NewEntry{in(baht(500), "ขายผัก")}},
		{"ค่ากับข้าว -120", []ledger.NewEntry{out(baht(120), "ค่ากับข้าว")}},
		{"ค่ากับข้าว-120บาท", []ledger.NewEntry{out(baht(120), "ค่ากับข้าว")}},

		// several lines
		{"-120 ค่ากับข้าว\n-50 ค่ารถ\n\n+300 ขายไข่", []ledger.NewEntry{
			out(baht(120), "ค่ากับข้าว"), out(baht(50), "ค่ารถ"), in(baht(300), "ขายไข่"),
		}},
		{"-120 ค่ากับข้าว\r\n-50 ค่ารถ", []ledger.NewEntry{out(baht(120), "ค่ากับข้าว"), out(baht(50), "ค่ารถ")}},
	}
	for _, tt := range tests {
		got := Parse(tt.text)
		if got.Action != Record {
			t.Errorf("Parse(%q).Action = %v, want Record", tt.text, got.Action)
			continue
		}
		if !reflect.DeepEqual(got.Entries, tt.want) {
			t.Errorf("Parse(%q).Entries = %+v, want %+v", tt.text, got.Entries, tt.want)
		}
	}
}

func TestParseIgnoresConversation(t *testing.T) {
	for _, text := range []string{
		"",
		"   ",
		"สวัสดีจ้า",
		"กินข้าวยัง",
		"วันนี้ซื้อผักไป 120", // no keyword or sign
		"500",            // bare number is ambiguous
		"รับลูก 5 โมง",   // keyword, but not an entry
		"- ไปตลาดก่อนนะ", // dash without amount
		"+0 ขายผัก",      // zero amount
		"+12,50 ขายผัก",  // bad grouping
		"-120 ค่ากับข้าว\nไปแล้วนะ", // one chatty line spoils the batch
		"สรุปยอดให้หน่อย",           // not an exact command
	} {
		if got := Parse(text); got.Action != None {
			t.Errorf("Parse(%q) = %+v, want None", text, got)
		}
	}
}

func TestParseTooManyLines(t *testing.T) {
	text := ""
	for i := 0; i <= MaxLines; i++ {
		text += "-1 x\n"
	}
	if got := Parse(text); got.Action != None {
		t.Errorf("Parse(%d lines).Action = %v, want None", MaxLines+1, got.Action)
	}
}

func TestParseCommands(t *testing.T) {
	tests := []struct {
		text string
		want Command
	}{
		{"วิธีใช้", Command{Action: Help}},
		{"HELP", Command{Action: Help}},
		{"ลบล่าสุด", Command{Action: Undo}},
		{" undo ", Command{Action: Undo}},
		{"สรุป", Command{Action: Summary, Period: ledger.ThisMonth}},
		{"สรุปวันนี้", Command{Action: Summary, Period: ledger.Today}},
		{"สรุป วันนี้", Command{Action: Summary, Period: ledger.Today}},
		{"สรุปเมื่อวาน", Command{Action: Summary, Period: ledger.Yesterday}},
		{"สรุปเดือนที่แล้ว", Command{Action: Summary, Period: ledger.LastMonth}},
		{"/summary today", Command{Action: Summary, Period: ledger.Today}},
	}
	for _, tt := range tests {
		if got := Parse(tt.text); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Parse(%q) = %+v, want %+v", tt.text, got, tt.want)
		}
	}
}
