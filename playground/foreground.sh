docker run -d --name dozzle-varlog -v /var/log:/logs lifailon/dozzle-varlog:latest
podman run -d --name dozzle-varlog -v /var/log:/logs lifailon/dozzle-varlog:latest

curl -sS https://raw.githubusercontent.com/Lifailon/lazyjournal/main/install.sh | bash
. /root/.bashrc

lazyjournal