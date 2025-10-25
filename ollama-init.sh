#!/bin/bash

/bin/ollama serve &

echo "Waiting for Ollama to start..."
until ollama list > /dev/null 2>&1; do
    sleep 2
done

echo "Ollama started!"

echo "Checking for GGUF file..."
ls -lah /models/phi/ || echo "Directory /models/phi/ not found!"
echo ""

if [ -f "/models/phi/phi3-mini-4k.gguf" ]; then
    echo "Found GGUF file: /models/phi/phi3-mini-4k.gguf"
else
    echo "X GGUF file not found at /models/phi/phi3-mini-4k.gguf"
    echo "Available files in /models:"
    ls -lah /models/ || echo "Cannot access /models/"
    exit 1
fi

if ollama list | grep -q "phi3:mini"; then
    echo "Model phi3:mini already exists."
else
    echo "Creating model phi3:mini from GGUF file..."

    cat > /tmp/Modelfile << 'EOF'
FROM /models/phi/phi3-mini-4k.gguf

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
    echo "Modelfile contents:"
    cat /tmp/Modelfile
    echo ""

    echo "Running: ollama create phi3:mini -f /tmp/Modelfile"
    ollama create phi3:mini -f /tmp/Modelfile

    echo "Completed model creation script."
fi

echo "Available models:"
ollama list

wait
