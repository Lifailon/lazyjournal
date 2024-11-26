#!/bin/bash

ARCH=$(uname -m)
case $ARCH in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *)
        echo -e "\033[31mError.\033[0m Processor architecture not supported: $ARCH"
        echo -e "Create a request with a \033[31mproblem\033[0m: https://github.com/Lifailon/lazyjournal/issues"
        exit 1
        ;;
esac

mkdir -p $HOME/.local/bin
grep -F 'export PATH=$PATH:$HOME/.local/bin' $HOME/.bashrc > /dev/null || { 
    echo 'export PATH=$PATH:$HOME/.local/bin' >> $HOME/.bashrc && source $HOME/.bashrc; 
}

GITHUB_LATEST_VERSION=$(curl -L -s -H 'Accept: application/json' https://github.com/Lifailon/lazyjournal/releases/latest | sed -e 's/.*"tag_name":"\([^"]*\)".*/\1/')
if [ -z "$GITHUB_LATEST_VERSION" ]; then
    echo -e "\033[31mError.\033[0m Unable to get the latest version from GitHub repository, check your internet connection."
    exit 1
else
    BIN_URL="https://github.com/Lifailon/lazyjournal/releases/download/$GITHUB_LATEST_VERSION/lazyjournal-$GITHUB_LATEST_VERSION-linux-$ARCH"
    curl -L -s "$BIN_URL" -o $HOME/.local/bin/lazyjournal
    chmod +x $HOME/.local/bin/lazyjournal
    echo -e "✔  Installation completed \033[32msuccessfully\033[0m"
    exit 0
fi
