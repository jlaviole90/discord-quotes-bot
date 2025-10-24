#!/bin/bash

echo "Setting up Ollama with Qwen2.5 model..."

echo "Waiting for Ollama service to start..."
until curl -s http://localhost:11434/api/tags > /dev/null 2>&1; do
    sleep 2
    echo "Waiting..."
done

echo "Ollama is ready!"

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

echo "Creating Ollama model from GGUF file..."
docker exec discord-quotes-bot-ollama-1 ollama create qwen2.5:3b -f /tmp/Modelfile

echo "Model setup complete!"
echo "You can test it with: docker exec discord-quotes-bot-ollama-1 ollama run qwen2.5:3b 'Hello!'"
