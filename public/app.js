// Elements
const connectionStatus = document.getElementById('connectionStatus');
const currentRoomDisplay = document.getElementById('currentRoomDisplay');
const receivedContent = document.getElementById('receivedContent');
const historyList = document.getElementById('historyList');
const qrContainer = document.getElementById('qrcode');

let socket = null;

function connectToRoom(roomId) {
    if (socket) {
        socket.close();
    }

    // Update UI
    currentRoomDisplay.textContent = roomId;
    connectionStatus.textContent = 'Connecting...';
    connectionStatus.className = 'status-indicator';

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/ws/${roomId}`;

    try {
        socket = new WebSocket(wsUrl);

        socket.onopen = () => {
            connectionStatus.textContent = 'Connected';
            connectionStatus.classList.add('connected');
            receivedContent.textContent = 'Waiting for content...';
        };

        socket.onclose = (event) => {
            connectionStatus.textContent = 'Disconnected';
            connectionStatus.classList.remove('connected');
            socket = null;
        };

        socket.onerror = (error) => {
            console.error('WebSocket Error:', error);
            connectionStatus.textContent = 'Connection Error';
        };

        socket.onmessage = (event) => {
            const content = event.data;
            const isUrl = content.startsWith('http://') || content.startsWith('https://');

            if (isUrl) {
                receivedContent.innerHTML = '';
                const a = document.createElement('a');
                a.href = content;
                a.target = '_blank';
                a.textContent = content;
                receivedContent.appendChild(a);
            } else {
                receivedContent.textContent = content;
            }
            
            // Generate QR Code
            qrContainer.innerHTML = ''; // Clear previous
            try {
                new QRCode(qrContainer, {
                    text: content,
                    width: 256,
                    height: 256,
                    colorDark : "#000000",
                    colorLight : "#ffffff",
                    correctLevel : QRCode.CorrectLevel.H
                });
            } catch (e) {
                console.error("Error generating QR code:", e);
                qrContainer.textContent = "Error generating QR code";
            }

            // Add to history
            const li = document.createElement('li');
            const time = new Date().toLocaleTimeString();
            const timeSpan = document.createElement('span');
            timeSpan.textContent = `[${time}] `;
            li.appendChild(timeSpan);

            if (isUrl) {
                const a = document.createElement('a');
                a.href = content;
                a.target = '_blank';
                a.textContent = content;
                li.appendChild(a);
            } else {
                li.appendChild(document.createTextNode(content));
            }
            historyList.insertBefore(li, historyList.firstChild);
        };

    } catch (e) {
        console.error(e);
        connectionStatus.textContent = 'Failed to create connection';
    }
}

function handleHashChange() {
    const hash = window.location.hash.substring(1); // Remove '#'
    if (hash) {
        connectToRoom(hash);
    } else {
        currentRoomDisplay.textContent = 'None';
        connectionStatus.textContent = 'No room specified in URL hash (e.g. #room1)';
        connectionStatus.classList.remove('connected');
        if (socket) {
            socket.close();
        }
    }
}

// Listen for hash changes
window.addEventListener('hashchange', handleHashChange);

// Initial check
handleHashChange();
