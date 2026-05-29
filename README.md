# go-bot

聊天平台 bot 封裝庫。實裝 Telegram（long polling）、Discord（WebSocket gateway）、LINE（webhook）、TTS。Telegram / Discord 為 outbound 連線，本機 / NAT 後皆可跑；LINE 走 webhook（inbound），需公開 HTTPS endpoint。

## telegram

| API | 用途 |
|---|---|
| `New(token, opts ...Option)` | 建立 `*Bot` |
| `Start(ctx)` | 啟動 long polling |
| `Close()` | 停止；冪等 |
| `Status()` | `{Running, Username, UserID}` |
| `Reply(handler)` | 註冊 sync handler |
| `Send(ctx, chatID, replyTo, text, opts...)` | 發訊息 |
| `Delete(ctx, chatID, msgID)` | 刪訊息 |
| `SendFile(ctx, chatID, t, path, caption...)` | `t = TypeDocument / TypeVideo / TypeAudio` |
| `SendPhoto(ctx, chatID, paths, caption...)` | 1–10 張 |
| `SendVoice(ctx, chatID, text, apiKey, caption...)` | TTS → OGG 上傳 |
| `SendInput(ctx, chatID, replyTo, text, opts...)` | ForceReply prompt |
| `SendSelect(ctx, chatID, replyTo, text, items, opts...)` | inline keyboard single-select |
| `SendMultiSelect(ctx, chatID, replyTo, text, items, opts...)` | inline keyboard multi-select |
| `SendStatus(ctx, chatID, replyTo, text, opts...)` | 「思考中」訊息 + reaction；debounce 1s |
| `FinishStatus(ctx, chatID)` | 清 reaction + 刪 status |
| `SaveFile(ctx, fileID, dir)` | 下載落地；UUID 命名；20 MB cap |

`Option`：`WithHTTPClient` / `WithPollTimeout`
`MessageOption`：`WithSendType(TypeMarkdown / TypeHTML)`
`StatusOption`：`WithStatusEmoji` / `WithStatusSendType`

`Input`：`ChatID / ChatName / MessageID / UserID / Username / Text / Caption / Photo / Document / CallbackData / CallbackPicks / Raw`

```go
bot, _ := telegram.New(token)
defer bot.Close()

bot.Reply(func(ctx context.Context, in telegram.Input) string {
    return "echo: " + in.Text
})
bot.Start(ctx)

bot.Send(ctx, chatID, 0, "*hello*", telegram.WithSendType(telegram.TypeMarkdown))
bot.SendFile(ctx, chatID, telegram.TypeDocument, "./report.pdf")
bot.SendPhoto(ctx, chatID, []string{"./a.png", "./b.png"}, "caption")
bot.SendVoice(ctx, chatID, "今天天氣不錯", geminiAPIKey)
bot.SendSelect(ctx, chatID, msgID, "選個顏色", []string{"紅", "綠", "藍"})
bot.SendMultiSelect(ctx, chatID, msgID, "選興趣", []string{"閱讀", "電影"})
bot.SendInput(ctx, chatID, msgID, "你叫什麼名字？")
bot.SendStatus(ctx, chatID, msgID, "處理中...")
bot.FinishStatus(ctx, chatID)

path, _ := bot.SaveFile(ctx, in.Photo[len(in.Photo)-1].FileID, "./tmp")
```

## discord

底層 `bwmarrin/discordgo`；WebSocket gateway。

| API | 用途 |
|---|---|
| `New(token)` | 建立 `*Bot` |
| `Start(ctx)` | 驗 token + 開 WebSocket |
| `Close()` | 停止；冪等 |
| `Status()` | `{Running, Username, UserID}` |
| `Reply(handler)` | 註冊 sync handler |
| `Send(ctx, channelID, replyTo, text)` | 發訊息 |
| `Delete(ctx, channelID, messageID)` | 刪訊息 |
| `SendFiles(ctx, channelID, replyTo, paths, caption...)` | 1–10 個 attachment |
| `SendVoice(ctx, channelID, replyTo, text, apiKey, caption...)` | TTS → OGG attachment |
| `SendInput(ctx, channelID, replyTo, prompt)` | Button → Modal → ReplyHandler |
| `SendSelect(ctx, channelID, replyTo, text, items)` | StringSelectMenu single-pick |
| `SendMultiSelect(ctx, channelID, replyTo, text, items)` | StringSelectMenu multi-pick |
| `SendStatus(ctx, channelID, replyTo, text, opts...)` | 「思考中」訊息 + reaction；debounce 1s |
| `FinishStatus(ctx, channelID)` | 清 reaction + 刪 status |
| `Save(ctx, att, dir)` | 下載 attachment 落地；UUID 命名；25 MiB cap |

`StatusOption`：`WithStatusEmoji`

`Input`：`ChannelID / ChannelName / GuildID / MessageID / UserID / Username / Text / Attachments / CallbackPicks / Raw`

需在 Developer Portal 開 **Message Content Intent**，否則 `Text` 為空（mention / DM / 自己訊息除外）。

```go
bot, _ := discord.New(token)
defer bot.Close()

bot.Reply(func(ctx context.Context, in discord.Input) string {
    for _, att := range in.Attachments {
        bot.Save(ctx, att, "./tmp")
    }
    return "echo: " + in.Text
})
bot.Start(ctx)

bot.Send(ctx, channelID, "", "hello")
bot.SendFiles(ctx, channelID, "", []string{"./a.png", "./b.png"}, "caption")
bot.SendVoice(ctx, channelID, "", "今天天氣不錯", geminiAPIKey)
bot.SendInput(ctx, channelID, msgID, "你叫什麼名字？")
bot.SendSelect(ctx, channelID, msgID, "選個顏色", []string{"紅", "綠", "藍"})
bot.SendMultiSelect(ctx, channelID, msgID, "選興趣", []string{"閱讀", "電影"})
bot.SendStatus(ctx, channelID, msgID, "思考中...")
bot.FinishStatus(ctx, channelID)
bot.Delete(ctx, channelID, promptID)
```

## line

底層 `line-bot-sdk-go/v8`；webhook server（inbound，需公開 HTTPS endpoint）。範疇最小：只有 `Reply` + `Send`，無互動元件。

| API | 用途 |
|---|---|
| `New(secret, token, port, opts ...Option)` | 建立 `*Bot`；listen `:port` |
| `Start(ctx)` | 驗 token + 起 webhook server |
| `Close()` | 停止；冪等 |
| `Status()` | `{Running, UserID, BasicID, DisplayName}` |
| `Reply(handler)` | 註冊 sync handler；走 reply token 回覆 |
| `Send(ctx, to, text)` | PushMessage 到指定 target（user / group / room id） |

`Option`：`WithPath`（webhook path，預設 `/linebot/webhook`）

`Input`：`SourceType / UserID / GroupID / RoomID / ReplyToken / MessageID / Text / Raw`

需在 LINE Developer Console 回填 webhook URL（`https://<domain>/linebot/webhook`）、開 **Use webhook**、關 auto-reply。

```go
bot, _ := line.New(secret, token, "16722")
defer bot.Close()

bot.Reply(func(ctx context.Context, in line.Input) string {
    return "echo: " + in.Text
})
bot.Start(ctx)

bot.Send(ctx, userID, "hello")
```

## tts

| API | 用途 |
|---|---|
| `Get(ctx, apiKey, text)` | Gemini TTS → OGG/OPUS bytes |

需 `ffmpeg` in PATH。

```go
ogg, err := tts.Get(ctx, apiKey, "今天天氣不錯")
```
