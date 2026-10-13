// Package parser turns chat text into bot commands.
//
// It is deliberately strict: in a family group most messages are normal
// conversation, so anything that does not clearly look like a command is
// ignored (Action None) instead of guessed.
package parser

import (
	"regexp"
	"strings"

	"github.com/Jppnp/jodbanchee-bot-group/internal/ledger"
	"github.com/Jppnp/jodbanchee-bot-group/internal/money"
)

// Action is what the bot should do with a message.
type Action int

const (
	None    Action = iota // not for the bot; stay quiet
	Record                // save Entries
	Summary               // report totals for Period
	Undo                  // delete the sender's latest entry
	Help                  // show usage
)

// Command is the parsed result of one message.
type Command struct {
	Action  Action
	Entries []ledger.NewEntry // for Record
	Period  ledger.Period     // for Summary
}

// MaxLines is the most entries accepted in one multi-line message.
const MaxLines = 50

// An entry line can be written three ways. Thai is often typed without
// spaces, so spaces between parts are optional where that is unambiguous.
//
//	amountFirst:  +500 ขายผัก   -120ค่ากับข้าว   รับ 500 ขายผัก   จ่าย 1,250.50 บาท ค่าไฟ
//	keywordNote:  จ่ายค่าไฟ 1,200   รับเงินค่าผัก 500 บาท
//	noteSign:     ขายผัก +500   ค่ากับข้าว -120
//
// Longer keywords come first so "รายรับ" is not read as "ราย" + "รับ".
const (
	amountPat  = `([0-9][0-9,]*(?:\.[0-9]+)?)`
	keywordPat = `(\+|-|รายรับ|รายจ่าย|รับ|จ่าย)`
	bahtPat    = `\s*(?:บาท|฿)?`
)

var (
	amountFirstRe = regexp.MustCompile(`^` + keywordPat + `\s*` + amountPat + bahtPat + `\s*(.*)$`)
	keywordNoteRe = regexp.MustCompile(`^(รายรับ|รายจ่าย|รับ|จ่าย)\s*([^0-9+\-\s].*?)\s+` + amountPat + bahtPat + `$`)
	noteSignRe    = regexp.MustCompile(`^([^0-9+\-].*?)\s*([+-])\s*` + amountPat + bahtPat + `$`)
)

var (
	helpWords = set("วิธีใช้", "ช่วยเหลือ", "help", "/help", "/start")
	undoWords = set("ลบล่าสุด", "ยกเลิกล่าสุด", "ลบอันล่าสุด", "undo", "/undo")
	summaries = map[string]ledger.Period{
		"สรุป":             ledger.ThisMonth,
		"สรุปเดือนนี้":     ledger.ThisMonth,
		"สรุปวันนี้":       ledger.Today,
		"สรุปเมื่อวาน":     ledger.Yesterday,
		"สรุปเดือนที่แล้ว": ledger.LastMonth,
		"สรุปเดือนก่อน":    ledger.LastMonth,
		"/summary":           ledger.ThisMonth,
		"/summary today":     ledger.Today,
		"/summary yesterday": ledger.Yesterday,
		"/summary last":      ledger.LastMonth,
	}
)

// Parse reads one chat message.
func Parse(text string) Command {
	text = normalize(text)
	if text == "" {
		return Command{}
	}

	key := strings.ToLower(text)
	if helpWords[key] {
		return Command{Action: Help}
	}
	if undoWords[key] {
		return Command{Action: Undo}
	}
	// Accept both "สรุปวันนี้" and "สรุป วันนี้".
	if p, ok := summaries[key]; ok {
		return Command{Action: Summary, Period: p}
	}
	if p, ok := summaries[strings.ReplaceAll(key, " ", "")]; ok && strings.HasPrefix(key, "สรุป") {
		return Command{Action: Summary, Period: p}
	}

	return parseEntries(text)
}

// parseEntries accepts a message only if every non-empty line is an entry.
// One stray line of chat means the whole message is ignored.
func parseEntries(text string) Command {
	var entries []ledger.NewEntry
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		e, ok := parseLine(line)
		if !ok {
			return Command{}
		}
		entries = append(entries, e)
		if len(entries) > MaxLines {
			return Command{}
		}
	}
	if len(entries) == 0 {
		return Command{}
	}
	return Command{Action: Record, Entries: entries}
}

func parseLine(line string) (ledger.NewEntry, bool) {
	var keyword, amountText, note string
	if m := amountFirstRe.FindStringSubmatch(line); m != nil {
		keyword, amountText, note = m[1], m[2], m[3]
	} else if m := keywordNoteRe.FindStringSubmatch(line); m != nil {
		keyword, note, amountText = m[1], m[2], m[3]
	} else if m := noteSignRe.FindStringSubmatch(line); m != nil {
		note, keyword, amountText = m[1], m[2], m[3]
	} else {
		return ledger.NewEntry{}, false
	}

	amount, err := money.Parse(amountText)
	if err != nil {
		return ledger.NewEntry{}, false
	}
	kind := ledger.Expense
	if keyword == "+" || keyword == "รับ" || keyword == "รายรับ" {
		kind = ledger.Income
	}
	return ledger.NewEntry{Kind: kind, Amount: amount, Note: strings.TrimSpace(note)}, true
}

// normalize tidies up text typed on phones: Thai digits, look-alike plus and
// minus signs, non-breaking spaces and Windows line endings.
func normalize(s string) string {
	s = money.NormalizeDigits(s)
	s = strings.NewReplacer(
		"\r\n", "\n",
		" ", " ", // non-breaking space
		"−", "-", // minus sign
		"–", "-", // en dash
		"—", "-", // em dash
		"＋", "+", // full-width plus
		"－", "-", // full-width minus
	).Replace(s)
	return strings.TrimSpace(s)
}

func set(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}
