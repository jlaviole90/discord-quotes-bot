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

    cat > /tmp/Modelfile << 'EOF'
FROM /models/qwen/qwen2.5-3b-instruct.Q4_K_M.gguf
TEMPLATE """{{ if .System }}<|im_start|>system
{{ .System }}<|im_end|>
{{ end }}{{ if .Prompt }}<|im_start|>user
{{ .Prompt }}<|im_end|>
{{ end }}<|im_start|>assistant
"""
PARAMETER stop "<|im_start|>"
PARAMETER stop "<|im_end|>"
PARAMETER temperature 0.7
PARAMETER top_p 0.8
PARAMETER top_k 20
EOF

    curl -X POST http://ollama:11434/api/create -d "{
        \"name\": \"qwen2.5:3b\",
        \"modelfile\": \"$(cat /tmp/Modelfile | sed 's/\"/\\\\"/g' | tr '\n' ' ')\"
    }"

    echo "Model created successfully!"
fi

echo "starting discord bot..."
exec /app/discord-quotes-bot
