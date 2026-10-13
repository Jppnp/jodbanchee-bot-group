# Deploy บน Google Cloud Run + Neon Postgres

คู่มือนี้พาบอทขึ้นไปรันบน Cloud Run (ปิดเองเมื่อไม่มีคนใช้ จ่ายตามการใช้งานจริง) และเก็บข้อมูลใน Neon (Postgres แบบ serverless) ใช้เวลาประมาณ 20–30 นาที

```
LINE ──webhook──▶ Cloud Run (asia-southeast1) ──▶ Neon Postgres (Singapore)
                        │
                        └── secrets จาก Secret Manager
```

> **ค่าใช้จ่าย:** บอทสำหรับครอบครัวใช้งานน้อยมาก ปกติจะอยู่ใน free tier ของทุกบริการ แต่ Google Cloud ต้องผูกบัตร (billing account) ถึงจะใช้ได้ **ตั้ง budget alert ไว้ด้วย (ขั้นที่ 7)** จะได้รู้ทันทีถ้ามีค่าใช้จ่ายเกิดขึ้น

## สิ่งที่ต้องมี

- บัญชี Google Cloud ที่เปิด billing แล้ว
- `gcloud` CLI: `brew install --cask google-cloud-sdk` แล้วรัน `gcloud auth login`
- บัญชี [Neon](https://neon.com)
- บอท LINE ที่ทดสอบในเครื่องผ่านแล้ว (มี Channel secret และ Channel access token)

## 1. สร้างฐานข้อมูลใน Neon

1. สร้าง Project ใหม่ เลือก region ที่ใกล้ประเทศไทยที่สุด (**AWS Asia Pacific (Singapore)** ถ้ามีให้เลือก)
2. กด **Connect** ที่หน้า Project
3. **ปิด "Connection pooling"** ก่อน copy เพราะบอทใช้ advisory lock ตอน migrate schema ซึ่งใช้กับ pooler แบบ transaction ไม่ได้ (URL ที่ถูกต้องจะ *ไม่มี* คำว่า `-pooler`)
4. Copy connection string เก็บไว้ หน้าตาประมาณ
   `postgresql://user:pass@ep-xxxx.ap-southeast-1.aws.neon.tech/neondb?sslmode=require&channel_binding=require`

   ใช้ string นี้ได้ตรงๆ เลย บอทจะตัด `channel_binding` ที่ไดรเวอร์ไม่รองรับออกให้เอง

ตารางในฐานข้อมูลบอทสร้างเองตอนเริ่มทำงานครั้งแรก ไม่ต้องรัน SQL อะไร

## 2. ตั้งค่า Google Cloud project

```bash
export PROJECT_ID=jodbanchee-xxxx      # ตั้งชื่อเองได้ ต้องไม่ซ้ำใคร
export REGION=asia-southeast1          # สิงคโปร์ ใกล้ไทยที่สุด

gcloud projects create $PROJECT_ID
gcloud config set project $PROJECT_ID
# ผูก billing: Console → Billing → Link a billing account กับ project นี้

gcloud services enable run.googleapis.com cloudbuild.googleapis.com \
  artifactregistry.googleapis.com secretmanager.googleapis.com
```

## 3. เก็บ secrets ใน Secret Manager

แต่ละคำสั่งจะรอให้วางค่า (ตัวอักษรจะไม่ขึ้นบนจอ) แล้วกด Enter ค่าจะไม่ไปค้างใน shell history และไม่มีขึ้นบรรทัดใหม่ติดไปด้วย

```bash
secret() { printf '%s: ' "$1"; read -rs v; echo; printf '%s' "$v" | gcloud secrets create "$1" --data-file=-; unset v; }

secret line-channel-secret        # วาง Channel secret
secret line-channel-access-token  # วาง Channel access token
secret database-url               # วาง Neon connection string
```

## 4. สร้าง service account ให้บอท

ให้บอทใช้ service account ของตัวเอง ที่อ่านได้เฉพาะ 3 secrets นี้

```bash
gcloud iam service-accounts create jodbanchee-run --display-name="jodbanchee bot"
export SA=jodbanchee-run@$PROJECT_ID.iam.gserviceaccount.com

for s in line-channel-secret line-channel-access-token database-url; do
  gcloud secrets add-iam-policy-binding $s \
    --member=serviceAccount:$SA --role=roles/secretmanager.secretAccessor
done
```

## 5. Deploy

รันจากโฟลเดอร์โปรเจกต์ Cloud Build จะ build จาก `Dockerfile` ให้เอง

```bash
gcloud run deploy jodbanchee \
  --source . \
  --region $REGION \
  --service-account $SA \
  --allow-unauthenticated \
  --set-secrets LINE_CHANNEL_SECRET=line-channel-secret:latest,LINE_CHANNEL_ACCESS_TOKEN=line-channel-access-token:latest,DATABASE_URL=database-url:latest \
  --memory 256Mi \
  --max-instances 2
```

- `--allow-unauthenticated` จำเป็น เพราะ LINE ต้องเรียก webhook ได้ แต่บอทยังปลอดภัยอยู่ เพราะทุก request ต้องมีลายเซ็นที่ถูกต้องจาก Channel secret
- `--max-instances 2` กันไม่ให้ scale เกินจนเกิดค่าใช้จ่าย ถ้ามีคนยิง request มาเยอะผิดปกติ
- ถ้า gcloud ถามเรื่องสร้าง Artifact Registry repository หรือให้สิทธิ์ Cloud Build ให้ตอบ `Y`

พอเสร็จจะได้ URL แบบ `https://jodbanchee-xxxxx.asia-southeast1.run.app` ลองเช็ก:

```bash
curl https://jodbanchee-xxxxx.asia-southeast1.run.app/health   # ควรได้ ok
```

> ใช้ `/health` อย่าใช้ `/healthz` เพราะ Cloud Run สงวน path บางตัวที่ลงท้ายด้วย `z` ไว้

## 6. ชี้ LINE มาที่ Cloud Run

LINE Developers Console → Messaging API → **Webhook URL** ใส่
`https://jodbanchee-xxxxx.asia-southeast1.run.app/webhook` แล้วกด **Verify**

ปิด cloudflared กับ `make run` ในเครื่องได้เลย บอทย้ายขึ้น cloud แล้ว

## 7. ตั้ง budget alert (สำคัญ)

Console → **Billing → Budgets & alerts → Create budget** ตั้งไว้สัก **$1** แล้วเปิดแจ้งเตือนทางอีเมลที่ 50% / 90% / 100%

## ใช้งานประจำ

| ต้องการ | คำสั่ง |
|---|---|
| ดู log | `gcloud run services logs read jodbanchee --region $REGION --limit 50` |
| อัปเดตโค้ด | รันคำสั่ง `gcloud run deploy` ในขั้นที่ 5 อีกครั้ง |
| เปลี่ยน token | `read -rs v; printf '%s' "$v" \| gcloud secrets versions add line-channel-access-token --data-file=-; unset v` แล้ว deploy ใหม่ |
| backup ข้อมูล | Neon มี point-in-time restore ในตัว หรือ `pg_dump "$DATABASE_URL" > backup.sql` |

## เรื่องที่ควรรู้

- **ข้อความแรกหลังเงียบไปนานอาจช้า 1–3 วินาที** เพราะ Cloud Run และ Neon ต้องตื่นจาก scale-to-zero ทั้งคู่ ถือเป็นเรื่องปกติ LINE รอได้
- **ย้ายข้อมูลทดสอบ:** บัญชีในไฟล์ `data/ledger.json` จากตอนทดสอบในเครื่องจะไม่ย้ายตามมา บอทบน cloud เริ่มจากศูนย์
