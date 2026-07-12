# go-bot - Architecture

> Back to [README](../README.md)

## Overview

```mermaid
graph TB
    Client[Application] --> Telegram[telegram package]
    Client --> Discord[discord package]
    Client --> Line[line package]
    Telegram --> TGSDK[go-telegram/bot]
    Discord --> DGSDK[discordgo]
    Line --> LNSDK[LINE Bot SDK]
    Telegram --> TTS[tts package]
    Discord --> TTS
    TTS --> Gemini[Gemini API]
    TTS --> FFmpeg[ffmpeg]
```

## Module: Telegram

The Telegram adapter owns long-polling lifecycle, dispatches updates synchronously, and maps platform features to the package API.

```mermaid
graph TB
    Update[Telegram Update] --> Dispatch[dispatch]
    Dispatch --> Handler[ReplyHandler]
    Handler --> Reply[SendMessage reply]
    Dispatch --> Callback[Callback dispatch]
    Callback --> Single[Single select]
    Callback --> Multi[Multi-select state]
    Bot[Bot lifecycle] --> Poll[Long polling]
    Bot --> Status[Debounced status messages]
    Bot --> Media[File and photo helpers]
```

## Module: Discord

The Discord adapter manages a Gateway session and uses Discord-native components for interaction flows.

```mermaid
graph TB
    Gateway[Discord Gateway event] --> Dispatch[Message dispatch]
    Dispatch --> Handler[ReplyHandler]
    Handler --> Reply[Channel message reply]
    Component[Component interaction] --> Modal[Button to Modal]
    Component --> Select[String select menu]
    Modal --> Handler
    Select --> Handler
    Bot[Bot lifecycle] --> Status[Debounced channel status]
    Bot --> Media[Attachments and voice]
```

## Module: LINE

The LINE adapter runs an inbound HTTP webhook server and keeps the API surface limited to replies, push messages, and media persistence.

```mermaid
graph TB
    LINE[LINE Platform] --> Webhook[HTTP webhook]
    Webhook --> Parse[Signature-validated ParseRequest]
    Parse --> Event[Text or media event]
    Event --> Profile[Best-effort profile lookup]
    Profile --> Handler[ReplyHandler]
    Handler --> Reply[Reply token response]
    App[Application] --> Push[PushMessage]
    Event --> Save[Media Save]
```

## Module: Text to Speech

The TTS package converts a text request into audio bytes consumable by Telegram and Discord send helpers.

```mermaid
graph LR
    Text[Text and API key] --> Request[Gemini generateContent]
    Request --> PCM[PCM audio payload]
    PCM --> Encode[ffmpeg libopus encoding]
    Encode --> OGG[OGG/OPUS bytes]
    OGG --> Telegram[Telegram SendVoice]
    OGG --> Discord[Discord SendVoice]
```

## Data Flow

```mermaid
sequenceDiagram
    participant User
    participant Platform
    participant Adapter as Platform adapter
    participant Handler as ReplyHandler
    User->>Platform: message or interaction
    Platform->>Adapter: platform event
    Adapter->>Handler: normalized Input
    Handler-->>Adapter: reply string
    alt reply is non-empty
        Adapter->>Platform: reply to source message
    end
```

## State Machine

```mermaid
stateDiagram-v2
    [*] --> Created: New
    Created --> Running: Start
    Running --> Running: dispatch events
    Running --> Closing: Close
    Closing --> Closed: resources released
    Closed --> [*]
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
