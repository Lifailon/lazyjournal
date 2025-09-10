#!/bin/sh

TTYD_MODE=$(echo "$TTYD" | tr '[:upper:]' '[:lower:]')
if [ "$TTYD_MODE" = "true" ]; then
    TTYD_OPTIONS="-W -p ${PORT:-5555}"
    if [ -n "${USERNAME}" ] && [ -n "${PASSWORD}" ]; then
        TTYD_OPTIONS="$TTYD_OPTIONS -c ${USERNAME}:${PASSWORD}"
    fi
    ttyd $TTYD_OPTIONS lazyjournal
else
    lazyjournal
fi