# ConvinceMe - AI Agent Conversation Platform

## Overview
ConvinceMe is a sophisticated Go-based platform that enables dynamic, real-time conversations between AI agents and human participants. The system features advanced natural language processing, speech synthesis, and interactive dialogue management, creating engaging and context-aware conversations.

## Features

### AI Agent Capabilities
- **Dynamic Conversation Flow**: Agents maintain context-aware discussions and respond naturally to both player input and each other
- **Character Consistency**: Each agent maintains its unique personality and role throughout conversations
- **Memory Management**: Agents track conversation history and reference previous topics appropriately
- **Speech Synthesis**: Real-time text-to-speech conversion for natural audio responses

### Technical Features
- **WebSocket Communication**: Real-time bidirectional communication between clients and server
- **Audio Streaming**: Efficient handling of synthesized speech audio
- **Concurrent Processing**: Parallel processing of agent responses with proper synchronization
- **Context Management**: Sophisticated tracking of conversation context and agent states

## System Requirements

### Prerequisites
- Go 1.19 or later
- OpenSSL (for TLS certificate generation)
- OpenAI API key
- Modern web browser with WebSocket support

### Dependencies
- Gin Web Framework
- Gorilla WebSocket
- OpenAI Go Client
- LangChain Go

## Installation

1. **Clone the Repository**:
   ```bash
   git clone [repository-url]
   cd convinceme_back
   ```

2. **Install Dependencies**:
   ```bash
   go mod download
   go mod tidy
   ```

3. **Configure Environment**:
   Create a `.env` file in the project root:
   ```env
   OPENAI_API_KEY=your_openai_api_key
   ```

4. **Generate TLS Certificates**:
   ```bash
   openssl genpkey -algorithm RSA -out key.pem
   openssl req -new -key key.pem -out cert.csr
   openssl req -x509 -key key.pem -in cert.csr -out cert.pem -days 365
   ```

## Usage

### Starting the Server
1. Run the server:
   ```bash
   go run cmd/main.go
   ```
2. The server will start on the default port (check console output)

### Accessing the Interface
- Open `test.html` in your web browser
- Allow microphone access for voice input (optional)
- Start conversing with the AI agents

### API Endpoints

#### WebSocket
- `/ws/conversation`: Main WebSocket endpoint for real-time communication

#### HTTP
- `/api/conversation/start`: Initialize a new conversation
- `/api/audio/:id`: Stream synthesized audio responses
- `/api/stt`: Speech-to-text processing endpoint

## Architecture

### Components
- **Server**: Core Go server handling WebSocket connections and HTTP requests
- **Agent**: AI agent implementation with conversation management
- **Audio**: Speech synthesis and processing
- **Player**: Player input handling and processing
- **Conversation**: Conversation flow and context management

### Data Flow
1. Client connects via WebSocket
2. Player sends text/voice input
3. Server processes input and routes to agents
4. Agents generate responses with context awareness
5. Responses are converted to audio and streamed
6. Client receives text and audio responses

## Development

### Project Structure
```
convinceme_back/
├── cmd/
│   └── main.go           # Application entry point
├── internal/
│   ├── agent/            # AI agent implementation
│   ├── audio/            # Audio processing
│   ├── conversation/     # Conversation management
│   ├── player/           # Player input handling
│   ├── server/           # Server implementation
│   └── types/            # Shared types and interfaces
├── test.html             # Web interface
└── .env                  # Environment configuration
```

### Adding New Features
1. Follow Go best practices and project structure
2. Maintain concurrent safety in WebSocket handlers
3. Update tests for new functionality
4. Document API changes

## Troubleshooting

### Common Issues
- **WebSocket Connection Failed**: Check browser console for errors and ensure server is running
- **Audio Not Playing**: Verify browser audio permissions and check audio endpoint responses
- **TLS Certificate Errors**: Ensure certificates are properly generated and placed in root directory
- **API Key Issues**: Verify `.env` file configuration and key validity

### Logging
- Check server logs for detailed error messages
- WebSocket communication logs are available in browser console
- Agent response generation logs provide insight into conversation flow
