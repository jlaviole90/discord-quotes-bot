#!/bin/bash

/bin/ollama serve &

echo "Waiting for Ollama to start..."
until ollama list > /dev/null 2>&1; do
    sleep 2
done

echo "Ollama started!"

if ollama list | grep -q "qwen2.5:3b"; then
    echo "Model qwen2.5:3b already exists."
else
    echo "Creating model qwen2.5:3b from GGUF file..."
    ollama create qwen2.5:3b -f - << 'EOF'
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
    ehcho "Model created successfully!"
fi

echo "Available models:"
ollama list

wait
