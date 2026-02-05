<p align="center">
    <img src="/img/logo.png">
</p>

<p align="center">
    <a href="https://github.com/Lifailon/lazyjournal/releases"><img title="GitHub Download" src="https://img.shields.io/github/downloads/lifailon/lazyjournal/total?logo=github&color=green&label=Downloads"></a>
    <a href="https://launchpad.net/~lifailon/+archive/ubuntu/lazyjournal"><img title="Ubuntu Launchpad PPA" src="https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fapi.launchpad.net%2F1.0%2F~lifailon%2F%2Barchive%2Fubuntu%2Flazyjournal%3Fws.op%3DgetPublishedSources%26distro_series%3Dhttps%3A%2F%2Fapi.launchpad.net%2F1.0%2Fubuntu%2Fjammy%26status%3DPublished&query=entries[0].source_package_version&label=Ubuntu+PPA&logo=ubuntu&color=orange"></a>
    <a href="https://formulae.brew.sh/formula/lazyjournal"><img title="Homebrew" src="https://img.shields.io/homebrew/v/lazyjournal?logo=homebrew&color=yellow&label=Homebrew"></a>
    <a href="https://anaconda.org/conda-forge/lazyjournal"><img title="Conda Forge" src="https://img.shields.io/conda/vn/conda-forge/lazyjournal?logo=anaconda&color=green&label=Conda"></a>
    <a href="https://aur.archlinux.org/packages/lazyjournal"><img title="Arch Linux" src="https://img.shields.io/aur/version/lazyjournal?logo=arch-linux&color=blue&label=AUR"></a>
    <a href="https://hub.docker.com/r/lifailon/lazyjournal"><img title="Docker Hub" src="https://img.shields.io/docker/image-size/lifailon/lazyjournal/latest?logo=docker&color=blue&label=Docker+Hub"></a>
<br>
    <a href="https://github.com/Lifailon/lazyjournal/actions/workflows/build.yml"><img title="Actions Build"src="https://github.com/Lifailon/lazyjournal/actions/workflows/build.yml/badge.svg"></a>
    <a href="https://github.com/Lifailon/lazyjournal/wiki"><img title="Go coverage report"src="https://raw.githubusercontent.com/wiki/Lifailon/lazyjournal/coverage.svg"></a>
    <a href="https://goreportcard.com/report/github.com/Lifailon/lazyjournal"><img src="https://goreportcard.com/badge/github.com/Lifailon/lazyjournal" alt="Go Report"></a>
    <a href="https://app.fossa.com/projects/git%2Bgithub.com%2FLifailon%2Flazyjournal?ref=badge_shield&issueType=security" alt="FOSSA Status"><img src="https://app.fossa.com/api/projects/git%2Bgithub.com%2FLifailon%2Flazyjournal.svg?type=shield&issueType=security"/></a>
<br>
    <a href="https://github.com/avelino/awesome-go?tab=readme-ov-file#logging"><img src="https://awesome.re/mentioned-badge.svg" alt="Mentioned in Awesome Go"></a>
    <a href="https://pkg.go.dev/github.com/Lifailon/lazyjournal"><img src="https://pkg.go.dev/badge/github.com/Lifailon/lazyjournal.svg" alt="Go Reference"></a>
    <a href="https://deepwiki.com/Lifailon/lazyjournal"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</p>

Terminal user interface for viewing logs from `journald`, `auditd`, file system, Docker and Podman containers, Compose stacks and Kubernetes pods with supports log highlighting and several filtering modes. Written in Go with the [awesome-gocui](https://github.com/awesome-gocui/gocui) (fork [gocui](https://github.com/jroimartin/gocui)) library.

This tool is inspired by and with love for [LazyDocker](https://github.com/jesseduffield/lazydocker) and [LazyGit](https://github.com/jesseduffield/lazygit). It is also included in [Awesome-Go](https://github.com/avelino/awesome-go?tab=readme-ov-file#logging), [Awesome-TUIs](https://github.com/rothgar/awesome-tuis?tab=readme-ov-file#development) and [Awesome-Docker](https://github.com/veggiemonk/awesome-docker?tab=readme-ov-file#terminal-ui), check out other useful projects on the repository pages.

> [!NOTE]
> You can try it out on the [Killercoda](https://killercoda.com/lazyjournal/scenario/playground) playground.

![Regex filtering](/img/fuzzy.png)

## Features

- Simple installation, to run download one executable file without dependencies and settings.
- It is possible to launch the interface in a Web browser when running in a Docker container.
- Centralized search for the required journal by filtering all lists (log sources).
- Streaming output of new events from the selected journal (like `tail`).
- List of all services (including disabled unit files) with current state from `systemd` to access their logs.
- View all system and user journals via `journalctl` (tool for reading logs from [journald](https://github.com/systemd/systemd/tree/main/src/journal)).
- List of all system boots for kernel log output.
- List of audit rules from `auditd` for filtering by keys and viewing in `interpret` format.
- File system logs such as for `Apache` or `Nginx`, as well as `syslog`, `messages`, etc. from `/var/log`.
- Lists all log files in users home directories, as well as descriptor log files used by processes.
- Reading archive logs truncated during rotation (`gz`, `xz` and `bz2` formats) and Packet Capture (`pcap` format).
- Apple System Logs support (`asl` format).
- Windows Event Logs via `PowerShell` and `wevtutil`, as well as application logs from Windows file system.
- Docker and Swarm logs from the file system or stream, including build-in timestamps and filtering by stream.
- Logs of all containers in Docker Compose stacks, sorted by time for all entries.
- Podman logs, without the need to run a background process (socket).
- Kubernetes pods logs (you must first configure the cluster connection in `kubeconfig`).
- Logs of [k3s](https://github.com/k3s-io/k3s) pods and containers from the file system on any nodes (including workers).
- Search and analyze all logs from remote hosts in one interface using [rsyslog](https://www.rsyslog.com) configuration.
- Access to logs on remote systems via the `ssh` protocol.
- Supports context and namespace switching interface for Docker and Kubernetes.

### Filtering

Supports 4 filtering modes:

- **Default** - case sensitive exact search.
- **Fuzzy** (like `fzf`) - custom inexact case-insensitive search (searches for all phrases separated by a space anywhere on a line).
- **Regex** (like `grep`) - search with regular expression support, based on the built-in [regexp](https://pkg.go.dev/regexp) library, case-insensitive by default (in case a regular expression syntax error occurs, the input field will be highlighted in red).
- **Date** - filtering by date (`since` and/or `until`) for journald logs, as well as Docker or Podman containers (only supported in streaming mode) using the left and right arrow keys. This mode affects log loading (this is especially relevant for large logs, thereby increasing performance) and can be used in combination with other filtering modes.

### Highlighting

Several log output coloring modes are supported:

- **default** - built-in log highlighter by default, requires no dependencies and is several times faster than other tools (including in [command-line mode](#command-line-mode)).
- **tailspin** - uses [tailspins](https://github.com/bensadeh/tailspin).
- **bat** - uses [bat](https://github.com/sharkdp/bat) in ansi mode and log language (much slower than other modes).

When using external tools, they are required to be already installed on your system. You can also disable output highlighting with the `Ctrl+Q` keyboard shortcut, this is useful for improving performance when viewing large logs or if your terminal already has a built-in highlighting feature, such as [WindTerm](https://github.com/kingToolbox/WindTerm).

The built-in highlighting by default supports several color groups:

- **Custom** - URLs, HTTP request methods and response status codes, double quotes and braces for json, file paths and processes in UNIX.
- **Yellow** - warnings and known names (host name and system users).
- **Green** - keywords indicating success.
- **Red** - keywords indicating error.
- **Blue** - statuses and actions (restart, update, etc).
- **Light blue** - numbers (date, time, timestamp, bytes, versions, percentage, integers, IP and MAC addresses).

A full list of all keywords can be found in the [color.log](/color.log) file (used for testing only).

> [!IMPORTANT]
> If you have suggestions for improving log highlighting (e.g. adding new keywords), you can open an [issue](https://github.com/Lifailon/lazyjournal/issues) for a new feature.

## Install

Binaries are available for download on the GitHub [releases](https://github.com/Lifailon/lazyjournal/releases) page.

### Unix-based

Run the command in the console to quickly install or update the stable version for Linux, macOS or the BSD-based system:

```bash
curl -sS https://raw.githubusercontent.com/Lifailon/lazyjournal/main/install.sh | bash
```

This command will run a script that will download the latest binary (auto-detect OS and arch) from the GitHub repository to your home directory along with other executables (default path is `~/.local/bin/lazyjournal`) and configurations (`~/.config/lazyjournal/config.yml`) for the current user, and also grant execute permission.

### apt (Ubuntu / Debian)

To install and remove packages on an Ubuntu system, you can use the `apt` package manager from the [PPA](https://launchpad.net/~lifailon/+archive/ubuntu/lazyjournal) repository:

```bash
sudo add-apt-repository -y ppa:lifailon/lazyjournal
sudo apt update
sudo apt install -y lazyjournal
```

Or download the `deb` package from the GitHub releases page for installation on any Debian-based system.

```bash
curl -sSL https://github.com/Lifailon/lazyjournal/releases/download/0.8.4/lazyjournal-0.8.4-$(dpkg --print-architecture).deb -o /tmp/lazyjournal.deb
sudo apt install -y /tmp/lazyjournal.deb && rm /tmp/lazyjournal.deb
```

### Homebrew (macOS / Linux)

Use the following command to install `lazyjournal` using [Homebrew](https://formulae.brew.sh/formula/lazyjournal):

```bash
brew install lazyjournal
```

### Conda / Mamba / Pixi (Linux / macOS / Windows)

If you use package managers like conda or mamba, you can install `lazyjournal` from [conda-forge](https://anaconda.org/conda-forge/lazyjournal):

```bash
conda install -c conda-forge lazyjournal
mamba install -c conda-forge lazyjournal
```

You can install `lazyjournal` user-globally using [pixi](https://prefix.dev):

```bash
pixi global install lazyjournal
```

### Arch Linux

If you an Arch Linux user you can also install from the [AUR](https://aur.archlinux.org/packages/lazyjournal):

```bash
paru -S lazyjournal
```

### Docker (Debian-based)

Download the [docker-compose](/docker-compose.yml) file and run the container using the image from [Docker Hub](https://hub.docker.com/r/lifailon/lazyjournal):

```bash
mkdir lazyjournal && cd lazyjournal
curl -sS https://raw.githubusercontent.com/Lifailon/lazyjournal/refs/heads/main/docker-compose.yml -o docker-compose.yml
docker compose up -d
docker exec -it lazyjournal lazyjournal
```

The image is based on Debian with `systemd`, docker cli, `docker-compose` and `kubectl` pre-installed. The necessary permissions are already pre-set in the file to support all log sources from the host system.

To access Kubernetes logs, you need to forward the configuration to the container. If you're using a local cluster (e.g., k3s), change the cluster server address in the configuration to the host address on the local network.

### Web mode

Supports running in a container with a Web interface, using [ttyd](https://github.com/tsl0922/ttyd) to access logs via a browser. To do this, edit the variables:

```env
# Enable Web mode
TTYD=true
PORT=5555
# Credentials for accessing via Web browser (optional)
USERNAME=admin
PASSWORD=admin
# Flags used (optional)
OPTIONS=-t 5000 -u 2
```

### Windows

Use the following command to quickly install in your PowerShell console:

```PowerShell
irm https://raw.githubusercontent.com/Lifailon/lazyjournal/main/install.ps1 | iex
```

The following directories are used to search for logs in the file system:

- `Program Files`
- `Program Files (x86)`
- `ProgramData`
- `AppData\Local` and `AppData\Roamin` for current user

To read logs, automatic detection of the following encodings is supported:

- `UTF-8`
- `UTF-16 with BOM`
- `UTF-16 without BOM`
- `Windows-1251` by default

### Go package

You can install it as a package using [Go](https://pkg.go.dev/github.com/Lifailon/lazyjournal) (this will install the version from the latest commit, which may be unstable):

```bash
go install github.com/Lifailon/lazyjournal@latest
```

## Usage

You can start the interface from anywhere: `lazyjournal` or use `lazyjournal -h` for help on available flags.

The application is an interface for viewing logs with the ability to filter them for analysis. Therefore, to access the logs themselves, it is necessary that such programs as [docker-cli](https://github.com/docker/cli), [compose](https://github.com/docker/compose), [podman](https://github.com/containers/podman) and [kubectl](https://github.com/kubernetes/kubectl) are already installed on your system.

Access to all system logs and containers may require elevated privileges for the current user. For example, if a user does not have read permission to the directory `/var/lib/docker/containers`, he will not be able to access all archived logs from the moment the container is started, but only from the moment the containerization system is started, so the process of reading logs is different. However, reading in streaming mode is faster than parsing json logs from the file system.

### Configuration

Hotkeys, interface settings and application flag values can be overridden using the [config](/config.yml) file (see [#23](https://github.com/Lifailon/lazyjournal/issues/23), [#27](https://github.com/Lifailon/lazyjournal/issues/27) and [#37](https://github.com/Lifailon/lazyjournal/issues/37) issues), which can be located in `~/.config/lazyjournal/config.yml`, as well as next to the executable file or in the current startup directory (has high priority).

Mouse control is supported (but can also be disabled with the `-m` flag or configuration) for selecting window and the log from list, as well as lists and logs scrolling. To copy text, use the `Alt+Shift` key combination while selecting.

### Remote mode

Supports access to logs on a remote system using standard `ssh` arguments to configure the connection (passed as a single quoted parameter).

```bash
lazyjournal --ssh "lifailon@192.168.3.101"
# Passing arguments
lazyjournal --ssh "lifailon@192.168.3.101 -p 2121 -o Compression=yes"
# If sudo is supported without entering a password
lazyjournal --ssh "lifailon@192.168.3.101 sudo"
```

You can also pass a list of hosts to the lazyjournal configuration and switch between them in the interface (use the `F2` key to invoke ssh manager):

```yaml
ssh:
  hosts:
    - lifailon@192.168.3.101
    - lifailon@192.168.3.105 -p 2121 -o Compression=yes
    - lifailon@192.168.3.106 -o StrictHostKeyChecking=no -o ConnectTimeout=5
```

Remote access is only possible using an ssh key (since each function call requires a password). However, password-based access is possible by configuring password storage for a long period of time in the `~/.ssh/config` file:

```sshconfig
Host *
  # Keep the connection open for 1 hour
  ControlMaster auto
  ControlPath ~/.ssh/%r@%h:%p
  ControlPersist 1h
  # Avoid errors when checking the key of a host you have not connected to before
  StrictHostKeyChecking no
```

### Command-line mode

Output highlighting and filtering are supported in command line mode:

```bash
# Add an alias to your shell profile
alias lj=lazyjournal >> $HOME/.bashrc

# Highlight output from standard input
cat /var/log/syslog | lj -c

# Filtering in fuzzy search
cat /var/log/syslog | lj -f "error"

# Filtering using regular expressions
cat /var/log/syslog | lj -r "error|fatal|fail|crash"

# Filtering with subsequent highlighting of the output
cat /var/log/syslog | lj -r "error|fatal|fail|crash" -c
```

### Build

Clone the repository and use Make to build the binary:

```bash
git clone https://github.com/Lifailon/lazyjournal
cd lazyjournal
make build
```

> [!NOTE]
> The repository uses AI-powered release analysis in the [changelog](/CHANGELOG.md) file, as well review commits, PR and issues in [Actions](https://github.com/Lifailon/lazyjournal/actions).

### Testing

Unit tests cover all main functions and interface operation.

```bash
# Get a list of all tests
make test-list
# Run selected or all tests
make test-run case=TestMockInterface
make test-all
```

> [!NOTE]
> Build and testing is automated using CI Actions. A detailed test coverage report for Linux, macOS and Windows systems is available on the [Wiki](https://github.com/Lifailon/lazyjournal/wiki) page.

## Contributing

Since this is my first Go project, there may be some bad practices, BUT I want to make `lazyjournal` better. Any contribution will be appreciated! If you want to implement any new feature or fix something, please [open an issue](https://github.com/Lifailon/lazyjournal/issues) first.

Thanks to all participants for their contributions:

- [Matteo Giordano](https://github.com/malteo) for upload and update the package in `AUR`.
- [Ueno M.](https://github.com/eunos-1128) for upload and update the package in `Homebrew` and `Conda`.

You can also upload the package yourself to any package manager you use and make [Pull Requests](https://github.com/Lifailon/lazyjournal/pulls).

## Alternatives

- [Lnav](https://github.com/tstack/lnav) - The Logfile Navigator is a log file viewer for the terminal.
- [Toolong](https://github.com/Textualize/toolong) - A terminal application to view, tail, merge, and search log files.
- [Nerdlog](https://github.com/dimonomid/nerdlog) - A remote, multi-host TUI syslog viewer with timeline histogram and no central server.
- [Gonzo](https://github.com/control-theory/gonzo) - A log analysis terminal UI with beautiful charts and AI-powered insights.
- [Dozzle](https://github.com/amir20/dozzle) - A small lightweight application with a Web based interface to monitor Docker logs.

## License

This project is licensed under the **MIT License**. See the [LICENSE](LICENSE) file for details.

Copyright (C) 2024 Lifailon (Alex Kup)