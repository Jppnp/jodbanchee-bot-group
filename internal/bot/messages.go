package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/jodbanchee/jodbanchee/internal/ledger"
)

// All user-facing Thai text lives in this file so it is easy to review,
// reword, or translate.

const helpText = `📒 วิธีใช้บอทจดบัญชี

บันทึกรายรับ
+500 ขายผัก
รับ 500 ขายผัก

บันทึกรายจ่าย
-120 ค่ากับข้าว
จ่ายค่าไฟ 1,200

พิมพ์หลายบรรทัดในข้อความเดียวได้ (ทุกบรรทัดต้องเป็นรายการ)

ดูยอด
สรุป · สรุปวันนี้ · สรุปเมื่อวาน · สรุปเดือนที่แล้ว

แก้ไข
ลบล่าสุด — ลบรายการล่าสุดที่คุณบันทึกเอง

พิมพ์ "วิธีใช้" เพื่อดูข้อความนี้อีกครั้ง`

const welcomeText = `สวัสดีครับ 🙏 ผมจะช่วยจดรายรับรายจ่ายให้แชทนี้
ข้อความคุยกันปกติผมจะไม่ตอบ ตอบเฉพาะข้อความที่เป็นรายการหรือคำสั่งเท่านั้น`

const (
	errSaveText    = "⚠️ บันทึกไม่สำเร็จ ลองส่งใหม่อีกครั้งนะครับ"
	errReadText    = "⚠️ ดึงข้อมูลไม่สำเร็จ ลองใหม่อีกครั้งนะครับ"
	noUserText     = "ไม่ทราบว่าใครเป็นคนส่ง เลยลบให้ไม่ได้ครับ"
	nothingToUndo  = "ยังไม่มีรายการที่คุณบันทึกในแชทนี้ครับ"
	emptyPeriodFmt = "📊 สรุป%s\nยังไม่มีรายการครับ"
)

var (
	thaiMonths = [...]string{"มกราคม", "กุมภาพันธ์", "มีนาคม", "เมษายน", "พฤษภาคม", "มิถุนายน",
		"กรกฎาคม", "สิงหาคม", "กันยายน", "ตุลาคม", "พฤศจิกายน", "ธันวาคม"}
	thaiMonthsShort = [...]string{"ม.ค.", "ก.พ.", "มี.ค.", "เม.ย.", "พ.ค.", "มิ.ย.",
		"ก.ค.", "ส.ค.", "ก.ย.", "ต.ค.", "พ.ย.", "ธ.ค."}
)

// buddhistYear converts a Gregorian year to the Thai (B.E.) year.
func buddhistYear(t time.Time) int { return t.Year() + 543 }

func shortDate(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), thaiMonthsShort[t.Month()-1], buddhistYear(t))
}

func periodLabel(p ledger.Period, from time.Time) string {
	switch p {
	case ledger.Today:
		return "วันนี้ (" + shortDate(from) + ")"
	case ledger.Yesterday:
		return "เมื่อวาน (" + shortDate(from) + ")"
	default:
		return fmt.Sprintf("เดือน%s %d", thaiMonths[from.Month()-1], buddhistYear(from))
	}
}

func kindLabel(k ledger.Kind) string {
	if k == ledger.Income {
		return "รายรับ"
	}
	return "รายจ่าย"
}

// entryLine renders "รายจ่าย 120 บาท · ค่ากับข้าว".
func entryLine(e ledger.Entry) string {
	s := fmt.Sprintf("%s %s บาท", kindLabel(e.Kind), e.Amount)
	if e.Note != "" {
		s += " · " + e.Note
	}
	return s
}

func recordedText(saved []ledger.Entry, month ledger.Summary) string {
	var b strings.Builder
	if len(saved) == 1 {
		b.WriteString("✅ " + entryLine(saved[0]))
	} else {
		fmt.Fprintf(&b, "✅ บันทึก %d รายการ", len(saved))
		for _, e := range saved {
			b.WriteString("\n• " + entryLine(e))
		}
	}
	fmt.Fprintf(&b, "\n\nเดือนนี้: รับ %s · จ่าย %s · คงเหลือ %s",
		month.Income, month.Expense, month.Net().Signed())
	return b.String()
}

func summaryText(s ledger.Summary) string {
	label := periodLabel(s.Period, s.From)
	if s.Count == 0 {
		return fmt.Sprintf(emptyPeriodFmt, label)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "📊 สรุป%s\n", label)
	fmt.Fprintf(&b, "รายรับ %s บาท\n", s.Income)
	fmt.Fprintf(&b, "รายจ่าย %s บาท\n", s.Expense)
	fmt.Fprintf(&b, "คงเหลือ %s บาท\n", s.Net().Signed())
	fmt.Fprintf(&b, "(%d รายการ)", s.Count)

	if len(s.TopExpenses) > 0 {
		b.WriteString("\n\nจ่ายมากสุด")
		for i, t := range s.TopExpenses {
			note := t.Note
			if note == "" {
				note = "(ไม่ระบุ)"
			}
			fmt.Fprintf(&b, "\n%d. %s %s บาท", i+1, note, t.Amount)
		}
	}
	return b.String()
}

func undoneText(e ledger.Entry, loc *time.Location) string {
	at := e.CreatedAt.In(loc)
	return fmt.Sprintf("🗑️ ลบแล้ว: %s\n(บันทึกเมื่อ %d %s %02d:%02d)",
		entryLine(e), at.Day(), thaiMonthsShort[at.Month()-1], at.Hour(), at.Minute())
}
