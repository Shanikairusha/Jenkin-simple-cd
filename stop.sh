#!/bin/bash

PID_FILE="cd-agent.pid"

echo "Stopping CD Agent..."

if [ ! -f "$PID_FILE" ]; then
    echo "Error: PID file not found. Is the agent running?"
    exit 1
fi

PID=$(cat "$PID_FILE")

if kill -0 $PID 2>/dev/null; then
    # Process is running, send gracefully termination signal
    kill $PID
    echo "Sent termination signal to PID $PID."
    
    # Wait for it to shut down
    sleep 2
    if kill -0 $PID 2>/dev/null; then
        echo "Process did not stop gracefully. Forcing termination..."
        kill -9 $PID
    fi
    
    rm "$PID_FILE"
    echo "CD Agent stopped successfully."
else
    echo "Process $PID is not running. Cleaning up stale PID file."
    rm "$PID_FILE"
fi
