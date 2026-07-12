# go-bot - Documentation

> Back to [README](../README.md)

## Prerequisites

- Go 1.25.0 or later.
- A platform token for the bot you use: Telegram, Discord, or LINE.
- `ffmpeg` in `PATH` when using `tts.Get`, `telegram.SendVoice`, or `discord.SendVoice`.
- A Gemini API key for text-to-speech.
- A public HTTPS endpoint for LINE webhooks; Telegram and Discord connect outbound and can run behind NAT.

## Installation

### Add to a Go module

```bash
go get github.com/pardnchiu/go-bot
```

### Build the included contract examples

```bash
git clone https://github.com/pardnchiu/go-bot.git
cd go-bot
go build ./...
```

## Configuration

The examples read credentials from `.env`, which Make includes directly. Keep values unquoted and never commit this file.

| Variable | Required | Used by | Description |
|---|---:|---|---|
| `TELEGRAM_TOKEN` | Telegram only | `cmd/tg` | Telegram bot token |
| `TELEGRAM_CHAT_ID` | send examples | `cmd/tg` | Target chat ID |
| `DISCORD_TOKEN` | Discord only | `cmd/dc` | Discord bot token |
| `DISCORD_CHANNEL_ID` | send examples | `cmd/dc` | Target channel ID |
| `LINEBOT_SECRET` | LINE only | `cmd/line` | LINE channel secret |
| `LINEBOT_TOKEN` | LINE only | `cmd/line` | LINE channel access token |
| `LINEBOT_TO` | LINE send | `cmd/line` | User, group, or room target ID |
| `LINEBOT_PORT` | No | `cmd/line` | Webhook port; defaults to `16722` |
| `LINEBOT_WEBHOOK` | No | `cmd/line` | Webhook path; defaults to `/linebot/webhook` |
| `GEMINI_API_KEY` or `GOOGLE_API_KEY` | voice only | examples | Gemini TTS API key |

For Discord message bodies, enable **Message Content Intent** in the Developer Portal.

## Usage

### Basic Telegram reply bot

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

### Discord interaction and attachment handling

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

### Run contract examples

```bash
make listen
make send TEXT="hello"
make dc-bot
make dc-send TEXT="hello"
make line-listen
make line-send TEXT="hello"
```

## API Reference

### Shared convention

Each platform exposes `New`, `Start`, `Close`, `Status`, and `Reply`. `Reply` accepts a synchronous handler and sends its non-empty return value to the source conversation. `Close` is idempotent.

### Telegram

| API | Signature / purpose |
|---|---|
| Constructor | `telegram.New(token, opts ...Option)` |
| Messaging | `Send`, `Delete`, `SendFile`, `SendPhoto`, `SendVoice` |
| Interaction | `SendInput`, `SendSelect`, `SendMultiSelect` |
| Status | `SendStatus`, `FinishStatus` |
| Download | `SaveFile(ctx, fileID, dir)`; 20 MB cap |

`WithHTTPClient` and `WithPollTimeout` configure polling. `WithSendType` selects plain text, MarkdownV2, or HTML.

### Discord

| API | Signature / purpose |
|---|---|
| Constructor | `discord.New(token)` |
| Messaging | `Send`, `Delete`, `SendFiles`, `SendVoice` |
| Interaction | `SendInput`, `SendSelect`, `SendMultiSelect` |
| Status | `SendStatus`, `FinishStatus` |
| Download | `Save(ctx, attachment, dir)`; 25 MiB cap |

Discord input uses a button-to-modal flow; select menus return a single `Text` value or `CallbackPicks` for multi-select.

### LINE

| API | Signature / purpose |
|---|---|
| Constructor | `line.New(secret, token, port, opts ...Option)` |
| Messaging | `Send(ctx, to, text)` uses PushMessage |
| Download | `Save(ctx, messageID, dir)`; 50 MiB cap |
| Webhook path | `line.WithPath(path)` |

LINE handles text, image, video, audio, and file message events. It intentionally does not provide the Telegram or Discord interaction components.

### Text to speech

```go
func Get(ctx context.Context, apiKey, text string) ([]byte, error)
```

`tts.Get` requests Gemini audio, converts PCM to OGG/OPUS with `ffmpeg`, and returns the encoded bytes.

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
