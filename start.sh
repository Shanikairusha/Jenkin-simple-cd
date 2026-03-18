#!/bin/bash

# Configuration
PID_FILE="cd-agent.pid"
LOG_FILE="cd-agent.log"
BINARY="./cd-agent-linux"

echo "Starting CD Agent..."

if [ -f "$PID_FILE" ]; then
    # Check if process is actually running
    PID=$(cat "$PID_FILE")
    if ps -p $PID > /dev/null; then
        echo "Error: CD Agent is already running with PID $PID."
        exit 1
    else
        echo "Found stale PID file. Cleaning up..."
        rm "$PID_FILE"
    fi
fi

# Need to ensure the binary is executable
if [ ! -x "$BINARY" ]; then
    echo "Making $BINARY executable..."
    chmod +x "$BINARY"
fi

# Run in the background using nohup
nohup $BINARY > "$LOG_FILE" 2>&1 &
NEW_PID=$!

# Save the process ID to a file so we can stop it later
echo $NEW_PID > "$PID_FILE"

echo "CD Agent started successfully in the background!"
echo "Process ID: $NEW_PID"
echo "Logs are being written to: $LOG_FILE"
