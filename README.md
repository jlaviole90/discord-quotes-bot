# Discord Quotes Bot

A Discord chatbot powered by local LLM inference through Ollama. Designed to run on low-power hardware like a Raspberry Pi.

## Features

- **Conversational AI**: Responds to mentions or configurable trigger words using a locally-hosted LLM via Ollama
- **Streaming Responses**: Tokens are streamed from Ollama and progressively edited into a Discord message, so users see output as it's generated
- **Trigger Words**: Define a comma-separated list of words in `TRIGGER_WORDS` that will activate the bot when found anywhere in a message
- **Per-User Context**: Maintains conversation context for 15 minutes per user (generate API only), capped at 4096 tokens
- **Reply Awareness**: Understands when messages are replies and includes the referenced message in the prompt
- **Custom System Prompts**: Supports a global system prompt and per-user overrides via environment variables
- **Dual Model Support**: Automatically detects a fine-tuned `impersonate` model (chat API) and falls back to `hermes` (generate API)
- **Concurrency Control**: Limits inference to one request at a time to avoid memory thrashing on constrained hardware
- **Prompt Length Limit**: Rejects prompts over 1000 characters

## Project Structure

```
discord-quotes-bot/
├── main.go              # Entry point and Discord session setup
├── inference.go         # LLM inference, streaming, trigger words, context management
├── Dockerfile           # Multi-stage Docker build
├── compose.yaml         # Docker Compose orchestration (Ollama + bot)
├── init-model.sh        # Bot startup script (waits for Ollama readiness)
├── ollama-init.sh       # Ollama initialization and model creation
├── go.mod               # Go module dependencies
└── go.sum               # Go module checksums
```

## Prerequisites

- **Docker** and **Docker Compose** (recommended), or **Go 1.23.5+** for local development
- **Discord Bot Token** ([Developer Portal](https://discord.com/developers/applications))
- **LLM Model**: A Hermes-compatible GGUF file (e.g., `hermes-llama3.2.gguf`)

## Discord Bot Setup

1. Create an application in the [Discord Developer Portal](https://discord.com/developers/applications)
2. Create a bot under the application and enable **Message Content Intent**
3. Copy the bot token
4. Generate an OAuth2 invite URL with scopes `bot` and `applications.commands`, and these bot permissions:
   - Send Messages
   - Read Message History
5. Invite the bot to your server using the generated URL

## Installation

### Docker Compose (Recommended)

1. **Clone the repository**:

```bash
git clone <repository-url>
cd discord-quotes-bot
```

2. **Place your GGUF model** at `~/models/hermes/hermes-llama3.2.gguf` (or update the path in `ollama-init.sh` and `compose.yaml`).

3. **Create a `.env` file**:

```bash
# Required
DISCORD_TOKEN=your_discord_bot_token_here

# Optional: prefix the bot responds to (default: georgibot)
MENTION_PREFIX=georgibot

# Optional: comma-separated words that trigger a response when found in any message
TRIGGER_WORDS=bulgaria,georgi

# Optional: default system prompt for AI responses
SYSTEM_PROMPT=You are georgibot, a friendly AI assistant in a Discord server.

# Optional: per-user system prompt overrides (username in UPPERCASE)
# SYSTEM_PROMPT_JOHNDOE=You are a coding tutor for John.
```

4. **Start the services**:

```bash
docker compose up -d
```

5. **Check logs**:

```bash
docker compose logs -f discord-quotes-bot
docker compose logs -f ollama
```

### Local Development

```bash
go mod download
export DISCORD_TOKEN=your_token_here
export OLLAMA_HOST=http://localhost:11434

# Ensure Ollama is running with the hermes model available
go run .
```

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DISCORD_TOKEN` | **Yes** | — | Discord bot token |
| `MENTION_PREFIX` | No | `georgibot` | Prefix that triggers the bot (case-insensitive) |
| `TRIGGER_WORDS` | No | — | Comma-separated words that trigger a response when found in a message |
| `SYSTEM_PROMPT` | No | Generic friendly prompt | Base system prompt for all interactions |
| `SYSTEM_PROMPT_<USERNAME>` | No | — | Per-user system prompt override (username in UPPERCASE) |
| `OLLAMA_HOST` | No | `http://localhost:11434` | Ollama service URL |

### Ollama Generation Parameters

These are set in `inference.go` and tuned for Raspberry Pi hardware:

| Parameter | Value | Effect |
|-----------|-------|--------|
| `num_predict` | 512 | Maximum tokens to generate per response |
| `num_ctx` | 4096 | Context window size |
| `num_threads` | 4 | CPU threads for inference (match your Pi's core count) |

Modelfile parameters (in `ollama-init.sh`):

| Parameter | Value | Effect |
|-----------|-------|--------|
| `temperature` | 0.7 | Creativity level (0.0–1.0) |
| `top_p` | 0.8 | Nucleus sampling threshold |
| `top_k` | 20 | Token selection diversity |
| `repeat_penalty` | 1.1 | Repetition reduction |

## Usage

### Mention the bot

```
georgibot, what's for dinner?
```

or with `@`:

```
@georgibot tell me a joke
```

### Trigger words

If `TRIGGER_WORDS=bulgaria,history` is set, any message containing those words will get a response:

```
I've always wanted to visit Bulgaria
```

### Reply to continue a conversation

Reply to the bot's message to continue with context (hermes model only). Context persists for 15 minutes of inactivity per user.

### Custom prefix

Set `MENTION_PREFIX=alfred` in `.env`:

```
alfred, how's it going?
```

## Docker Compose Services

| Service | Description |
|---------|-------------|
| `ollama` | Hosts the LLM. Runs on `linux/arm64`, exposes port `11434`, persists model data in a named volume. |
| `discord-quotes-bot` | The bot application. Waits for Ollama readiness before starting. Loads config from `.env`. |

## Troubleshooting

### Bot not responding

1. Verify `DISCORD_TOKEN` is correct in `.env`
2. Confirm **Message Content Intent** is enabled in the Developer Portal
3. Check that `MENTION_PREFIX` or `TRIGGER_WORDS` match what you're typing (they're case-insensitive)
4. Check logs: `docker compose logs -f discord-quotes-bot`

### AI returns errors

1. Confirm Ollama is running: `docker compose ps ollama`
2. Verify the model exists: `docker compose exec ollama ollama list`
3. Ensure the GGUF file is at the expected path
4. Check Ollama logs: `docker compose logs -f ollama`

### Context not persisting

- Context clears after 15 minutes of inactivity per user
- Context is only maintained for the `hermes` model (generate API), not `impersonate` (chat API)
- Restarting the bot clears all context
- Context is capped at 4096 tokens; older tokens are dropped

### Bot says it's busy

Only one inference runs at a time to protect Pi resources. Wait for the current request to finish and try again.

## Acknowledgments

- [discordgo](https://github.com/bwmarrin/discordgo) — Discord API wrapper for Go
- [Ollama](https://ollama.com/) — Local LLM runtime
- [Hermes](https://huggingface.co/NousResearch) — Base model family
