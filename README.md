# go-bot

不需暴露 HTTPS endpoint 的聊天平台 bot 封裝庫。目前實裝 Telegram（long polling）、Discord（WebSocket gateway）、獨立 TTS package。

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
| `SaveFile(ctx, fileID, dir)` | 下載任意 fileID 落地 `dir`；filename = UUID + server 副檔名；20 MB hard cap（超過 error 不截斷）；atomic write via `go-pkg/filesystem`。回傳完整 path |

`Option`：`WithHTTPClient(*http.Client)`、`WithPollTimeout(time.Duration)`。
`MessageOption`：`WithSendType(SendType)`。
`StatusOption`：`WithStatusEmoji(string)`、`WithStatusSendType(SendType)`。

`Input` 欄位：`ChatID / ChatName / MessageID / UserID / Username / Text / Caption / Photo / Document / CallbackData / CallbackPicks / Raw`。`ChatName` 自動填：`Chat.Title`（group/channel）→ `Chat.Username`（public username）→ `FirstName + LastName`（private 1-1）→ `""`。`CallbackData` 為 single-select 結果、`CallbackPicks` 為 multi-select 完成結果，兩者互斥。

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

path, _ := bot.SaveFile(ctx, in.Photo[len(in.Photo)-1].FileID, "./tmp")
```

## discord

底層 `bwmarrin/discordgo` v0.29.0；走 WebSocket gateway，**不需暴露 webhook endpoint**。

| API | 用途 |
|---|---|
| `New(token)` | 建立 `*Bot`；token 內部自動加 `"Bot "` prefix；空 token 即 err |
| `Start(ctx)` | REST `User("@me")` 驗 token → `Session.Open()` 拉 WebSocket |
| `Close()` | cancel + `Session.Close()`；冪等 |
| `Status()` | 回 `Status{Running, Username, UserID string}` |
| `Reply(handler)` | 註冊 sync `ReplyHandler`；過濾 bot 自己訊息 |
| `Send(ctx, channelID, replyTo, text)` | `ChannelMessageSendComplex`；`replyTo != ""` 掛 `MessageReference` |
| `Delete(ctx, channelID, messageID)` | 薄封裝 `ChannelMessageDelete`；典型流程：用 SendInput / SendSelect 拿到 `prompt.ID`，互動完成後 caller 自行清掉 |
| `SendFiles(ctx, channelID, replyTo, paths, caption...)` | 1-10 個 attachment 一次上傳（Discord 單一 API，無 telegram single/album 分流）；`os.Open` streaming；server 從副檔名推 MIME，無 FileType enum |
| `SendVoice(ctx, channelID, replyTo, text, apiKey, caption...)` | 內部 `tts.Get` 拿 OGG/OPUS → 上傳為 `audio/ogg` attachment（**非**波形 voice message bubble；discordgo v0.29.0 沒暴露 `duration_secs/waveform` 設值欄位） |
| `SendInput(ctx, channelID, replyTo, prompt)` | 送 prompt 訊息掛 Primary Button；user 點 button → 開 Modal（TextInput Required, Short, Title 取 prompt 前 45 char）→ user 填字送出觸發 ReplyHandler 帶 `Input.Text` |
| `SendSelect(ctx, channelID, replyTo, text, items)` | StringSelectMenu single-pick（MinValues=1, MaxValues=1）；1-25 items；user 點選觸發 ReplyHandler 帶 `Input.Text` = 選項；library 自動清空 dropdown components |
| `SendMultiSelect(ctx, channelID, replyTo, text, items)` | StringSelectMenu multi-pick（MinValues=0, MaxValues=len）；Discord 原生多選，**無需** telegram 那套 ✅/⬜ + Done button 中介；submit 觸發 ReplyHandler 帶 `Input.CallbackPicks` 為勾選列表 |
| `SendStatus(ctx, channelID, replyTo, text, opts ...StatusOption)` | per-channel 單一「思考中」訊息；首次 `MessageReactionAdd`（預設 🤔，`WithStatusEmoji` 覆寫）；後續 `ChannelMessageEdit`；debounce 1s |
| `FinishStatus(ctx, channelID)` | `MessageReactionRemove("@me")` + `ChannelMessageDelete` |
| `Save(ctx, att *discordgo.MessageAttachment, dir)` | 把 `input.Attachments` 收到的附件下載到 `dir`；filename = UUID + 原副檔名；25 MiB hard cap（對齊 Discord 免費 server 上限，Nitro / boost 大檔會被 reject）；atomic write via `go-pkg/filesystem`。回傳完整 path |

`StatusOption`：`WithStatusEmoji(string)`。Discord 不支援 ParseMode 切換（auto-markdown），無 SendType option。

`Input` 欄位：`ChannelID / ChannelName / GuildID / MessageID / UserID / Username / Text / Attachments / CallbackPicks / Raw`（全 string）。`ChannelName` 從 discordgo `Session.State` cache 取（gateway 連線時 `GUILD_CREATE` 已自動塞滿，零 round-trip）；DM 或 cache miss 為空字串。需開 **Message Content Intent**（Developer Portal → Bot）否則 `Text` 為空（除 mention / DM / 自己訊息外）。`Attachments` 為 user 上傳的全部附件、`CallbackPicks` 只在 `SendMultiSelect` 完成收尾時 non-empty；`Text` 與 `CallbackPicks` 互斥（single-tap / modal answer 走 `Text`，multi-select 走 `CallbackPicks`）。

```go
import (
    "context"
    "github.com/pardnchiu/go-bot/discord"
)

bot, err := discord.New(token)
if err != nil {
    return err
}
defer bot.Close()

bot.Reply(func(ctx context.Context, in discord.Input) string {
    for _, att := range in.Attachments {
        path, _ := bot.Save(ctx, att, "./tmp")
        _ = path
    }
    return "echo: " + in.Text
})

if err := bot.Start(ctx); err != nil {
    return err
}

bot.Send(ctx, channelID, "", "hello")
bot.SendFiles(ctx, channelID, "", []string{"./a.png", "./b.png"}, "two images")
bot.SendVoice(ctx, channelID, "", "今天天氣不錯", geminiAPIKey)
bot.SendInput(ctx, channelID, triggerMsgID, "你叫什麼名字？")
bot.SendSelect(ctx, channelID, triggerMsgID, "選個顏色", []string{"紅", "綠", "藍"})
bot.SendMultiSelect(ctx, channelID, triggerMsgID, "選幾個興趣", []string{"閱讀", "電影", "音樂"})
bot.SendStatus(ctx, channelID, userMsgID, "思考中...")
bot.FinishStatus(ctx, channelID)
bot.Delete(ctx, channelID, promptMsgID)
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
