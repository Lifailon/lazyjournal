#!/bin/sh

TTYD_MODE=$(echo "$TTYD" | tr '[:upper:]' '[:lower:]')
if [ "$TTYD_MODE" = "true" ]; then
    TTYD_OPTIONS="-W -p ${PORT:-5555}"
    # Credential for basic auth
    if [ -n "${USERNAME}" ] && [ -n "${PASSWORD}" ]; then
        TTYD_OPTIONS="$TTYD_OPTIONS -c ${USERNAME}:${PASSWORD}"
    fi
    # SSL
    if [ -n "${CERT}" ] && [ -n "${KEY}" ] && [ -f "${CERT}" ] && [ -f "${KEY}" ]; then
        TTYD_OPTIONS="$TTYD_OPTIONS -S -C ${CERT} -K ${KEY}"
    fi
    # Theme and font size
    TTYD_OPTIONS="$TTYD_OPTIONS -t theme={\"background\":\"${BACKGROUND:-#1e1e2e}\",\"foreground\":\"${FOREGROUND:-#cdd6f4}\"} -t fontSize=${FONT_SIZE:-14}"
    exec ttyd $TTYD_OPTIONS lazyjournal $OPTIONS
else
    exec lazyjournal $OPTIONS
fi