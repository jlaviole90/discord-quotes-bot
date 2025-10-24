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

    MODELFILE_CONTENT=$(cat /app/Modelfile | sed 's/"/\\"/g' | awk '{printf "%s\\n", $0}' | sed 's/\\n$//')

    cat > /tmp/payload.json << EOF
{
    "name": "qwen2.5:3b",
    "modelfile": "${MODELFILE_CONTENT}"
}
EOF
    echo "Sending model creation request to Ollama..."
    curl -X POST http://ollama:11434/api/create \
        -H "Content-Type: application/json" \
        -d @/tmp/payload.json

    echo ""
    echo "Model created successfully!"
fi

echo "starting discord bot..."
exec /app/discord-quotes-bot
