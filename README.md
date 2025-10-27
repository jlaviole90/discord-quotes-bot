# Discord Quotes Bot

A feature-rich Discord bot that combines message archiving with local AI interaction. Built with Go and powered by Ollama for running large language models locally.

## Features

### 1. Message Quoting System
- **React to Quote**: Users can react to any message with the 📸 (camera with flash) emoji
- **Automatic Crossposting**: Quoted messages are automatically crossposted to a dedicated `#quotes` channel
- **Identity Preservation**: Uses Discord webhooks to preserve the original author's username, global name, and avatar
- **Attachment Support**: Handles image attachments in quoted messages as embeds
- **Duplicate Prevention**: Built-in caching prevents the same message from being quoted multiple times
- **Bot Message Filtering**: Prevents quoting of bot messages (except its own)

### 2. Local AI Integration
- **Conversational Interface**: Responds to mentions with a configurable prefix (default: "georgibot")
- **Context Awareness**: Maintains conversation context for 15 minutes per user
- **Reply Detection**: Understands when messages are replies and includes reply context in prompts
- **Custom System Prompts**: Supports per-user and global system prompts via environment variables
- **Character Limit**: Enforces a 1000 character limit on prompts to ensure reasonable inference times
- **Typing Indicators**: Shows typing status every 5 seconds during inference
- **Timeout Handling**: 5-minute timeout for LLM responses to prevent hanging

## Architecture

### Project Structure
```
discord-quotes-bot/
├── main.go              # Application entry point and Discord session setup
├── quote.go             # Quote functionality implementation
├── inference.go         # LLM inference handling and Ollama integration
├── Dockerfile           # Multi-stage Docker build configuration
├── compose.yaml         # Docker Compose orchestration
├── init-model.sh        # Bot initialization script (waits for Ollama)
├── ollama-init.sh       # Ollama service initialization and model setup
├── go.mod              # Go module dependencies
└── go.sum              # Go module checksums
```

### Key Components

#### Quote System (`quote.go`)
- Event-driven reaction handler using `MessageReactionAdd` events
- Webhook-based message replication for authentic user impersonation
- Channel state caching for duplicate detection
- Error handling with user-friendly feedback messages

#### AI Inference (`inference.go`)
- HTTP client integration with Ollama's `/api/generate` endpoint
- Thread-safe conversation context storage using mutex-protected maps
- Context timeout mechanism to manage memory usage
- Prompt enrichment with message metadata (author, reply context)
- Automatic context cleanup after 15 minutes of inactivity

#### Containerization
- **Multi-stage Docker Build**: Separates build and runtime environments for minimal image size
- **Service Dependencies**: Ensures Ollama is ready before starting the bot
- **Volume Mounts**: Persistent storage for Ollama models and data
- **Health Checks**: Startup scripts verify service availability

## Prerequisites

- **Docker** and **Docker Compose** (recommended) OR
- **Go 1.23.5+** for local development
- **Discord Bot Token** (see [Setup](#discord-bot-setup))
- **LLM Model**: A Hermes-compatible GGUF model file (e.g., `hermes-llama3.2.gguf`)

## Discord Bot Setup

1. Go to the [Discord Developer Portal](https://discord.com/developers/applications)
2. Create a new application
3. Navigate to the "Bot" section and create a bot
4. Enable the following **Privileged Gateway Intents**:
   - Message Content Intent
   - Server Members Intent (optional, for enhanced features)
5. Copy the bot token for configuration
6. Generate an OAuth2 URL with the following permissions:
   - **Bot Permissions**: 
     - Send Messages
     - Manage Messages
     - Manage Webhooks
     - Add Reactions
     - Read Message History
   - **Scopes**: `bot`, `applications.commands`
7. Use the generated URL to invite the bot to your Discord server
8. Create a text channel named `quotes` (case-insensitive) in your server

## Installation & Setup

### Using Docker Compose (Recommended)

1. **Clone the repository**:
```bash
git clone <repository-url>
cd discord-quotes-bot
```

2. **Prepare your LLM model**:
   - Place your GGUF model file at: `~/models/hermes/hermes-llama3.2.gguf`
   - Or modify the path in `ollama-init.sh` and `compose.yaml`

3. **Create environment configuration**:
```bash
cat > .env << 'EOF'
# Required: Discord bot token from Developer Portal
DISCORD_TOKEN=your_discord_bot_token_here

# Optional: Custom bot mention prefix (default: "georgibot")
MENTION_PREFIX=georgibot

# Optional: Default system prompt for AI responses
SYSTEM_PROMPT=You are georgibot, an AI bot in a Discord server where it is your job to maintain records of quoted messages. You love Bulgaria and its vibrant history, and love talking about it any chance you get. You are friendly and helpful to all requests.

# Optional: User-specific system prompts (username must be UPPERCASE)
# SYSTEM_PROMPT_JOHNDOE=You are a helpful assistant specifically for John.

# Optional: Ollama host (default: http://localhost:11434)
# OLLAMA_HOST=http://ollama:11434
EOF
```

4. **Build and start the services**:
```bash
docker compose up -d
```

5. **Monitor logs**:
```bash
docker compose logs -f discord-quotes-bot
docker compose logs -f ollama
```

### Local Development Setup

1. **Install Go dependencies**:
```bash
go mod download
```

2. **Set up Ollama locally**:
```bash
# Install Ollama (macOS/Linux)
curl -fsSL https://ollama.com/install.sh | sh

# Create the model
ollama create hermes -f <path-to-your-modelfile>
```

3. **Set environment variables**:
```bash
export DISCORD_TOKEN=your_token_here
export MENTION_PREFIX=georgibot
export OLLAMA_HOST=http://localhost:11434
```

4. **Run the bot**:
```bash
go run .
```

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DISCORD_TOKEN` | **Yes** | - | Your Discord bot token |
| `MENTION_PREFIX` | No | `georgibot` | Prefix for invoking the AI (case-insensitive) |
| `SYSTEM_PROMPT` | No | Default Bulgaria-themed prompt | Base system prompt for all AI interactions |
| `SYSTEM_PROMPT_<USERNAME>` | No | - | User-specific system prompt (USERNAME in UPPERCASE) |
| `OLLAMA_HOST` | No | `http://localhost:11434` | URL of the Ollama service |

### Model Configuration

The bot expects an Ollama model named `hermes`. You can customize model parameters in `ollama-init.sh`:

```bash
TEMPLATE """<|im_start|>system
{{ .System }}<|im_end|>
<|im_start|>user
{{ .Prompt }}<|im_end|>
<|im_start|>assistant
"""

PARAMETER stop "<|im_start|>"
PARAMETER stop "<|im_end|>"
PARAMETER temperature 0.7      # Creativity (0.0-1.0)
PARAMETER top_p 0.8             # Nucleus sampling
PARAMETER top_k 20              # Token selection diversity
PARAMETER repeat_penalty 1.1    # Repetition reduction
```

## Usage

### Quoting Messages

1. Find a message you want to save to the quotes channel
2. React to it with the 📸 emoji (`:camera_with_flash:`)
3. The bot will automatically crosspost it to `#quotes` with the original author's identity
4. Attachments (images) will be embedded in the quoted message

**Note**: Each message can only be quoted once to prevent spam.

### Interacting with the AI

1. **Mention the bot** with its prefix:
```
georgibot, what's the weather like?
```

2. **Alternative trigger** (easter egg):
```
Tell me about bulgaria
```

3. **Reply to the bot** to continue a conversation with context:
```
User: georgibot, what's your favorite color?
Bot: I don't have personal preferences...
User: [replies to bot] What about blue?
```

**Context Behavior**:
- Conversation context persists for 15 minutes per user
- After 15 minutes of inactivity, context is cleared
- Context is user-specific and isolated

**Character Limit**:
- Prompts exceeding 1000 characters will be rejected

### Custom Prefix Example

If you set `MENTION_PREFIX=alfred`:
```
alfred, how are you today?
```

## Docker Compose Services

### Ollama Service
- **Image**: `ollama/ollama:latest`
- **Platform**: `linux/arm64` (adjust for your architecture)
- **Ports**: `11434:11434`
- **Volumes**:
  - `~/models:/models` - Model files directory
  - `ollama_data:/root/.ollama` - Persistent Ollama data
- **Function**: Hosts the LLM inference engine

### Discord Bot Service
- **Build**: From local `Dockerfile`
- **Depends On**: Ollama service must start first
- **Ports**: `3000:3000` (currently unused, for future expansion)
- **Environment**: Loads from `.env` file
- **Function**: Discord bot application

## Development

### Building Locally

```bash
# Build the binary
go build -o discord-quotes-bot .

# Run tests (when implemented)
go test ./...

# Build Docker image
docker build -t discord-quotes-bot .
```

### Code Structure

#### `main.go`
- Initializes Discord session with bot token
- Registers event handlers for Quote and Inference
- Implements graceful shutdown on interrupt signal

#### `quote.go`
- `Quote()`: Main reaction handler
- `getQuotesChannel()`: Locates the quotes channel by name
- `enableChannelCache()`: Configures state caching for duplicate detection

#### `inference.go`
- `Inference()`: Main message handler for AI interactions
- `isProperlyMentioned()`: Checks if message properly invokes the bot
- `getOllamaRequestData()`: Prepares and sanitizes prompts
- `enrichPrompt()`: Adds context metadata to user prompts
- `getSystemPrompt()`: Retrieves appropriate system prompt
- Context management with mutex-protected maps

## Troubleshooting

### Bot is not responding
1. **Check bot token**: Verify `DISCORD_TOKEN` in `.env`
2. **Check permissions**: Ensure bot has required Discord permissions
3. **Check intents**: Verify "Message Content Intent" is enabled
4. **View logs**: `docker compose logs -f discord-quotes-bot`

### Quotes not working
1. **Channel exists**: Ensure a channel named `quotes` exists (case-insensitive)
2. **Webhook permissions**: Bot needs "Manage Webhooks" permission
3. **Message history**: Bot needs "Read Message History" permission
4. **Check reaction**: Use the actual 📸 emoji or `:camera_with_flash:`

### AI not responding
1. **Ollama status**: Check if Ollama is running
   ```bash
   docker compose ps ollama
   docker compose logs ollama
   ```
2. **Model loaded**: Verify `hermes` model exists
   ```bash
   docker compose exec ollama ollama list
   ```
3. **GGUF file**: Ensure GGUF file exists at the expected path
4. **Proper invocation**: Use the correct prefix (default: `georgibot`)
5. **Character limit**: Ensure prompt is under 1000 characters

### Model fails to load
1. **Check GGUF path**: Verify file exists at `~/models/hermes/hermes-llama3.2.gguf`
2. **File permissions**: Ensure Docker can read the model file
3. **Disk space**: Verify sufficient disk space for model loading
4. **View Ollama logs**: `docker compose logs ollama`

### Context not persisting
- Context automatically clears after 15 minutes of inactivity
- Each user has isolated context (cross-user conversations won't share context)
- Bot restart clears all context

## Advanced Configuration

### Custom Model
To use a different model or GGUF file:

1. Update `ollama-init.sh`:
```bash
# Change the GGUF file path
FROM /models/your-custom-model/model.gguf
```

2. Update `inference.go`:
```go
// Change the model name in the request
Model: "your-model-name",
```

### Per-User System Prompts
Create specialized bot behaviors for specific users:

```bash
# In .env file
SYSTEM_PROMPT_ALICE=You are a coding assistant specialized in Python.
SYSTEM_PROMPT_BOB=You are a creative writing assistant.
```

**Note**: Usernames must be in UPPERCASE in the environment variable name.

### Adjusting Context Timeout
Modify `inference.go`:

```go
var (
    contextTimeout = time.Minute * 15  // Change duration here
)
```

## Performance Considerations

- **Model Size**: Larger models provide better responses but require more RAM and CPU
- **Context Length**: Longer conversations increase inference time
- **Temperature**: Lower temperature (0.3-0.5) = more focused, higher (0.7-1.0) = more creative
- **Concurrent Requests**: Single-instance bot processes requests sequentially per user

## Security Notes

- **Token Security**: Never commit `.env` file or expose `DISCORD_TOKEN`
- **Webhook Cleanup**: Bot automatically deletes webhooks after use
- **Rate Limiting**: Discord enforces rate limits; bot handles gracefully
- **Model Access**: Models run locally, ensuring data privacy

## Future Enhancements

Potential features for future development:
- [ ] Database integration for persistent quote storage
- [ ] Quote search functionality
- [ ] Configurable quote channels (multiple quote channels)
- [ ] Slash commands for modern Discord UX
- [ ] Web dashboard for bot statistics
- [ ] Quote voting system
- [ ] Message thread support for AI conversations
- [ ] Streaming responses for faster perceived latency

## Contributing

Contributions are welcome! Areas for improvement:
- Unit tests for quote and inference handlers
- Integration tests with mock Discord API
- Documentation improvements
- Performance optimizations
- Additional features

## License

This project is open source. Please check the repository for license details.

## Acknowledgments

- [discordgo](https://github.com/bwmarrin/discordgo) - Discord API wrapper for Go
- [Ollama](https://ollama.com/) - Local LLM runtime
- [Hermes](https://huggingface.co/NousResearch/Hermes-2-Pro-Llama-3-8B) - Base model family

## Support

For issues, questions, or contributions:
1. Check existing issues in the repository
2. Review troubleshooting section
3. Check Docker Compose logs for error details
4. Create a new issue with logs and reproduction steps

