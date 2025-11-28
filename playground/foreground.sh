echo -e "\033[32mInstalling k3s (lightweight Kubernetes cluster)...\033[0m"
curl -sfL https://get.k3s.io | sh -

echo -e "\033[32mInstalling docker-compose...\033[0m"
curl -sSL "https://github.com/docker/compose/releases/download/v2.40.3/docker-compose-$(uname -s)-$(uname -m)" -o /usr/bin/docker-compose

echo -e "\033[32mInstalling Docker Socket Proxy in the Podman...\033[0m"
podman run -d -p 2375:2375 --name docker-socket-haproxy -v /var/run/docker.sock:/var/run/docker.sock lifailon/docker-socket-proxy:amd64
while true; do curl -s http://localhost:2375/_ping > /dev/null; sleep 2; done &

echo -e "\033[32mInstalling prometheus and logporter (lightweight alternative to cadvisor) in the compose stack...\033[0m"
curl -sSL https://raw.githubusercontent.com/Lifailon/lazyjournal/refs/heads/main/playground/prometheus.yml -o prometheus.yml
curl -sSL https://raw.githubusercontent.com/Lifailon/lazyjournal/refs/heads/main/playground/docker-compose.yml -o docker-compose.yml
docker-compose up -d

echo -e "\033[32mCreate test log for custom path in the file system...\033[0m"
mkdir /test
curl -sSL https://raw.githubusercontent.com/Lifailon/lazyjournal/refs/heads/main/color.log -o /test/color.log

echo -e "\033[32mInstalling lazyjournal binary from the GitHub repository...\033[0m"
curl -sS https://raw.githubusercontent.com/Lifailon/lazyjournal/main/install.sh | bash
. /root/.bashrc

lazyjournal -p /test -t 5000 -u 2