Here is the updated `README.md` file with the added line about `test.html`:

```markdown
# Project Description
This project is a Go application that facilitates live conversations between AI agents, simulating interactions in a dynamic environment.

## Getting Started
To set up and run the project locally, follow these steps:

### Prerequisites
- Go 1.19 or later
- OpenSSL (for generating self-signed certificates)
- Environment variables set up, including `OPENAI_API_KEY` in your `.env` file

### Installation
1. **Clone the repository**:

2. **Install dependencies**:
   go mod tidy

3. **Create a `.env` file**:
   Create a `.env` file in the root directory of the project and add your OpenAI API key:
   ```env
   OPENAI_API_KEY=your_openai_api_key
   ```

4. **Generate self-signed certificates**:
   Use OpenSSL to generate self-signed certificates for HTTPS:
   ```sh
   openssl genpkey -algorithm RSA -out key.pem
   openssl req -new -key key.pem -out cert.csr
   openssl req -x509 -key key.pem -in cert.csr -out cert.pem -days 365
   ```

### Running the Application
1. **Start the server**:
   ```sh
   go run cmd/main.go
   ```

2. **Open the chat interface**:
   Open `test.html` in your web browser to interact with the AI agents.

### API Endpoints
- **WebSocket Endpoint**: `/ws/conversation`
- **Audio Stream Endpoint**: `/api/audio/:id`
- **Start Conversation Endpoint**: `/api/conversation/start`

### Troubleshooting
- **TLS Handshake Error**: Ensure that the `cert.pem` and `key.pem` files are correctly generated and placed in the root directory.
- **Environment Variables**: Make sure the `.env` file is correctly set up with the `OPENAI_API_KEY`.

```