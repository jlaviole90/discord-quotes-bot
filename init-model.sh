#!/bin/bash
set -e

echo "Waiting for Ollama service to start..."
until curl -s http://ollama:11434/api/tags > /dev/null 2>&1; do
    sleep 2
    echo "Waiting..."
done

echo "Checking if model exists..."
if curl -s http://ollama:11434/api/tags | grep -q "qwen2.5:3b"; then
    echo "Model already exists, skipping creation..."
else
    echo "Creating model qwen2.5:3b from GGUF file..."

    echo '{"name":"qwen2.5:3b","modelfile":"FROM /models/qwen/qwen2.5-3b-instruct.Q4_K_M.gguf"}' | \
        curl -X POST http://ollama:11434/api/create \
        -H "Content-Type: application/json" \
        -d @-
    echo ""
    echo "Waiting for model to be ready..."
    sleep 10

    echo "Model created. Listing models:"
    curl -s http://ollama:11434/api/tags | jq .
fi

echo "starting discord bot..."
exec /app/discord-quotes-bot
