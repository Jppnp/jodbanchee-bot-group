// Command devchat lets you talk to the bot in your terminal, without LINE.
//
//	go run ./cmd/devchat
//
// Each line you type is sent as a group message; end a line with \ to write a
// multi-line message. Type "/as <name>" to switch
// sender (to try ลบล่าสุด with several people) and "/quit" to exit.
// Data goes to a temporary file unless you pass -data path/to/ledger.json.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/jodbanchee/jodbanchee/internal/bot"
	"github.com/jodbanchee/jodbanchee/internal/ledger"
	"github.com/jodbanchee/jodbanchee/internal/line"
	"github.com/jodbanchee/jodbanchee/internal/store/jsonfile"
)

func main() {
	dataFile := flag.String("data", "", "ledger file (default: a temp file)")
	tz := flag.String("tz", "Asia/Bangkok", "time zone")
	flag.Parse()

	if err := run(*dataFile, *tz, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(dataFile, tz string, in io.Reader, out io.Writer) error {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return err
	}
	if dataFile == "" {
		dir, err := os.MkdirTemp("", "jodbanchee-devchat-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		dataFile = filepath.Join(dir, "ledger.json")
	}
	store, err := jsonfile.Open(dataFile)
	if err != nil {
		return err
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	b := bot.New(ledger.NewService(store, loc), log)
	ctx := context.Background()
	user := "mom"

	say := func(msgs []line.Message) {
		for _, m := range msgs {
			fmt.Fprintf(out, "🤖 %s\n\n", strings.ReplaceAll(m.Text, "\n", "\n   "))
		}
	}
	say(b.Respond(ctx, line.Event{Type: line.EventJoin, Source: groupSource(user)}))

	sc := bufio.NewScanner(in)
	var pending []string // lines of the message being typed
	flush := func() {
		if len(pending) == 0 {
			return
		}
		text := strings.Join(pending, "\n")
		pending = nil
		msgs := b.Respond(ctx, line.Event{
			Type:    line.EventMessage,
			Source:  groupSource(user),
			Message: &line.EventMessageBody{Type: "text", Text: text},
		})
		if len(msgs) == 0 {
			fmt.Fprint(out, "   (บอทไม่ตอบ — ถือเป็นข้อความคุยกันปกติ)\n\n")
		}
		say(msgs)
	}

	fmt.Fprintf(out, "Enter = ส่ง · ลงท้ายบรรทัดด้วย \\ เพื่อพิมพ์หลายบรรทัด · /as <ชื่อ> เปลี่ยนผู้ส่ง · /quit ออก\n[%s] ", user)
	for sc.Scan() {
		input := strings.TrimSpace(sc.Text())
		switch {
		case input == "/quit":
			return nil
		case strings.HasPrefix(input, "/as "):
			user = strings.TrimSpace(strings.TrimPrefix(input, "/as "))
			fmt.Fprintf(out, "ตอนนี้ส่งในนาม %q\n", user)
		case strings.HasSuffix(input, `\`):
			pending = append(pending, strings.TrimSuffix(input, `\`))
			fmt.Fprint(out, "  ... ")
			continue // keep collecting this message
		default:
			pending = append(pending, input)
			flush()
		}
		fmt.Fprintf(out, "[%s] ", user)
	}
	flush()
	return sc.Err()
}

func groupSource(user string) line.Source {
	return line.Source{Type: line.SourceGroup, GroupID: "Cdevchatgroup", UserID: "U" + user}
}
