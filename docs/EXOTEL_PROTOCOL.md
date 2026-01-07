# Exotel WebSocket Protocol

This document describes the WebSocket protocol used for real-time voice communication between SIP servers (like blayzen-sip) and Blayzen voice agents.

## Overview

The protocol uses JSON messages over WebSocket for signaling and base64-encoded audio for media. Audio is transmitted as μ-law (G.711) encoded at 8kHz.

## Connection Flow

```
SIP Server                    Blayzen Agent
    |                              |
    |------- WebSocket Connect --->|
    |                              |
    |<------ "connected" ---------|
    |                              |
    |------- "start" ------------->|
    |                              |
    |<====== "media" (audio) =====>|  (bidirectional)
    |                              |
    |------- "stop" -------------->|
    |                              |
    |------- WebSocket Close ----->|
```

## Message Types

### Server to Client Events

#### `connected`

Sent when the WebSocket connection is established.

```json
{
  "event": "connected"
}
```

#### `start`

Sent when a call begins. Contains call metadata.

```json
{
  "event": "start",
  "streamSid": "unique-stream-id",
  "callSid": "unique-call-id",
  "accountSid": "account-id",
  "from": "+14155551234",
  "to": "+14155555678",
  "customData": {
    "routing_rule": "support",
    "priority": "high"
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `event` | string | Always `"start"` |
| `streamSid` | string | Unique identifier for this media stream |
| `callSid` | string | Unique identifier for this call |
| `accountSid` | string | Account/tenant identifier |
| `from` | string | Caller's phone number or SIP URI |
| `to` | string | Called number or SIP URI |
| `customData` | object | Optional custom metadata from SIP headers |

#### `media`

Contains audio data (inbound from caller to agent).

```json
{
  "event": "media",
  "streamSid": "unique-stream-id",
  "media": {
    "payload": "base64-encoded-audio-data",
    "chunk": 1,
    "timestamp": 1704567890123
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `event` | string | Always `"media"` |
| `streamSid` | string | Stream identifier |
| `media.payload` | string | Base64 encoded μ-law audio (8kHz, mono) |
| `media.chunk` | integer | Sequential chunk number (starts at 1) |
| `media.timestamp` | integer | Timestamp in milliseconds |

#### `stop`

Sent when the call ends.

```json
{
  "event": "stop",
  "streamSid": "unique-stream-id"
}
```

#### `dtmf`

Sent when a DTMF digit is detected.

```json
{
  "event": "dtmf",
  "dtmf": "5"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `event` | string | Always `"dtmf"` |
| `dtmf` | string | DTMF digit: `0-9`, `*`, `#` |

#### `mark`

Used for synchronization markers.

```json
{
  "event": "mark",
  "name": "greeting-complete"
}
```

### Client to Server Events

#### `media`

Send audio back to the caller (agent's response).

```json
{
  "event": "media",
  "media": "base64-encoded-audio-data",
  "timestamp": 1704567890123,
  "chunk": 1
}
```

| Field | Type | Description |
|-------|------|-------------|
| `event` | string | Always `"media"` |
| `media` | string | Base64 encoded μ-law audio (8kHz, mono) |
| `timestamp` | integer | Optional timestamp |
| `chunk` | integer | Optional chunk number |

#### `clear`

Request to clear the audio playback buffer (for barge-in/interruption).

```json
{
  "event": "clear"
}
```

#### `mark`

Send a marker for synchronization.

```json
{
  "event": "mark",
  "name": "response-start"
}
```

## Audio Format

- **Codec**: μ-law (G.711 PCMU)
- **Sample Rate**: 8000 Hz
- **Channels**: Mono (1 channel)
- **Bit Depth**: 8 bits per sample
- **Encoding**: Base64

### Audio Chunk Size

Typical chunk size is 20ms of audio:
- 20ms × 8000 samples/sec = 160 samples
- 160 samples × 1 byte/sample = 160 bytes raw
- Base64 encoded: ~214 characters

## Error Handling

If an error occurs, the WebSocket connection may be closed with an appropriate close code:

| Code | Reason |
|------|--------|
| 1000 | Normal closure |
| 1001 | Going away |
| 1008 | Policy violation |
| 1011 | Internal error |

## Example Implementation

### Receiving Audio (Go)

```go
import "github.com/shiv6146/blayzen/pkg/protocol/exotel"

// Parse incoming message
msg, err := exotel.ParseMessage(data)
if err != nil {
    log.Printf("Parse error: %v", err)
    return
}

switch m := msg.(type) {
case *exotel.StartMessage:
    log.Printf("Call started: %s -> %s", m.From, m.To)
    
case *exotel.MediaMessage:
    audio, _ := m.DecodeAudio()
    // Process audio...
    
case *exotel.StopMessage:
    log.Println("Call ended")
}
```

### Sending Audio (Go)

```go
import "github.com/shiv6146/blayzen/pkg/protocol/exotel"

// Create media message
msg := exotel.NewMediaMessage("stream-123", audioBytes, chunkNum, timestamp)

// Marshal and send
data, _ := json.Marshal(msg)
conn.WriteMessage(websocket.TextMessage, data)
```

## Compatibility

This protocol is compatible with:
- Exotel Voice API
- Twilio Media Streams (with minor adaptations)
- blayzen-sip

## See Also

- [Blayzen Framework](https://github.com/shiv6146/blayzen)
- [blayzen-sip](https://github.com/shiv6146/blayzen-sip)
