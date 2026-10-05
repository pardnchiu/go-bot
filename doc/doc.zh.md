# go-bot - 技術文件

> 返回 [README](./README.zh.md)

## 前置需求

- Go 1.25.0 以上
- 至少一個平台的 bot 憑證：Telegram bot token、Discord bot token，或 LINE channel secret + channel access token
- 使用 LINE 時需可公開存取的 HTTPS endpoint（本機測試可用 ngrok 等反向代理）；Telegram 與 Discord 皆為 outbound 連線，可在 NAT 後執行

## 安裝

### 以 go get 加入模組

```bash
go get github.com/pardnchiu/go-bot
```

依需要的平台 import `core/` 下的套件：

```go
import (
    "github.com/pardnchiu/go-bot/core/discord"
    "github.com/pardnchiu/go-bot/core/line"
    "github.com/pardnchiu/go-bot/core/telegram"
)
```

### 從原始碼建置

```bash
git clone https://github.com/pardnchiu/go-bot.git
cd go-bot
go build ./...
```

## 設定

本函式庫不讀取環境變數，所有憑證皆由 caller 傳入建構函式。各平台需在開發者後台完成下列設定：

| 平台 | 設定 | 為何 |
|---|---|---|
| Telegram | 無 webhook 綁定（`Start` 若偵測到 webhook 會自動刪除） | long polling 與 webhook 互斥 |
| Discord | Developer Portal → Bot → 開啟 **Message Content Intent** | 未開啟時 `Input.Text` 只在 mention／DM 有值 |
| LINE | Developer Console → Messaging API → Webhook URL 填入 `https://<host><path>` 並啟用 | LINE 以 webhook 推送事件，預設 path 為 `/linebot/webhook` |

## 使用方式

### 基礎：Telegram 回覆機器人

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/pardnchiu/go-bot/core/telegram"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    bot, err := telegram.New(os.Getenv("TELEGRAM_TOKEN"))
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
    <-ctx.Done()
}
```

### 基礎：LINE webhook 機器人

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/pardnchiu/go-bot/core/line"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    bot, err := line.New(os.Getenv("LINEBOT_SECRET"), os.Getenv("LINEBOT_TOKEN"), "16722")
    if err != nil {
        log.Fatal(err)
    }
    defer bot.Close()

    bot.Reply(func(ctx context.Context, in line.Input) string {
        if in.MessageType != "text" {
            return ""
        }
        return "you said: " + in.Text
    })

    if err := bot.Start(ctx); err != nil {
        log.Fatal(err)
    }
    <-ctx.Done()
}
```

### 進階：Discord 附件落地與下拉選單

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "strings"
    "syscall"

    "github.com/pardnchiu/go-bot/core/discord"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    bot, err := discord.New(os.Getenv("DISCORD_TOKEN"))
    if err != nil {
        log.Fatal(err)
    }
    defer bot.Close()

    bot.Reply(func(ctx context.Context, in discord.Input) string {
        // 多選結果走 CallbackPicks
        if len(in.CallbackPicks) > 0 {
            return "picked: " + strings.Join(in.CallbackPicks, ", ")
        }
        // 附件逐一落地，回傳本機路徑
        for _, att := range in.Attachments {
            if _, err := bot.Save(ctx, att, "./tmp"); err != nil {
                return "save failed: " + err.Error()
            }
        }
        if in.Text == "color" {
            if _, err := bot.SendMultiSelect(ctx, in.ChannelID, in.MessageID, "選顏色（可多選）", []string{"紅", "綠", "藍"}); err != nil {
                log.Println(err)
            }
            return ""
        }
        return "echo: " + in.Text
    })

    if err := bot.Start(ctx); err != nil {
        log.Fatal(err)
    }
    <-ctx.Done()
}
```

### 進階：思考中狀態訊息

```go
bot.Reply(func(ctx context.Context, in telegram.Input) string {
    // 首次呼叫在原訊息加 reaction 並送出狀態訊息；之後編輯同一則訊息（1 秒去彈跳）
    if err := bot.SendStatus(ctx, in.ChatID, in.MessageID, "思考中...", telegram.WithStatusEmoji("👀")); err != nil {
        log.Println(err)
    }
    for i := 1; i <= 3; i++ {
        time.Sleep(time.Second)
        _ = bot.SendStatus(ctx, in.ChatID, in.MessageID, fmt.Sprintf("步驟 %d/3", i))
    }
    // 清除 reaction 並刪除狀態訊息
    if err := bot.FinishStatus(ctx, in.ChatID); err != nil {
        log.Println(err)
    }
    return "完成"
})
```

## API 參考

### 共通慣例

`core/telegram`、`core/discord`、`core/line` 各自宣告 `Bot`、`Input`、`ReplyHandler`、`Status`，對外一致的只有下列生命週期與回覆慣例：

| 方法 | 行為 |
|---|---|
| `New(...)` | 驗證必填參數並建立 client，不建立連線 |
| `Start(ctx)` | 先以 API 驗證 token（失敗即回 error），再啟動 polling／Gateway／webhook server |
| `Close()` | 取消內部 context、關閉連線並清除互動與狀態訊息 state；冪等 |
| `Status()` | 回傳目前執行狀態與 bot 身分 |
| `Reply(handler)` | 註冊 `func(ctx, Input) string`；回傳非空字串即回覆觸發訊息，handler panic 會被 recover |

### Telegram（`core/telegram`）

```go
func New(token string, opts ...Option) (*Bot, error)
func WithHTTPClient(client *http.Client) Option
func WithPollTimeout(d time.Duration) Option
```

`WithPollTimeout` 為 `getUpdates` 的 server 端 timeout；`WithHTTPClient` 的 `Timeout` 必須大於 poll timeout，否則閒置的 long poll 會在 server 回應前被中止。

| 方法 | 簽名 | 說明 |
|---|---|---|
| `Send` | `(ctx, chatID int64, replyTo int, text string, opts ...MessageOption) (*models.Message, error)` | 送文字；`replyTo > 0` 掛回覆對象 |
| `Delete` | `(ctx, chatID int64, msgID int) error` | 刪除訊息 |
| `SendFile` | `(ctx, chatID int64, fileType FileType, path string, caption ...string) (*models.Message, error)` | `TypeDocument`／`TypeVideo`／`TypeAudio`，串流上傳 |
| `SendPhoto` | `(ctx, chatID int64, paths []string, caption ...string) ([]*models.Message, error)` | 1 張走單張 API、2–10 張走 album |
| `SendVoice` | `(ctx, chatID int64, path string, caption ...string) (*models.Message, error)` | 上傳既有 OGG/OPUS 檔為語音訊息 |
| `SendInput` | `(ctx, chatID int64, replyTo int, text string, opts ...MessageOption) (*models.Message, error)` | 以 ForceReply 要求使用者回覆 |
| `SendSelect` | `(ctx, chatID int64, replyTo int, text string, items []string, opts ...MessageOption) (*models.Message, error)` | inline keyboard 單選，結果在 `Input.CallbackData` |
| `SendMultiSelect` | `(ctx, chatID int64, replyTo int, text string, items []string, opts ...MessageOption) (*models.Message, error)` | ✅／⬜ 切換 + 完成按鈕，結果在 `Input.CallbackPicks` |
| `SendStatus` | `(ctx, chatID int64, replyTo int, text string, opts ...StatusOption) error` | 每 chat 一則狀態訊息，1 秒去彈跳 |
| `FinishStatus` | `(ctx, chatID int64) error` | 清除 reaction 並刪除狀態訊息 |
| `Save` | `(ctx, fileID, dir string) (string, error)` | 下載檔案到 `dir`，上限 20 MiB，回傳路徑 |

| 選項 | 說明 |
|---|---|
| `WithSendType(TypeMarkdown \| TypeHTML)` | `MessageOption`；設定 ParseMode，未指定為純文字 |
| `WithStatusEmoji(emoji)` | `StatusOption`；reaction emoji，預設 `🤔` |
| `WithStatusSendType(t)` | `StatusOption`；狀態訊息的 ParseMode |

`Input` 欄位：`ChatID`、`ChatName`、`MessageID`、`UserID`、`Username`、`Text`、`Caption`、`Photo`、`Document`、`CallbackData`、`CallbackPicks`、`Raw *models.Update`。

### Discord（`core/discord`）

```go
func New(token string) (*Bot, error)
```

| 方法 | 簽名 | 說明 |
|---|---|---|
| `Send` | `(ctx, channelID, replyTo, text string) (*discordgo.Message, error)` | 送文字；`replyTo != ""` 掛回覆對象 |
| `Delete` | `(ctx, channelID, messageID string) error` | 刪除訊息 |
| `SendFiles` | `(ctx, channelID, replyTo string, paths []string, caption ...string) (*discordgo.Message, error)` | 單則訊息 1–10 個附件 |
| `SendVoice` | `(ctx, channelID, replyTo, path string, caption ...string) (*discordgo.Message, error)` | 上傳 OGG 為 audio 附件（非波形語音泡泡） |
| `SendInput` | `(ctx, channelID, replyTo, prompt string) (*discordgo.Message, error)` | 「回答」按鈕開啟 Modal，填入值在 `Input.Text` |
| `SendSelect` | `(ctx, channelID, replyTo, text string, items []string) (*discordgo.Message, error)` | 下拉單選（1–25 項），結果在 `Input.Text` |
| `SendMultiSelect` | `(ctx, channelID, replyTo, text string, items []string) (*discordgo.Message, error)` | 下拉多選，結果在 `Input.CallbackPicks` |
| `SendStatus` | `(ctx, channelID, replyTo, text string, opts ...StatusOption) error` | 每 channel 一則狀態訊息，1 秒去彈跳 |
| `FinishStatus` | `(ctx, channelID string) error` | 清除 reaction 並刪除狀態訊息 |
| `Save` | `(ctx, att *discordgo.MessageAttachment, dir string) (string, error)` | 下載附件到 `dir`，上限 25 MiB，回傳路徑 |

`WithStatusEmoji(emoji)` 設定 reaction emoji，預設 `🤔`。

`Input` 欄位：`ChannelID`、`ChannelName`、`GuildID`、`MessageID`、`UserID`、`Username`、`Text`、`Attachments`、`CallbackPicks`、`Raw *discordgo.MessageCreate`。互動事件中 `MessageID` 為 prompt 訊息 ID，可直接傳給 `Delete` 清除。

### LINE（`core/line`）

```go
func New(secret, token, port string, opts ...Option) (*Bot, error)
func WithPath(path string) Option
```

webhook server 監聽 `:<port>`，path 預設 `/linebot/webhook`。簽章錯誤回 400、未啟動回 503，每個事件 handler 有 30 秒 timeout。

| 方法 | 簽名 | 說明 |
|---|---|---|
| `Send` | `(ctx, to, text string) (*linebot.BasicResponse, error)` | PushMessage 到 user／group／room |
| `Save` | `(ctx, messageID, dir string) (string, error)` | 下載 image／video／audio／file 內容，上限 50 MiB，副檔名依 content-type 推斷 |

`Input` 欄位：`SourceType`、`UserID`、`Username`、`GroupID`、`RoomID`、`ReplyToken`、`MessageID`、`MessageType`（`text`／`image`／`video`／`audio`／`file`）、`Text`、`FileName`、`Raw *linebot.Event`。`Username` 每則訊息以 profile API 取得，失敗時為空字串。LINE 不提供互動元件與狀態訊息。

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
