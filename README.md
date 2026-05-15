# go-bot

不需要對外暴露 HTTPS endpoint 即可運行的聊天平台 bot 封裝庫，分階段一個一個 platform 實作。各 platform package 自行宣告 `Input` 與 `ReplyHandler`，**不抽** `internal/shared`；對外統一的只有「`Reply(handler)` + handler return string 即送回」這個慣例。目前實裝 Telegram（long polling）與獨立 TTS package。

不支援 LINE：LINE 走 webhook（inbound — 平台往主機 push），需要公開 HTTPS endpoint；Telegram long polling 與 Discord WebSocket gateway 皆 outbound（bot 連出去），本機／NAT 後／無公網 IP 都能跑。本專案定位是「不需要暴露主機就能跑的 bot library」，因此 LINE 排除。

## 內容

### telegram

Telegram Bot API 封裝。`New(token)` 建立 client（空 token 即 err，**不**啟動 polling）；`Start(ctx)` 啟 long polling goroutine（`GetMe` 失敗即 fatal、`DeleteWebhook` 失敗僅 warn）；`Close()` 冪等收尾 — cancel parent ctx、等 goroutine、停所有 status timer、重置 multi-select state。

訊息傳送一律走 streaming 上傳（`os.Open` 或 `bytes.NewReader`），不在 process 內聚積 binary。`Send` / `SendFile` / `SendPhoto` / `SendVoice` / `SendInput` / `SendSelect` / `SendMultiSelect` 全部吃 `replyTo int`：傳 `0` 表 standalone，`> 0` 自動掛 `ReplyParameters{MessageID: replyTo}`。

互動 UI 的生命週期由 dispatch 內部處理：

- **Single-select**（`SendSelect`）— inline keyboard 每項一行 button，user 點按 → library 自動 `AnswerCallbackQuery` ack + `EditMessageReplyMarkup` 清 keyboard → `Input.CallbackData` 進 ReplyHandler 一次
- **Multi-select**（`SendMultiSelect`）— inline keyboard 每項一行 toggle + 最後一行「完成」；toggle 不呼 handler，僅由 library 翻轉 selected map 並重繪 ✅／⬜；按「完成」才清 keyboard、刪 state、以 `Input.CallbackPicks` 呼 handler 一次。item 不可為 sentinel `"__bot_multiselect_done__"`，否則 `SendMultiSelect` 回 err
- **Force reply**（`SendInput`）— prompt 掛 `ForceReply{ForceReply: true}`，user 端自動彈鍵盤聚焦於 reply 模式；user 回覆走 `update.Message.Text`，`Input.Raw.Message.ReplyToMessage.ID` 指回 prompt msg 供 caller 關聯

`SendStatus(ctx, chatID, replyTo, text)` 維護 per-chat 單一「思考中」訊息：首次 flush 對 `replyTo` 加 🤔 reaction（best-effort，失敗 warn 不擋），送一則新訊息帶 `ReplyParameters{MessageID: replyTo}`；後續同 chat 重呼 → `editMessageText` 同訊息（debounce 1s，Telegram per-chat cap），同訊息同文字自動跳 no-op。`FinishStatus(ctx, chatID)` 等 in-flight edit 收尾、`SetMessageReaction` clear bot reaction、刪 status 訊息；無 status entry 為 no-op。

`Reply(handler)` 註冊同步 handler；dispatch 收到 update → 呼 `handler(ctx, Input)` 等回傳 → 非空 string 自動 `SendMessage` 帶 `ReplyParameters{MessageID}` 掛在 user 原訊息下送回同 chat。空字串不送、handler panic 由 `recover` 接住不送，不會打掛 SDK goroutine。`Input.Raw` 留 `*models.Update` 供 caller 取 SDK-only 欄位（如下載 Photo / Document 原始 bytes）。

| API | 行為 |
|---|---|
| `New(token)` | 建立 `*Bot`；空 token 即 err；**不**啟動 polling |
| `Start(ctx)` | 啟 long polling goroutine；重複呼叫回 `already started` |
| `Close()` | cancel + 等 goroutine + 清 timer / multi-select state；冪等 |
| `Status()` | 回 `Status{Running, Username, UserID}` |
| `Send(ctx, chatID, replyTo, text, sendType...)` | 薄封裝 `SendMessage`；`sendType` 選用，`TypeMarkdown`（→ MarkdownV2）／`TypeHTML`，省略為 plain text |
| `Delete(ctx, chatID, msgID)` | 薄封裝 `DeleteMessage` |
| `SendFile(ctx, chatID, t, path, caption...)` | `t = TypeDocument / TypeVideo / TypeAudio`；`os.Open` streaming；unknown type 即 err |
| `SendPhoto(ctx, chatID, paths, caption...)` | 1 張走 `SendPhoto` 單張 API、2–10 張走 `SendMediaGroup` album；超出範圍即 err |
| `SendVoice(ctx, chatID, text, apiKey, caption...)` | 內部呼 `tts.Get(ctx, apiKey, text)` 取 OGG bytes → `bytes.NewReader` 上傳 |
| `SendInput(ctx, chatID, replyTo, text)` | 送 `ForceReply` prompt |
| `SendSelect(ctx, chatID, replyTo, text, items)` | 送 inline keyboard single-select |
| `SendMultiSelect(ctx, chatID, replyTo, text, items)` | 送 inline keyboard multi-select；item 命中 sentinel 即 err |
| `SendStatus(ctx, chatID, replyTo, text)` | per-chat 單一思考中訊息；首次加 🤔 reaction；debounce 1s；同文字 no-op |
| `FinishStatus(ctx, chatID)` | 等 in-flight edit、clear reaction、刪 status 訊息；冪等 |
| `Reply(handler)` | 註冊 sync `ReplyHandler` |

`Input` 帶下列欄位：`ChatID / MessageID / UserID / Username / Text / Caption / Photo / Document / CallbackData / CallbackPicks / Raw`。`MessageID` 在 text update 下是 user 原訊息 ID；在 CallbackQuery 下是 bot 的 prompt msg.ID（供 `SendStatus(..., replyTo, ...)` 使用）。`CallbackData` 與 `CallbackPicks` 互斥：single-tap 走 `CallbackData`、multi-select 完成走 `CallbackPicks`。

<details>
<summary>範例</summary>

```go
import (
    "context"
    "strings"
    "github.com/pardnchiu/go-bot/telegram"
)

bot, err := telegram.New(token)
if err != nil {
    return err
}
defer bot.Close()

bot.Reply(func(ctx context.Context, in telegram.Input) string {
    switch {
    case in.CallbackData != "":
        return "you picked: " + in.CallbackData
    case len(in.CallbackPicks) > 0:
        return "you picked: " + strings.Join(in.CallbackPicks, ", ")
    default:
        return "echo: " + in.Text
    }
})

if err := bot.Start(ctx); err != nil {
    return err
}

// 主動發送
msg, _ := bot.Send(ctx, chatID, 0, "hello")
bot.Send(ctx, chatID, msg.ID, "reply to above")

// 檔案／圖片／語音
bot.SendFile(ctx, chatID, telegram.TypeDocument, "./report.pdf", "caption")
bot.SendPhoto(ctx, chatID, []string{"./a.png", "./b.png"}, "album caption")
bot.SendVoice(ctx, chatID, "今天天氣不錯", geminiAPIKey)

// 互動 UI（全部 reply 到 trigger msg）
bot.SendSelect(ctx, chatID, triggerMsgID, "選個顏色", []string{"紅", "綠", "藍"})
bot.SendMultiSelect(ctx, chatID, triggerMsgID, "選幾個興趣", []string{"閱讀", "電影", "音樂"})
bot.SendInput(ctx, chatID, triggerMsgID, "你叫什麼名字？")

// 思考中狀態
bot.SendStatus(ctx, chatID, userMsgID, "正在處理...")
bot.SendStatus(ctx, chatID, userMsgID, "查詢資料庫")
bot.FinishStatus(ctx, chatID)
```

</details>

### tts

text → OGG/OPUS bytes。`Get(ctx, apiKey, text)` 呼叫 Gemini TTS 取得 PCM，經 `ffmpeg` 轉 OGG/OPUS 回 `[]byte`。Package 不知道 Telegram，可獨立給其他 platform 使用；`telegram.SendVoice` 是 caller 一行送 voice 的便利封裝（內部就是呼 `tts.Get` + 上傳）。

- 預設 model `gemini-3.1-flash-tts-preview`、voice `Kore`、sample rate `24000Hz`、bitrate `32k`
- 依賴 `ffmpeg` in PATH，缺失即 err
- 環境變數 `GEMINI_API_KEY` 或 `GOOGLE_API_KEY` 任一即可（caller 自行讀取後傳入）

| API | 行為 |
|---|---|
| `Get(ctx, apiKey, text)` | 回 `([]byte, error)`；OGG/OPUS bytes |

<details>
<summary>範例</summary>

```go
import "github.com/pardnchiu/go-bot/tts"

ogg, err := tts.Get(ctx, apiKey, "今天天氣不錯")
if err != nil {
    return err
}
// ogg 可直接寫檔或上傳至任一 platform
```

</details>

---
