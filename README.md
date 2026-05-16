# go-bot

不需暴露 HTTPS endpoint 的聊天平台 bot 封裝庫。目前實裝 Telegram（long polling）與獨立 TTS package。

## telegram

| API | 用途 |
|---|---|
| `New(token, opts ...Option)` | 建立 `*Bot`；空 token 即 err |
| `Start(ctx)` | 啟動 long polling goroutine |
| `Close()` | cancel + 等 goroutine；冪等 |
| `Status()` | 回 `Status{Running, Username, UserID}` |
| `Reply(handler)` | 註冊 sync `ReplyHandler` |
| `Send(ctx, chatID, replyTo, text, opts ...MessageOption)` | 發訊息；`WithSendType(TypeMarkdown / TypeHTML)` 套 ParseMode |
| `Delete(ctx, chatID, msgID)` | 刪訊息 |
| `SendFile(ctx, chatID, t, path, caption...)` | `t = TypeDocument / TypeVideo / TypeAudio` |
| `SendPhoto(ctx, chatID, paths, caption...)` | 1 張單 API、2–10 張 album |
| `SendVoice(ctx, chatID, text, apiKey, caption...)` | 內部呼 `tts.Get` 轉 OGG 後上傳 |
| `SendInput(ctx, chatID, replyTo, text, opts ...MessageOption)` | `ForceReply` prompt |
| `SendSelect(ctx, chatID, replyTo, text, items, opts ...MessageOption)` | inline keyboard single-select |
| `SendMultiSelect(ctx, chatID, replyTo, text, items, opts ...MessageOption)` | inline keyboard multi-select |
| `SendStatus(ctx, chatID, replyTo, text, opts ...StatusOption)` | per-chat 單一「思考中」訊息；首次加 reaction（預設 🤔，`WithStatusEmoji` 覆寫）；`WithStatusSendType` 套 ParseMode；debounce 1s |
| `FinishStatus(ctx, chatID)` | 清 reaction + 刪 status 訊息 |

`Option`：`WithHTTPClient(*http.Client)`、`WithPollTimeout(time.Duration)`。
`MessageOption`：`WithSendType(SendType)`。
`StatusOption`：`WithStatusEmoji(string)`、`WithStatusSendType(SendType)`。

`Input` 欄位：`ChatID / MessageID / UserID / Username / Text / Caption / Photo / Document / CallbackData / CallbackPicks / Raw`。`CallbackData` 為 single-select 結果、`CallbackPicks` 為 multi-select 完成結果，兩者互斥。

```go
import (
    "context"
    "github.com/pardnchiu/go-bot/telegram"
)

bot, err := telegram.New(token)
if err != nil {
    return err
}
defer bot.Close()

bot.Reply(func(ctx context.Context, in telegram.Input) string {
    return "echo: " + in.Text
})

if err := bot.Start(ctx); err != nil {
    return err
}

bot.Send(ctx, chatID, 0, "*hello*", telegram.WithSendType(telegram.TypeMarkdown))
bot.SendFile(ctx, chatID, telegram.TypeDocument, "./report.pdf")
bot.SendPhoto(ctx, chatID, []string{"./a.png", "./b.png"}, "album caption")
bot.SendVoice(ctx, chatID, "今天天氣不錯", geminiAPIKey)
bot.SendSelect(ctx, chatID, triggerMsgID, "選個顏色", []string{"紅", "綠", "藍"})
bot.SendMultiSelect(ctx, chatID, triggerMsgID, "選幾個興趣", []string{"閱讀", "電影", "音樂"})
bot.SendInput(ctx, chatID, triggerMsgID, "你叫什麼名字？")
bot.SendStatus(ctx, chatID, userMsgID, "正在處理...", telegram.WithStatusEmoji("⚡"))
bot.FinishStatus(ctx, chatID)
```

## tts

| API | 用途 |
|---|---|
| `Get(ctx, apiKey, text)` | 回 `([]byte, error)`；Gemini TTS → ffmpeg OGG/OPUS |

需 `ffmpeg` in PATH；`apiKey` 為 Gemini API key（caller 自行讀 `GEMINI_API_KEY` / `GOOGLE_API_KEY`）。

```go
import "github.com/pardnchiu/go-bot/tts"

ogg, err := tts.Get(ctx, apiKey, "今天天氣不錯")
```
