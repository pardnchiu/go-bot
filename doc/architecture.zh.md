# go-bot - 架構

> 返回 [README](./README.zh.md)

## 概覽

```mermaid
graph TB
    Client[應用程式] --> Telegram[telegram 套件]
    Client --> Discord[discord 套件]
    Client --> Line[line 套件]
    Telegram --> TGSDK[go-telegram/bot]
    Discord --> DGSDK[discordgo]
    Line --> LNSDK[LINE Bot SDK]
    Telegram --> TTS[tts 套件]
    Discord --> TTS
    TTS --> Gemini[Gemini API]
    TTS --> FFmpeg[ffmpeg]
```

## 模組：Telegram

Telegram adapter 負責 long-polling 生命週期、同步派送 update，並將平台功能對應至套件 API。

```mermaid
graph TB
    Update[Telegram Update] --> Dispatch[派送器]
    Dispatch --> Handler[ReplyHandler]
    Handler --> Reply[SendMessage 回覆]
    Dispatch --> Callback[Callback 派送]
    Callback --> Single[單選]
    Callback --> Multi[多選狀態]
    Bot[Bot 生命週期] --> Poll[Long polling]
    Bot --> Status[去彈跳狀態訊息]
    Bot --> Media[檔案與圖片 helper]
```

## 模組：Discord

Discord adapter 管理 Gateway session，並使用 Discord 原生 component 完成互動流程。

```mermaid
graph TB
    Gateway[Discord Gateway 事件] --> Dispatch[訊息派送]
    Dispatch --> Handler[ReplyHandler]
    Handler --> Reply[Channel 訊息回覆]
    Component[Component 互動] --> Modal[按鈕開啟 Modal]
    Component --> Select[String select menu]
    Modal --> Handler
    Select --> Handler
    Bot[Bot 生命週期] --> Status[去彈跳 channel 狀態]
    Bot --> Media[附件與語音]
```

## 模組：LINE

LINE adapter 執行 inbound HTTP webhook server，API 範圍限定於回覆、PushMessage 與媒體儲存。

```mermaid
graph TB
    LINE[LINE 平台] --> Webhook[HTTP webhook]
    Webhook --> Parse[驗證簽章的 ParseRequest]
    Parse --> Event[文字或媒體事件]
    Event --> Profile[盡力取得使用者資料]
    Profile --> Handler[ReplyHandler]
    Handler --> Reply[Reply token 回覆]
    App[應用程式] --> Push[PushMessage]
    Event --> Save[媒體儲存]
```

## 模組：文字轉語音

TTS 套件會將文字請求轉為可供 Telegram 與 Discord 傳送 helper 使用的音訊 bytes。

```mermaid
graph LR
    Text[文字與 API key] --> Request[Gemini generateContent]
    Request --> PCM[PCM 音訊 payload]
    PCM --> Encode[ffmpeg libopus 編碼]
    Encode --> OGG[OGG/OPUS bytes]
    OGG --> Telegram[Telegram SendVoice]
    OGG --> Discord[Discord SendVoice]
```

## 資料流

```mermaid
sequenceDiagram
    participant User as 使用者
    participant Platform as 平台
    participant Adapter as 平台 adapter
    participant Handler as ReplyHandler
    User->>Platform: 訊息或互動
    Platform->>Adapter: 平台事件
    Adapter->>Handler: 正規化 Input
    Handler-->>Adapter: 回覆字串
    alt 回覆非空
        Adapter->>Platform: 回覆來源訊息
    end
```

## 狀態機

```mermaid
stateDiagram-v2
    [*] --> Created: New
    Created --> Running: Start
    Running --> Running: 派送事件
    Running --> Closing: Close
    Closing --> Closed: 釋放資源
    Closed --> [*]
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
