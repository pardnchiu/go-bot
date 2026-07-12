# go-bot - 技術文件

> 返回 [README](./README.zh.md)

## 前置需求

- Go 1.25.0 或更新版本。
- 使用的 Bot 平台憑證：Telegram、Discord 或 LINE。
- 使用 `tts.Get`、`telegram.SendVoice` 或 `discord.SendVoice` 時，系統 `PATH` 必須有 `ffmpeg`。
- 文字轉語音需要 Gemini API key。
- LINE webhook 需要公開 HTTPS endpoint；Telegram 與 Discord 為 outbound 連線，可在 NAT 後執行。

## 安裝

### 加入 Go module

```bash
go get github.com/pardnchiu/go-bot
```

### 建置內附契約範例

```bash
git clone https://github.com/pardnchiu/go-bot.git
cd go-bot
go build ./...
```

## 設定

範例透過 Make 直接 include `.env`。值不可加引號，且不可提交此檔案。

| 變數 | 必要 | 使用處 | 說明 |
|---|---:|---|---|
| `TELEGRAM_TOKEN` | 僅 Telegram | `cmd/tg` | Telegram Bot token |
| `TELEGRAM_CHAT_ID` | 傳送範例 | `cmd/tg` | 目標 chat ID |
| `DISCORD_TOKEN` | 僅 Discord | `cmd/dc` | Discord Bot token |
| `DISCORD_CHANNEL_ID` | 傳送範例 | `cmd/dc` | 目標 channel ID |
| `LINEBOT_SECRET` | 僅 LINE | `cmd/line` | LINE channel secret |
| `LINEBOT_TOKEN` | 僅 LINE | `cmd/line` | LINE channel access token |
| `LINEBOT_TO` | LINE 傳送 | `cmd/line` | User、group 或 room target ID |
| `LINEBOT_PORT` | 否 | `cmd/line` | Webhook port；預設 `16722` |
| `LINEBOT_WEBHOOK` | 否 | `cmd/line` | Webhook path；預設 `/linebot/webhook` |
| `GEMINI_API_KEY` 或 `GOOGLE_API_KEY` | 僅語音 | 範例 | Gemini TTS API key |

Discord 若要取得一般訊息文字，請在 Developer Portal 啟用 **Message Content Intent**。

## 使用方式

### 基本 Telegram 回覆 Bot

```go
package main

import (
    "context"
    "log"

    "github.com/pardnchiu/go-bot/telegram"
)

func main() {
    ctx := context.Background()
    bot, err := telegram.New("<telegram-token>")
    if err != nil {
        log.Fatal(err)
    }
    defer bot.Close()

    bot.Reply(func(ctx context.Context, in telegram.Input) string {
        return "echo: " + in.Text
    })
    if err := bot.Start(ctx); err != nil {
        log.Fatal(err)
    }
    select {}
}
```

### Discord 互動與附件處理

```go
package main

import (
    "context"
    "log"

    "github.com/pardnchiu/go-bot/discord"
)

func main() {
    ctx := context.Background()
    bot, err := discord.New("<discord-token>")
    if err != nil {
        log.Fatal(err)
    }
    defer bot.Close()

    bot.Reply(func(ctx context.Context, in discord.Input) string {
        for _, attachment := range in.Attachments {
            if _, err := bot.Save(ctx, attachment, "./tmp"); err != nil {
                return "could not save attachment"
            }
        }
        return "echo: " + in.Text
    })
    if err := bot.Start(ctx); err != nil {
        log.Fatal(err)
    }
    select {}
}
```

### 執行契約範例

```bash
make listen
make send TEXT="hello"
make dc-bot
make dc-send TEXT="hello"
make line-listen
make line-send TEXT="hello"
```

## API 參考

### 共通慣例

每個平台都提供 `New`、`Start`、`Close`、`Status` 與 `Reply`。`Reply` 接受同步 handler，非空回傳值會送回來源對話；`Close` 可重複呼叫。

### Telegram

| API | 簽章／用途 |
|---|---|
| 建立 | `telegram.New(token, opts ...Option)` |
| 訊息 | `Send`、`Delete`、`SendFile`、`SendPhoto`、`SendVoice` |
| 互動 | `SendInput`、`SendSelect`、`SendMultiSelect` |
| 狀態 | `SendStatus`、`FinishStatus` |
| 下載 | `SaveFile(ctx, fileID, dir)`；20 MB 上限 |

`WithHTTPClient` 與 `WithPollTimeout` 用於設定 polling；`WithSendType` 可選 plain text、MarkdownV2 或 HTML。

### Discord

| API | 簽章／用途 |
|---|---|
| 建立 | `discord.New(token)` |
| 訊息 | `Send`、`Delete`、`SendFiles`、`SendVoice` |
| 互動 | `SendInput`、`SendSelect`、`SendMultiSelect` |
| 狀態 | `SendStatus`、`FinishStatus` |
| 下載 | `Save(ctx, attachment, dir)`；25 MiB 上限 |

Discord 的輸入流程由按鈕開啟 Modal；選單會以 `Text` 回傳單選結果，或以 `CallbackPicks` 回傳多選結果。

### LINE

| API | 簽章／用途 |
|---|---|
| 建立 | `line.New(secret, token, port, opts ...Option)` |
| 訊息 | `Send(ctx, to, text)` 使用 PushMessage |
| 下載 | `Save(ctx, messageID, dir)`；50 MiB 上限 |
| Webhook path | `line.WithPath(path)` |

LINE 處理 text、image、video、audio 與 file 事件；刻意不提供 Telegram 或 Discord 的互動元件。

### 文字轉語音

```go
func Get(ctx context.Context, apiKey, text string) ([]byte, error)
```

`tts.Get` 向 Gemini 請求音訊，使用 `ffmpeg` 將 PCM 轉為 OGG/OPUS，並回傳編碼後 bytes。

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
