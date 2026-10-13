# จดบัญชี (jodbanchee)

บอท LINE สำหรับจดรายรับรายจ่ายในไลน์กลุ่ม ดึงเข้ากลุ่มครอบครัวแล้วพิมพ์ `+500 ขายผัก` หรือ `-120 ค่ากับข้าว` บอทจะบันทึกให้ และสรุปยอดได้ทุกเมื่อ

*A self-hosted LINE bot that keeps an income/expense ledger for a group chat. Written in Go with zero third-party dependencies. Replies are in Thai. [English summary below](#english).*

```
แม่:  -120 ค่ากับข้าว
บอท: ✅ รายจ่าย 120 บาท · ค่ากับข้าว

     เดือนนี้: รับ 12,500 · จ่าย 8,320 · คงเหลือ +4,180

แม่:  สรุป
บอท: 📊 สรุปเดือนตุลาคม 2569
     รายรับ 12,500 บาท
     รายจ่าย 8,320 บาท
     คงเหลือ +4,180 บาท
     (23 รายการ)

     จ่ายมากสุด
     1. ค่ากับข้าว 3,200 บาท
     2. ค่าไฟ 1,200 บาท
     3. ค่ารถ 640 บาท
```

## จุดเด่น

- **ใช้ฟรี** บอทตอบด้วย reply message เท่านั้น ซึ่งไม่นับโควตาข้อความของ LINE OA
- **ข้อมูลอยู่กับเรา** self-host เอง ไม่มีข้อมูลการเงินไปอยู่ในระบบของคนอื่น
- **ไม่กวนแชท** ข้อความคุยกันปกติบอทจะเงียบ ตอบเฉพาะรายการและคำสั่ง
- **พิมพ์ได้หลายแบบ** มีหรือไม่มีเว้นวรรคก็ได้ ใช้เลขไทยได้ หลายรายการในข้อความเดียวได้
- **แยกบัญชีตามแชท** ดึงเข้ากี่กลุ่มก็ได้ แต่ละกลุ่มมีบัญชีของตัวเอง
- **ติดตั้งง่าย** ไบนารีเดียว ไม่ต้องมีฐานข้อมูล (เก็บเป็นไฟล์ JSON)

## วิธีใช้ในแชท

| พิมพ์ | ผลลัพธ์ |
|---|---|
| `+500 ขายผัก` · `รับ 500 ขายผัก` · `ขายผัก +500` | บันทึกรายรับ |
| `-120 ค่ากับข้าว` · `จ่ายค่าไฟ 1,200` · `ค่ารถ -40` | บันทึกรายจ่าย |
| หลายบรรทัดในข้อความเดียว | บันทึกทุกบรรทัด (ทุกบรรทัดต้องเป็นรายการ) |
| `สรุป` / `สรุปเดือนนี้` | สรุปเดือนนี้ |
| `สรุปวันนี้` · `สรุปเมื่อวาน` · `สรุปเดือนที่แล้ว` | สรุปช่วงอื่น |
| `ลบล่าสุด` | ลบรายการล่าสุดที่ *ตัวเอง* บันทึก |
| `วิธีใช้` | ดูวิธีใช้ |

จำนวนเงินใส่ลูกน้ำ ทศนิยม 2 ตำแหน่ง เลขไทย หรือคำว่า "บาท" ต่อท้ายได้ ส่วน `500` เฉยๆ ที่ไม่มี `+`/`-`/รับ/จ่าย บอทจะไม่บันทึก เพราะกำกวมเกินไป

## ลองเล่นในเครื่องก่อน (ไม่ต้องมีบัญชี LINE)

ต้องมี Go 1.24 ขึ้นไป

```bash
make chat        # หรือ go run ./cmd/devchat
```

พิมพ์ข้อความแล้วกด Enter เหมือนคุยในกลุ่ม ลงท้ายบรรทัดด้วย `\` เพื่อพิมพ์หลายบรรทัด และพิมพ์ `/as พ่อ` เพื่อเปลี่ยนคนส่ง

## ติดตั้งใช้จริง

### 1. สร้างบอทใน LINE

1. สร้าง LINE Official Account (ฟรี) ที่ [LINE Official Account Manager](https://manager.line.biz/)
2. ใน OA Manager → **Settings → Messaging API** กดเปิดใช้งาน แล้วเลือก Provider
3. ใน OA Manager → **Settings → Account settings** เปิด **"Allow account to join groups and multi-person chats"**
4. ใน OA Manager → **Settings → Response settings** ปิด *Auto-response* และ *Greeting message*, เปิด *Webhooks*
5. ไปที่ [LINE Developers Console](https://developers.line.biz/console/) → channel ของบอท
   - แท็บ **Basic settings** → คัดลอก **Channel secret**
   - แท็บ **Messaging API** → กด Issue **Channel access token (long-lived)** แล้วคัดลอก

### 2. รันเซิร์ฟเวอร์

```bash
cp .env.example .env    # ใส่ LINE_CHANNEL_SECRET และ LINE_CHANNEL_ACCESS_TOKEN
docker compose up -d    # หรือ make run ถ้าไม่ใช้ Docker
```

ข้อมูลเก็บที่ `DATA_FILE` (ค่าเริ่มต้น `data/ledger.json`, ใน Docker อยู่ใน volume `ledger-data`)

### 3. ต่อ webhook

LINE ต้องเรียกเซิร์ฟเวอร์ผ่าน HTTPS
- **ทดสอบในเครื่อง:** `cloudflared tunnel --url http://localhost:8080` หรือ `ngrok http 8080`
- **ใช้จริง:** VM ฟรี เช่น Oracle Cloud Always Free หรือ Google Cloud e2-micro + Docker

นำ URL ไปใส่ที่ LINE Developers Console → Messaging API → **Webhook URL** เป็น `https://<โดเมน>/webhook` เปิด **Use webhook** แล้วกด **Verify**

> ⚠️ บอทเก็บข้อมูลเป็นไฟล์ จึงต้องรันบนเครื่องที่มีดิสก์ถาวร แพลตฟอร์มแบบ stateless (เช่น Cloud Run, Lambda) จะทำข้อมูลหาย ถ้าจะใช้แพลตฟอร์มแบบนั้นให้รอ storage แบบ Postgres (ดู Roadmap)

### 4. ดึงเข้ากลุ่ม

เพิ่มบอทเป็นเพื่อน แล้วเชิญเข้ากลุ่มเหมือนเชิญคนทั่วไป บอทจะทักทายพร้อมบอกวิธีใช้

## ค่าตั้ง

| ตัวแปร | จำเป็น | ค่าเริ่มต้น | คำอธิบาย |
|---|---|---|---|
| `LINE_CHANNEL_SECRET` | ✓ | | ใช้ตรวจว่า request มาจาก LINE จริง |
| `LINE_CHANNEL_ACCESS_TOKEN` | ✓ | | ใช้ส่งข้อความตอบกลับ |
| `PORT` | | `8080` | |
| `DATA_FILE` | | `data/ledger.json` | ที่เก็บข้อมูล |
| `TIMEZONE` | | `Asia/Bangkok` | กำหนดว่า "วันนี้" และ "เดือนนี้" เริ่มเมื่อไร |

## ความเป็นส่วนตัวและความปลอดภัย

- ทุก request ถูกตรวจลายเซ็น `X-Line-Signature` ก่อนประมวลผล
- log ไม่เก็บเนื้อหาข้อความ
- ไฟล์ข้อมูลสร้างด้วยสิทธิ์ `0600` (อ่านได้เฉพาะเจ้าของ)
- **อย่า commit** `.env` หรือโฟลเดอร์ `data/` (ใส่ไว้ใน `.gitignore` แล้ว)
- สำรองไฟล์ `ledger.json` เป็นประจำ

## โครงสร้างโปรเจกต์

```
cmd/jodbanchee/        เซิร์ฟเวอร์ webhook
cmd/devchat/           คุยกับบอทใน terminal
internal/parser/       แปลงข้อความแชท → คำสั่ง
internal/ledger/       business logic: บันทึก สรุป ลบ
internal/money/        จำนวนเงินเป็นสตางค์ (int64) ไม่ใช้ float
internal/store/        ที่เก็บข้อมูล (ตอนนี้มี jsonfile)
internal/line/         LINE API: ตรวจลายเซ็น, webhook types, reply
internal/bot/          เชื่อมทุกอย่าง + ข้อความตอบกลับภาษาไทย
```

อยากเปลี่ยนที่เก็บข้อมูล? implement interface `ledger.Store` (3 เมธอด) แล้วเปลี่ยนใน `cmd/jodbanchee/main.go`

## Roadmap

- [ ] Storage แบบ PostgreSQL (สำหรับ Cloud Run / แพลตฟอร์ม stateless)
- [ ] แสดงชื่อคนบันทึกในสรุป
- [ ] Quick Reply เลือกหมวดหมู่หลังบันทึก
- [ ] สรุปแบบ Flex Message
- [ ] Export เป็น CSV / Google Sheets
- [ ] หน้ากราฟรายเดือนผ่าน LIFF

## ร่วมพัฒนา

ยินดีรับ issue และ pull request ก่อนส่ง PR รัน

```bash
make lint test
```

ถ้าเพิ่มรูปแบบข้อความใหม่ใน parser ช่วยเพิ่มทั้งเคสที่ *ต้อง* บันทึก และเคสบทสนทนาปกติที่ *ต้องไม่* บันทึกใน `parser_test.go` ด้วย

## License

[MIT](LICENSE)

---

## English

**jodbanchee** ("จดบัญชี", *keep the books*) is a LINE Messaging API bot you invite into a group chat to track income and expenses. Family members type entries such as `+500 sold vegetables` or `-120 groceries` (in Thai: `+500 ขายผัก`, `-120 ค่ากับข้าว`), and the bot confirms each one and reports daily or monthly totals on request.

- Go standard library only, no third-party dependencies.
- Uses reply messages only, which are free on every LINE Official Account plan.
- Ignores ordinary conversation; only clearly formatted entries and commands get a reply.
- Money is stored as integer satang; periods follow the configured time zone (default `Asia/Bangkok`).
- Storage is a single JSON file behind the `ledger.Store` interface, so other backends can be added.

Try it without LINE: `go run ./cmd/devchat`. Deploy: see the Thai setup steps above, or `docker compose up -d` with a filled-in `.env`.
