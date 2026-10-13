# จดบัญชี (jodbanchee)

[![CI](https://github.com/Jppnp/jodbanchee-bot-group/actions/workflows/ci.yml/badge.svg)](https://github.com/Jppnp/jodbanchee-bot-group/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

บอท LINE สำหรับจดรายรับรายจ่ายในไลน์กลุ่ม ดึงเข้ากลุ่มครอบครัวแล้วพิมพ์ `+500 ขายผัก` หรือ `-120 ค่ากับข้าว` บอทจะบันทึกให้ และสรุปยอดได้ทุกเมื่อ

*A self-hosted LINE bot that keeps an income/expense ledger for a group chat. Written in Go. Replies are in Thai. [English summary below](#english).*

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
- **เลือกที่เก็บข้อมูลได้** ไฟล์ JSON (ไบนารีเดียว ไม่ต้องมีฐานข้อมูล) หรือ PostgreSQL (สำหรับ Cloud Run และแพลตฟอร์ม serverless)

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
git clone https://github.com/Jppnp/jodbanchee-bot-group.git
cd jodbanchee-bot-group
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

ถ้าไม่ได้ตั้ง `DATABASE_URL` ข้อมูลจะเก็บที่ `DATA_FILE` (ค่าเริ่มต้น `data/ledger.json`, ใน Docker อยู่ใน volume `ledger-data`)

### 3. ต่อ webhook

LINE ต้องเรียกเซิร์ฟเวอร์ผ่าน HTTPS
- **ทดสอบในเครื่อง:** `cloudflared tunnel --url http://localhost:8080` หรือ `ngrok http 8080`
- **ใช้จริง (แนะนำ):** Cloud Run + Neon Postgres ดู [คู่มือ deploy](docs/deploy-cloud-run.md)
- **ใช้จริงแบบ VM:** เช่น Oracle Cloud Always Free + Docker (ใช้ไฟล์ JSON ได้ ไม่ต้องมีฐานข้อมูล)

นำ URL ไปใส่ที่ LINE Developers Console → Messaging API → **Webhook URL** เป็น `https://<โดเมน>/webhook` เปิด **Use webhook** แล้วกด **Verify**

> ⚠️ แพลตฟอร์มที่ไม่มีดิสก์ถาวร (Cloud Run, Lambda ฯลฯ) ต้องตั้ง `DATABASE_URL` ไม่อย่างนั้นข้อมูลจะหายเมื่อ instance ถูกปิด

### 4. ดึงเข้ากลุ่ม

เพิ่มบอทเป็นเพื่อน แล้วเชิญเข้ากลุ่มเหมือนเชิญคนทั่วไป บอทจะทักทายพร้อมบอกวิธีใช้

## ค่าตั้ง

| ตัวแปร | จำเป็น | ค่าเริ่มต้น | คำอธิบาย |
|---|---|---|---|
| `LINE_CHANNEL_SECRET` | ✓ | | ใช้ตรวจว่า request มาจาก LINE จริง |
| `LINE_CHANNEL_ACCESS_TOKEN` | ✓ | | ใช้ส่งข้อความตอบกลับ |
| `PORT` | | `8080` | |
| `DATABASE_URL` | | | Postgres connection string ถ้าตั้งไว้จะใช้ Postgres แทนไฟล์ และสร้างตารางให้เองตอนเริ่ม |
| `DATA_FILE` | | `data/ledger.json` | ไฟล์เก็บข้อมูล (ใช้เมื่อไม่ได้ตั้ง `DATABASE_URL`) |
| `TIMEZONE` | | `Asia/Bangkok` | กำหนดว่า "วันนี้" และ "เดือนนี้" เริ่มเมื่อไร |

## ความเป็นส่วนตัวและความปลอดภัย

- ทุก request ถูกตรวจลายเซ็น `X-Line-Signature` ก่อนประมวลผล
- log ไม่เก็บเนื้อหาข้อความ
- ไฟล์ข้อมูลสร้างด้วยสิทธิ์ `0600` (อ่านได้เฉพาะเจ้าของ)
- **อย่า commit** `.env` หรือโฟลเดอร์ `data/` (ใส่ไว้ใน `.gitignore` แล้ว)
- สำรองข้อมูลเป็นประจำ (ไฟล์ `ledger.json` หรือ `pg_dump`)

## โครงสร้างโปรเจกต์

```
cmd/jodbanchee/        เซิร์ฟเวอร์ webhook
cmd/devchat/           คุยกับบอทใน terminal
internal/parser/       แปลงข้อความแชท → คำสั่ง
internal/ledger/       business logic: บันทึก สรุป ลบ
internal/money/        จำนวนเงินเป็นสตางค์ (int64) ไม่ใช้ float
internal/store/        ที่เก็บข้อมูล: jsonfile, postgres
internal/store/storetest/  ชุดทดสอบกลางที่ store ทุกตัวต้องผ่าน
internal/line/         LINE API: ตรวจลายเซ็น, webhook types, reply
internal/bot/          เชื่อมทุกอย่าง + ข้อความตอบกลับภาษาไทย
```

อยากเพิ่มที่เก็บข้อมูลแบบอื่น? implement interface `ledger.Store` (3 เมธอด) แล้วเรียก `storetest.Run` ในเทสต์ของมัน ถ้าผ่านก็ใช้แทนกันได้เลย

## Roadmap

- [x] Storage แบบ PostgreSQL (สำหรับ Cloud Run / แพลตฟอร์ม stateless)
- [ ] Deploy อัตโนมัติจาก GitHub Actions ไป Cloud Run
- [ ] แสดงชื่อคนบันทึกในสรุป
- [ ] Quick Reply เลือกหมวดหมู่หลังบันทึก
- [ ] สรุปแบบ Flex Message
- [ ] Export เป็น CSV / Google Sheets
- [ ] หน้ากราฟรายเดือนผ่าน LIFF

## ร่วมพัฒนา

ยินดีรับ issue และ pull request ก่อนส่ง PR รัน

```bash
make lint test
make db && make test-pg   # รวมเทสต์ของ Postgres store (ต้องมี Docker)
```

CI รันเทสต์ทั้งหมดกับ Postgres จริงทุกครั้งที่ push

ถ้าเพิ่มรูปแบบข้อความใหม่ใน parser ช่วยเพิ่มทั้งเคสที่ *ต้อง* บันทึก และเคสบทสนทนาปกติที่ *ต้องไม่* บันทึกใน `parser_test.go` ด้วย

## License

[MIT](LICENSE)

---

## English

**jodbanchee** ("จดบัญชี", *keep the books*) is a LINE Messaging API bot you invite into a group chat to track income and expenses. Family members type entries such as `+500 sold vegetables` or `-120 groceries` (in Thai: `+500 ขายผัก`, `-120 ค่ากับข้าว`), and the bot confirms each one and reports daily or monthly totals on request.

- Written in Go with one dependency: the pure-Go Postgres driver `lib/pq`, which itself has none.
- Uses reply messages only, which are free on every LINE Official Account plan.
- Ignores ordinary conversation; only clearly formatted entries and commands get a reply.
- Money is stored as integer satang; periods follow the configured time zone (default `Asia/Bangkok`).
- Storage is a JSON file for simple self-hosting, or PostgreSQL (`DATABASE_URL`) for Cloud Run and other stateless hosts. Both pass one shared contract test suite. See [docs/deploy-cloud-run.md](docs/deploy-cloud-run.md).

Try it without LINE: `go run ./cmd/devchat`. Deploy: see the Thai setup steps above, or `docker compose up -d` with a filled-in `.env`.
