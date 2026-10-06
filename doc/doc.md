# go-bot - Documentation

Last updated: 2026-10-06

> Back to [README](../README.md)

## Prerequisites

- Go 1.25.0 or later
- Credentials for at least one platform: a Telegram bot token, a Discord bot token, or a LINE channel secret plus channel access token
- A publicly reachable HTTPS endpoint for LINE (use a reverse proxy such as ngrok for local testing); Telegram and Discord connect outbound and run behind NAT

## Installation

### Add with go get

```bash
go get github.com/pardnchiu/go-bot
```

Import the platform packages you need from `core/`:

```go
import (
    "github.com/pardnchiu/go-bot/core/discord"
    "github.com/pardnchiu/go-bot/core/line"
    "github.com/pardnchiu/go-bot/core/telegram"
)
```

### Build from source

```bash
git clone https://github.com/pardnchiu/go-bot.git
cd go-bot
go build ./...
```

## Configuration

The library reads no environment variables; the caller passes every credential to the constructor. Complete these settings in each platform's developer console:

| Platform | Setting | Why |
|---|---|---|
| Telegram | No webhook bound (`Start` deletes an existing webhook) | Long polling and webhooks are mutually exclusive |
| Discord | Developer Portal → Bot → enable **Message Content Intent** | Without it, `Input.Text` is only populated for mentions and DMs |
| LINE | Developer Console → Messaging API → set Webhook URL to `https://<host><path>` and enable it | LINE pushes events via webhook; the default path is `/linebot/webhook` |

## Usage

### Basic: Telegram reply bot

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

### Basic: LINE webhook bot

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

### Advanced: Discord attachments and select menus

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
        // Multi-select results arrive in CallbackPicks
        if len(in.CallbackPicks) > 0 {
            return "picked: " + strings.Join(in.CallbackPicks, ", ")
        }
        // Persist each attachment and get back its local path
        for _, att := range in.Attachments {
            if _, err := bot.Save(ctx, att, "./tmp"); err != nil {
                return "save failed: " + err.Error()
            }
        }
        if in.Text == "color" {
            if _, err := bot.SendMultiSelect(ctx, in.ChannelID, in.MessageID, "Pick colors", []string{"red", "green", "blue"}); err != nil {
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

### Advanced: thinking status message

```go
bot.Reply(func(ctx context.Context, in telegram.Input) string {
    // The first call reacts to the source message and posts a status message; later calls edit it (1s debounce)
    if err := bot.SendStatus(ctx, in.ChatID, in.MessageID, "thinking...", telegram.WithStatusEmoji("👀")); err != nil {
        log.Println(err)
    }
    for i := 1; i <= 3; i++ {
        time.Sleep(time.Second)
        _ = bot.SendStatus(ctx, in.ChatID, in.MessageID, fmt.Sprintf("step %d/3", i))
    }
    // Clear the reaction and delete the status message
    if err := bot.FinishStatus(ctx, in.ChatID); err != nil {
        log.Println(err)
    }
    return "done"
})
```

## API Reference

### Shared convention

`core/telegram`, `core/discord`, and `core/line` each declare their own `Bot`, `Input`, `ReplyHandler`, and `Status`. Only the following lifecycle and reply convention is shared:

| Method | Behavior |
|---|---|
| `New(...)` | Validates required arguments and builds the client without connecting |
| `Start(ctx)` | Verifies the token through the API (returns an error on failure), then starts polling, the Gateway, or the webhook server |
| `Close()` | Cancels the internal context, closes the connection, and resets interaction and status state; idempotent |
| `Status()` | Returns the running state and bot identity |
| `Reply(handler)` | Registers `func(ctx, Input) string`; a non-empty return replies to the triggering message, and handler panics are recovered |

### Telegram (`core/telegram`)

```go
func New(token string, opts ...Option) (*Bot, error)
func WithHTTPClient(client *http.Client) Option
func WithPollTimeout(d time.Duration) Option
```

`WithPollTimeout` sets the server-side `getUpdates` timeout; the `Timeout` of the client passed to `WithHTTPClient` must exceed it, or idle long polls abort before the server responds.

| Method | Signature | Description |
|---|---|---|
| `Send` | `(ctx, chatID int64, replyTo int, text string, opts ...MessageOption) (*models.Message, error)` | Sends text; `replyTo > 0` sets the reply target |
| `Delete` | `(ctx, chatID int64, msgID int) error` | Deletes a message |
| `SendFile` | `(ctx, chatID int64, fileType FileType, path string, caption ...string) (*models.Message, error)` | `TypeDocument` / `TypeVideo` / `TypeAudio`, streamed upload |
| `SendPhoto` | `(ctx, chatID int64, paths []string, caption ...string) ([]*models.Message, error)` | One photo uses the single API; 2–10 photos send an album |
| `SendVoice` | `(ctx, chatID int64, path string, caption ...string) (*models.Message, error)` | Uploads an existing OGG/OPUS file as a voice message |
| `SendInput` | `(ctx, chatID int64, replyTo int, text string, opts ...MessageOption) (*models.Message, error)` | Asks for a reply with ForceReply |
| `SendSelect` | `(ctx, chatID int64, replyTo int, text string, items []string, opts ...MessageOption) (*models.Message, error)` | Inline keyboard single pick; result in `Input.CallbackData` |
| `SendMultiSelect` | `(ctx, chatID int64, replyTo int, text string, items []string, opts ...MessageOption) (*models.Message, error)` | ✅/⬜ toggles plus a done button; result in `Input.CallbackPicks` |
| `SendStatus` | `(ctx, chatID int64, replyTo int, text string, opts ...StatusOption) error` | One status message per chat with a 1s debounce |
| `FinishStatus` | `(ctx, chatID int64) error` | Clears the reaction and deletes the status message |
| `Save` | `(ctx, fileID, dir string) (string, error)` | Downloads a file into `dir` (20 MiB cap) and returns its path |

| Option | Description |
|---|---|
| `WithSendType(TypeMarkdown \| TypeHTML)` | `MessageOption`; sets the ParseMode, plain text when omitted |
| `WithStatusEmoji(emoji)` | `StatusOption`; reaction emoji, defaults to `🤔` |
| `WithStatusSendType(t)` | `StatusOption`; ParseMode of the status message |

`Input` fields: `ChatID`, `ChatName`, `MessageID`, `UserID`, `Username`, `Text`, `Caption`, `Photo`, `Document`, `CallbackData`, `CallbackPicks`, `Raw *models.Update`.

### Discord (`core/discord`)

```go
func New(token string) (*Bot, error)
```

| Method | Signature | Description |
|---|---|---|
| `Send` | `(ctx, channelID, replyTo, text string) (*discordgo.Message, error)` | Sends text; `replyTo != ""` sets the reply target |
| `Delete` | `(ctx, channelID, messageID string) error` | Deletes a message |
| `SendFiles` | `(ctx, channelID, replyTo string, paths []string, caption ...string) (*discordgo.Message, error)` | 1–10 attachments in one message |
| `SendVoice` | `(ctx, channelID, replyTo, path string, caption ...string) (*discordgo.Message, error)` | Uploads an OGG file as an audio attachment (not a waveform voice bubble) |
| `SendInput` | `(ctx, channelID, replyTo, prompt string) (*discordgo.Message, error)` | An answer button opens a modal; the typed value arrives in `Input.Text` |
| `SendSelect` | `(ctx, channelID, replyTo, text string, items []string) (*discordgo.Message, error)` | Dropdown single pick (1–25 items); result in `Input.Text` |
| `SendMultiSelect` | `(ctx, channelID, replyTo, text string, items []string) (*discordgo.Message, error)` | Dropdown multi pick; result in `Input.CallbackPicks` |
| `SendStatus` | `(ctx, channelID, replyTo, text string, opts ...StatusOption) error` | One status message per channel with a 1s debounce |
| `FinishStatus` | `(ctx, channelID string) error` | Clears the reaction and deletes the status message |
| `Save` | `(ctx, att *discordgo.MessageAttachment, dir string) (string, error)` | Downloads an attachment into `dir` (25 MiB cap) and returns its path |

`WithStatusEmoji(emoji)` sets the reaction emoji, defaulting to `🤔`.

`Input` fields: `ChannelID`, `ChannelName`, `GuildID`, `MessageID`, `UserID`, `Username`, `Text`, `Attachments`, `CallbackPicks`, `Raw *discordgo.MessageCreate`. For interaction events, `MessageID` is the prompt message ID, ready to pass to `Delete`.

### LINE (`core/line`)

```go
func New(secret, token, port string, opts ...Option) (*Bot, error)
func WithPath(path string) Option
```

The webhook server listens on `:<port>` with the default path `/linebot/webhook`. It returns 400 on an invalid signature and 503 when not running, and gives each event handler a 30-second timeout.

| Method | Signature | Description |
|---|---|---|
| `Send` | `(ctx, to, text string) (*linebot.BasicResponse, error)` | PushMessage to a user, group, or room |
| `Save` | `(ctx, messageID, dir string) (string, error)` | Downloads image, video, audio, or file content (50 MiB cap); the extension comes from the content type |

`Input` fields: `SourceType`, `UserID`, `Username`, `GroupID`, `RoomID`, `ReplyToken`, `MessageID`, `MessageType` (`text` / `image` / `video` / `audio` / `file`), `Text`, `FileName`, `Raw *linebot.Event`. `Username` is fetched from the profile API per message and is empty on failure. LINE offers no interaction components or status messages.

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
