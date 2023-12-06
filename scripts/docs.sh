#!/bin/sh

ROOT=$(git rev-parse --show-toplevel)

if [ -f "$ROOT/jotbot.env" ]; then
  set -a
	. "$ROOT/jotbot.env"
	set +a
fi

if [ -z "$OPENAI_API_KEY" ]; then
	echo "Missing OPENAI_API_KEY environment variable"
	exit 1
fi

if ! command -v jotbot &> /dev/null; then
	echo "JotBot is not available. Run 'go install github.com/modernice/jotbot/cmd/jotbot@latest' to install."
	exit 1
fi

jotbot generate "$ROOT"
