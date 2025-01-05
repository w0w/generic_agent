#!/bin/bash

# Set environment variables
export TELEGRAM_BOT_TOKEN="your_bot_token_here"
export OPENROUTER_API_KEY="your_openrouter_key_here"
export LOG_LEVEL="debug"
export DEBUG=1

# Build with debug information
go build -gcflags="all=-N -l" -o debug_app

# Run with delve debugger
dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec ./debug_app
