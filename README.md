# QR Code Relay Service

This is a simple Go web service to relay QR code content (strings) from a publisher to subscribers via WebSockets.

## Features

- **Publish**: Send content to a specific room via HTTP GET.
- **Subscribe**: Receive content updates for a specific room via WebSocket.
- **Heartbeat**: WebSocket connections are kept alive with Ping/Pong messages.

## Usage

### 1. Start the Server

```bash
go run main.go
```

The server listens on port `8080`.

### 2. Subscribe (Client)

Connect to the WebSocket endpoint for a specific room (e.g., `room1`).

URL: `ws://localhost:8080/ws/room1`

You can use a WebSocket client or a browser console:

```javascript
var ws = new WebSocket("ws://localhost:8080/ws/room1");
ws.onmessage = function(event) {
    console.log("Received QR Code Content:", event.data);
};
ws.onopen = function() {
    console.log("Connected to room1");
};
```

### 3. Publish (Publisher)

Send a GET request to the room URL with the `content` parameter.

URL: `http://localhost:8080/room1?content=MY_QR_CODE_DATA`

Example using curl:

```bash
curl "http://localhost:8080/room1?content=HelloFromQR"
```

All clients connected to `room1` will receive `HelloFromQR`.
