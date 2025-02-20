# ConvinceMe - AI Agent Conversation Platform

## Quick Start

1. **Clone the repository**:
   ```bash
   git clone [repository-url]
   cd convinceme_back
   ```

2. **Initialize the application**:
   ```bash
   # Initialize directories and generate certificates
   convinceme init

   # Create .env file with your API key
   echo "OPENAI_API_KEY=your_key_here" > .env
   ```

3. **Start the server**:
   ```bash
   convinceme serve
   ```

4. **Access the interface**:
   - Open `https://localhost:8080` in your browser
   - Accept the self-signed certificate warning
   - Start conversing with the AI agents

## Command Line Interface

ConvinceMe uses Cobra for its CLI. Here are the available commands:

### Global Flags
```bash
--config, -c     Config file path (default: .env)
--help, -h       Help for any command
```

### Initialize Application
```bash
convinceme init
```
Sets up required directories and generates TLS certificates.

### Start Server
```bash
convinceme serve [flags]

Flags:
--port, -p       Port number (default: 8080)
--cert           Certificate file path (default: cert.pem)
--key            Key file path (default: key.pem)
```

### Examples
```bash
# Start server on custom port
convinceme serve --port 3000

# Use custom certificates
convinceme serve --cert /custom/cert.pem --key /custom/key.pem

# Use custom config file
convinceme --config prod.env serve
```

## Development

- **Hot reload mode**:
  ```bash
  make dev
  ```
  This uses Air for automatic rebuilding when files change.

- **Clean up**:
  ```bash
  make clean
  ```
  Removes generated files and certificates.

- **Run tests**:
  ```bash
  make test
  ```

## Available Make Commands

- `make deps` - Install dependencies
- `make cert` - Generate TLS certificates
- `make build` - Build the project
- `make run` - Run the server
- `make test` - Run tests
- `make clean` - Clean up generated files
- `make dev` - Run in development mode with hot reload
- `make all` - Complete setup and run (default)

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
│   ├── root.go            # Root command
│   ├── init.go            # Init command
│   └── serve.go           # Serve command
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
