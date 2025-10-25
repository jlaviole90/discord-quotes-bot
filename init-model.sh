#!/bin/bash
set -e

echo "Waiting for Ollama service to be ready..."
until curl -s http://ollama:11434/api/tags > /dev/null 2>&1; do
    sleep 2
    echo "Waiting for model..."
done

echo "Model phi3:mini is ready!"

echo "Starting quotes bot..."
exec /app/discord-quotes-bot
