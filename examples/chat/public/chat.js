let socket;
let user;
// let chat = []

// DOM elements
const connectionForm = document.getElementById('connectionForm');
const usernameForm = document.getElementById('usernameForm');
const usernameInput = document.getElementById('usernameInput');
const chatMain = document.getElementById('chatMain');
const inputBar = document.getElementById('inputBar');
const messageInput = document.getElementById('messageInput');
const sendButton = document.getElementById('sendButton');
const chatContainer = document.getElementById('chatContainer');
const statusText = document.getElementById('statusText');
const statusDot = document.getElementById('statusDot');
const leaveButton = document.getElementById('leaveButton');

usernameForm.addEventListener("submit", join);
leaveButton.addEventListener("click", function() {
    if (confirm('Are you sure you want to leave the chat?')) {
        leave();
    }
});
sendButton.addEventListener("click", sendMessage);

messageInput?.addEventListener('keypress', function(e) {
    if (e.key === 'Enter') {
        sendMessage();
    }
});

function sendMessage(){
    let message = messageInput.value.trim();
    if(!message){
        return;
    }
    const jsonString = JSON.stringify({
        messageType: "message",
        chatMessage: {
            sender: user,
            message: message,
        }
    });

    // Step 2: Encode the string to UTF-8
    const encoder = new TextEncoder();
    const uint8Array = encoder.encode(jsonString);

    // Step 3: Extract the ArrayBuffer
    const arrayBuffer = uint8Array.buffer;
    socket.send(arrayBuffer);

    messageInput.value = '';
}

function join(e){
    e.preventDefault();

    user = {
        username: document.getElementById("usernameInput").value,
    }
    const url = `ws://${window.location.host}/chat?username=${user.username}`
    console.log("connecting to: ",url)
    socket = new WebSocket(url);
    socket.binaryType = "arraybuffer";

    socket.addEventListener("error", (e) => {
        console.log("error", e)
        if(socket.readyState === 3){
            // Never connected
            console.log("Couldn't connect to server - unknown reason")
        }
        console.log(socket.readyState)
    })
    socket.addEventListener("close", () => {
        onClose()
    })
    socket.addEventListener("open", () => {
        console.log("connected");
        document.body.classList.remove("disconnected");
        document.body.classList.add("connected");
        socket.send(JSON.stringify(user));

        // Hide connection form and show chat interface
        connectionForm.classList.add('hidden');
        chatMain.classList.remove('hidden');
        chatMain.classList.add('flex');
        inputBar.classList.remove('hidden');
        leaveButton.classList.remove('hidden');

        // Update header status
        statusText.textContent = `Online as ${user.username}`;
        statusDot.classList.remove('bg-red-400');
        statusDot.classList.add('bg-green-400');

        // Add welcome message
        addSystemMessage(`Welcome to the chat, ${user.username}!`, 'info');

        // Focus on message input
        messageInput.focus();
    });
    socket.addEventListener("message", (event) => {
        const bytes = new Uint8Array(event.data).reduce((a,b)=> a+ String.fromCharCode(b),'');
        const data = JSON.parse(bytes)
        processMessage(data)
    })
}

function leave(){
    console.log("leaving")
    socket.close();
}
function onClose(){
    document.body.classList.remove("connected");
    document.body.classList.add("disconnected");
    socket = null;
    chat = [];
    console.log("left")
    // Show connection form and hide chat interface
    connectionForm.classList.remove('hidden');
    chatMain.classList.add('hidden');
    chatMain.classList.remove('flex');
    inputBar.classList.add('hidden');
    leaveButton.classList.add('hidden');

    // Update header status
    statusText.textContent = 'Disconnected';
    statusDot.classList.remove('bg-green-400');
    statusDot.classList.add('bg-red-400');

    // Clear chat messages
    chatContainer.innerHTML = '';

    // Clear and focus username input
    usernameInput.value = '';
    usernameInput.focus();
}


function processMessage(msg){
    console.log("message", msg)
    console.log(msg.messageType, msg.chatMessage)
    switch (msg.messageType) {
        case "user.joined":
            if(msg.chatMessage.sender.username === user.username){
                user.id = msg.id;
                return
            }
            addSystemMessage(msg.chatMessage.message, 'join', new Date(msg.chatMessage.tsp))
            break;
        case "user.left":
            addSystemMessage(msg.chatMessage.message, 'leave', new Date(msg.chatMessage.tsp))
            break;
        case "message":
            addMessage(msg.chatMessage);
            break;
        default:
            console.log("unknown message type", msg)
            break;
    }
}

// Function to add a system message
function addSystemMessage(text, type = 'info', tsp = new Date()) {
    const messageDiv = document.createElement('div');
    messageDiv.className = 'flex justify-center my-2';

    const timeString = tsp.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});

    let bgColor = 'bg-gray-100';
    let textColor = 'text-gray-600';
    let icon = '';

    switch(type) {
        case 'join':
            bgColor = 'bg-green-50';
            textColor = 'text-green-700';
            icon = '<svg class="w-3 h-3 mr-1 inline" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path></svg>';
            break;
        case 'leave':
            bgColor = 'bg-red-50';
            textColor = 'text-red-700';
            icon = '<svg class="w-3 h-3 mr-1 inline" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path></svg>';
            break;
        case 'info':
        default:
            bgColor = 'bg-blue-50';
            textColor = 'text-blue-700';
            icon = '<svg class="w-3 h-3 mr-1 inline" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"></path></svg>';
            break;
    }

    messageDiv.innerHTML = `
                <div class="${bgColor} ${textColor} text-sm px-3 py-2 rounded-full border">
                    ${icon}${text} • ${timeString}
                </div>
            `;

    chatContainer.appendChild(messageDiv);
    chatContainer.scrollTop = chatContainer.scrollHeight;
}

function addMessage(msg){
    //chat.push(msg)
    //renderChat();

    const isUser = msg.sender.username === user.username
    const {sender, message: text, tsp} = msg;

    const messageDiv = document.createElement('div');
    messageDiv.className = `flex justify-${isUser ? 'end' : 'start'}`;

    const time = new Date(tsp);
    const timeString = time.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});
    const displayName = isUser ? 'You' : (sender.username || 'User');

    messageDiv.innerHTML = `
                <div class="max-w-xs lg:max-w-md ${isUser ? 'bg-blue-600 text-white' : 'bg-gray-200'} rounded-lg p-3">
                    ${!isUser ? `<p class="text-xs font-medium ${isUser ? 'text-blue-100' : 'text-gray-500'} mb-1">${displayName}</p>` : ''}
                    <p class="text-sm ${isUser ? 'text-white' : 'text-gray-800'}">${text}</p>
                    <span class="text-xs ${isUser ? 'text-blue-100' : 'text-gray-500'} mt-1 block">${timeString}</span>
                </div>
            `;

    chatContainer.appendChild(messageDiv);
    chatContainer.scrollTop = chatContainer.scrollHeight;
}