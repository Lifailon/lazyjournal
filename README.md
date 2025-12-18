<p align="center">
    <img src="/img/logo.png">
</p>

<p align="center">
    <a href="https://github.com/Lifailon/lazyjournal/actions/workflows/build.yml"><img title="Actions Build"src="https://github.com/Lifailon/lazyjournal/actions/workflows/build.yml/badge.svg"></a>
    <a href="https://github.com/Lifailon/lazyjournal/wiki"><img title="Go coverage report"src="https://raw.githubusercontent.com/wiki/Lifailon/lazyjournal/coverage.svg"></a>
    <a href="https://goreportcard.com/report/github.com/Lifailon/lazyjournal"><img src="https://goreportcard.com/badge/github.com/Lifailon/lazyjournal" alt="Go Report"></a>
    <a href="https://app.fossa.com/projects/git%2Bgithub.com%2FLifailon%2Flazyjournal?ref=badge_shield&issueType=security" alt="FOSSA Status"><img src="https://app.fossa.com/api/projects/git%2Bgithub.com%2FLifailon%2Flazyjournal.svg?type=shield&issueType=security"/></a>
    <a href="https://github.com/avelino/awesome-go?tab=readme-ov-file#logging"><img src="https://awesome.re/mentioned-badge.svg" alt="Mentioned in Awesome Go"></a>
    <a href="https://pkg.go.dev/github.com/Lifailon/lazyjournal"><img src="https://pkg.go.dev/badge/github.com/Lifailon/lazyjournal.svg" alt="Go Reference"></a>
<br>
    <a href="https://github.com/Lifailon/lazyjournal/releases"><img title="GitHub Download" src="https://img.shields.io/github/downloads/lifailon/lazyjournal/total?logo=github&color=green&label=GitHub+Downloads"></a>
    <a href="https://formulae.brew.sh/formula/lazyjournal"><img title="Homebrew" src="https://img.shields.io/homebrew/v/lazyjournal?logo=homebrew&color=yellow&label=Homebrew"></a>
    <a href="https://anaconda.org/conda-forge/lazyjournal"><img title="Conda Forge" src="https://img.shields.io/conda/vn/conda-forge/lazyjournal?logo=anaconda&color=green&label=Conda"></a>
    <a href="https://aur.archlinux.org/packages/lazyjournal"><img title="Arch Linux" src="https://img.shields.io/aur/version/lazyjournal?logo=arch-linux&color=blue&label=AUR"></a>
    <a href="https://hub.docker.com/r/lifailon/lazyjournal"><img title="Docker Hub" src="https://img.shields.io/docker/image-size/lifailon/lazyjournal/latest?logo=docker&color=blue&label=Docker+Hub"></a>
</p>

Terminal user interface for reading logs from `journald`, `auditd`, file system, Docker (including Swarm) containers, Compose stacks, Podman and Kubernetes pods with support for output coloring and multiple filtering modes. Written in Go with the [awesome-gocui](https://github.com/awesome-gocui/gocui) (fork [gocui](https://github.com/jroimartin/gocui)) library.

This tool is inspired by and with love for [LazyDocker](https://github.com/jesseduffield/lazydocker) and [LazyGit](https://github.com/jesseduffield/lazygit). It is also included in [Awesome-Go](https://github.com/avelino/awesome-go?tab=readme-ov-file#logging), [Awesome-TUIs](https://github.com/rothgar/awesome-tuis?tab=readme-ov-file#development) and [Awesome-Docker](https://github.com/veggiemonk/awesome-docker?tab=readme-ov-file#terminal-ui), check out other useful projects on the repository pages.

> [!NOTE]
> You can try it out on the [Killercoda](https://killercoda.com/lazyjournal/scenario/playground) playground.

![Regex filtering](/img/regex.png)

<details>
    <summary>Screenshots</summary>
    <li>Filtering the file log using fuzzy search:</li>
    <img src="./img/fuzzy.jpg" alt="Filtering the file log using fuzzy search">
    <li>Filtering by timestamp for ssh log from systemd:</li>
    <img src="./img/timestamp.jpg" alt="Filtering by timestamp for ssh log from systemd" />
    <li>Disabling built-in timestamp in docker logs and filtering by stream:</li>
    <img src="./img/docker.jpg" alt="Disabling built-in timestamp in docker logs and filtering by stream" />
    <li>Demo file of built-in output coloring for the log:</li>
    <img src="./img/coloring.jpg" alt="Demo file of built-in output coloring for the log" />
</details>

## Features

- Simple installation, to run download one executable file without dependencies and settings.
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
- Docker and Swarm logs from the file system or stream, including build-in timestamps and filtering by stream.
- Logs of all containers in Docker Compose stacks, sorted by time for all entries.
- Podman logs, without the need to run a background process (socket).
- Kubernetes pods logs (you must first configure a connection to the cluster via `kubectl`).
- Logs of [k3s](https://github.com/k3s-io/k3s) pods and containers from the file system on any nodes (including workers).
- Windows Event Logs via `PowerShell` and `wevtutil`, as well as application logs from Windows file system.
- Search and analyze all logs from remote hosts in one interface using [rsyslog](https://www.rsyslog.com) configuration.
- Access to logs on a remote system via ssh protocol.

### Filtering

Supports 4 filtering modes:

- **Default** - case sensitive exact search.
- **Fuzzy** (like `fzf`) - custom inexact case-insensitive search (searches for all phrases separated by a space anywhere on a line).
- **Regex** (like `grep`) - search with regular expression support, based on the built-in [regexp](https://pkg.go.dev/regexp) library, case-insensitive by default (in case a regular expression syntax error occurs, the input field will be highlighted in red).
- **Timestamp** - filter `since` and/or `until` by date and time for `journald` and docker or podman logs (only in stream mode). This mode affects the loading of the log (thereby increasing performance) and can be used in conjunction with other filtering modes, so the current log should be reloaded by pressing `Enter` in the current input field.

Supported formats for filtering by timestamp:

- `00:00`
- `00:00:00`
- `2025-04-14`
- `2025-04-14 00:00`
- `2025-04-14 00:00:00`

Examples of short format:

- Since `-48h` until `-24h` for container logs from journald (logs for the previous day).
- Since `+1h` until `+30m` for system journals from docker or podman.

### Coloring

Several log output coloring modes are supported:

- **default** - built-in output coloring, requires no dependencies and is several times faster than other tools (including command-line mode).
- **tailspin** - uses [tailspin](https://github.com/bensadeh/tailspin) (requires the tool to be installed on the system).
- **bat** - uses [bat](https://github.com/sharkdp/bat) in ansi mode and log language (requires the tool to be installed on the system).

It is also possible to disable coloring, this is useful if your terminal already has output coloring built in, such as [WindTerm](https://github.com/kingToolbox/WindTerm).

The built-in coloring by default supports several color groups:

- **Custom** - URLs, HTTP methods (GET, POST, etc), double quotes and braces for json, file paths and processes in UNIX.
- **Yellow** - warnings and known names (host name and system users).
- **Green** - keywords indicating success.
- **Red** - keywords indicating error.
- **Blue** - statuses and actions (restart, update, etc).
- **Light blue** - numbers (date, time, timestamp, bytes, versions, percentage, integers, IP and MAC addresses).

A full list of all keywords can be found in the [color.log](/color.log) file (used for testing only). If you have suggestions for improving coloring (e.g. adding new words), you can open an [issue](https://github.com/Lifailon/lazyjournal/issues) for a new feature. 

Coloring directly affects the loading time of the log, to increase the performance of reading large logs, it is possible to disable coloring using the `Ctrl+Q`.

## Install

Binaries are available for download on the [releases](https://github.com/Lifailon/lazyjournal/releases) page.

List of supported systems and architectures in which functionality is checked: 

| OS        | amd64 | arm64 | Systems                                                                                                      |
| -         | -     | -     | -                                                                                                            |
| Linux     | ✔     |  ✔   | Raspberry Pi (`aarch64`), Oracle Linux (RHEL-based), Arch Linux, Rocky Linux, Ubuntu Server 20.04 and above. |
| Darwin    | ✔     |  ✔   | macOS Sequoia 15.2 `x64` on MacBook and the `arm64` in GitHub Actions.                                       |
| BSD       | ✔     |       | OpenBSD 7.6 and FreeBSD 14.2.                                                                                |
| Windows   | ✔     |       | Windows 10 and 11.                                                                                           |

### Unix-based

Run the command in the console to quickly install or update the stable version for Linux, macOS or the BSD-based system:

```shell
curl -sS https://raw.githubusercontent.com/Lifailon/lazyjournal/main/install.sh | bash
```

> [!NOTE]
> This command will run a script that will download the latest executable binary (auto-detect OS and arch) from the GitHub repository to your home directory along with other executables (default path is `~/.local/bin/`) and configurations (`~/.config/lazyjournal/`) for the current user, and also grant execute permission.

### Debian-based

If you are using Ubuntu or any other Debian-based system, you can also download the `deb` package to manage installation and removal:

```bash
VERSION=$(curl -sSL -H 'Accept: application/json' https://github.com/Lifailon/lazyjournal/releases/latest | sed -e 's/.*"tag_name":"\([^"]*\)".*/\1/')
curl -L -sS https://github.com/Lifailon/lazyjournal/releases/download/$VERSION/lazyjournal-$VERSION-$(dpkg --print-architecture).deb -o /tmp/lazyjournal.deb
sudo apt install /tmp/lazyjournal.deb && rm /tmp/lazyjournal.deb
```

### Homebrew (macOS / Linux)

Use the following command to install `lazyjournal` using [Homebrew](https://formulae.brew.sh/formula/lazyjournal):

```shell
brew install lazyjournal
```

### Conda / mamba / pixi (Linux / macOS / Windows)

If you use package managers like conda or mamba, you can install `lazyjournal` from [conda-forge](https://anaconda.org/conda-forge/lazyjournal):

```shell
conda install -c conda-forge lazyjournal
mamba install -c conda-forge lazyjournal
```

You can install `lazyjournal` user-globally using [pixi](https://prefix.dev):

```shell
pixi global install lazyjournal
```

### Arch Linux

If you an Arch Linux user you can also install from the [AUR](https://aur.archlinux.org/packages/lazyjournal):

```shell
paru -S lazyjournal
```

### Docker (Debian-based)

Download the [docker-compose](/docker-compose.yml) file and run the container using the image from [Docker Hub](https://hub.docker.com/r/lifailon/lazyjournal):

```shell
mkdir lazyjournal && cd lazyjournal
curl https://raw.githubusercontent.com/Lifailon/lazyjournal/refs/heads/main/docker-compose.yml -o docker-compose.yml
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

You can also use Go to install ([Go](https://go.dev/doc/install) must be installed on the system):

```shell
go install github.com/Lifailon/lazyjournal@latest
```

## Usage

You can run the interface from anywhere: `lazyjournal`

Access to all system logs and containers may require elevated privileges for the current user. For example, if a user does not have read permission to the directory `/var/lib/docker/containers`, he will not be able to access all archived logs from the moment the container is started, but only from the moment the containerization system is started, so the process of reading logs is different. However, reading in streaming mode is faster than parsing json logs from the file system.

Information in the subtitle of the `Logs` window (overridden by flags and hotkeys):

- `tail` - maximum number of log lines to output (affects log loading performance).
- `auto-update (interval)` - current mode of operation for automatic display of new events (disabled by manually scrolling the log output or using the `Ctrl+U` keyboard shortcut) and update interval (file logs are updated only when there are changes).
- `docker` - displays the current mode for loading the container log (stream mode from the docker api or in json format from the file system) and stream display mode (all, stdout or stderr only).
- `color` - displays the status (enabled or disabled) of the output coloring for the log.

Hotkeys and settings values ​​can be overridden using the [config](/config.yml) file (see issue [#23](https://github.com/Lifailon/lazyjournal/issues/23) and [#27](https://github.com/Lifailon/lazyjournal/issues/27)), which can be in `~/.config/lazydocker/config.yml`, as well as next to the executable or in the current startup directory (has high priority).

### Flags

`lazyjournal -h`

```
--help, -h                 Show help
--version, -v              Show version
--config, -g               Show configuration of hotkeys and settings (check values)
--audit, -a                Show audit information
--tail, -t                 Change the number of log lines to output (range: 200-200000, default: 50000)
--update, -u               Change the auto refresh interval of the log output (range: 2-10, default: 5)
--filter-symbols, -F       Minimum number of symbols for filtering output (range: 1-10, default: 3)
--disable-autoupdate, -d   Disable streaming of new events (log is loaded once without automatic update)
--disable-mouse, -m        Disable mouse control support
--disable-timestamp, -i    Disable timestamp for Docker logs
--only-stream, -o          Force reading of Docker container logs in stream mode (by default from the file system)
--docker-context, -D       Use the specified Docker context (default: default)
--kubernetes-context, -K   Use the specified Kubernetes context (default: default)
--namespace, -n            Use the specified Kubernetes namespace (default: all)
--path, -p                 Custom path to logs in the file system (e.g. "$(pwd)", default: /opt)
--color, -C                Color mode (available values: default, tailspin, bat or disable)
--command-color, -c        ANSI coloring in command line mode
--command-fuzzy, -f        Filtering using fuzzy search in command line mode
--command-regex, -r        Filtering using regular expression (regexp) in command line mode
--ssh, -s                  Connect to remote host (use standard ssh options, separated by spaces in quotes)
                           Example: lazyjournal --ssh "lifailon@192.168.3.101 -p 22"
```

### Hotkeys

List of all used keys and hotkeys (default values):

- `F1` - show help on hotkeys.
- `Up`/`PgUp`/`k` and `Down`/`PgDown`/`j` - move up and down through all journal lists and log output,  as well as changing the filtering mode in the filter window.
- `Shift`/`Alt`+`Up`/`Down` - quickly move up and down through all journal lists and log output every `10` or `100` lines (`500` for log output).
- `Shift`/`Ctrl`+`k`/`j` - quickly move up and down (like Vim and alternative for macOS from config).
- `Left`/`[`/`h` and `Right`/`]`/`l` - switch between journal lists in the selected window.
- `Tab` - switch to next window.
- `Shift+Tab` - return to previous window.
- `Enter` - load a log from the list window or return to the previous window from the filter window.
- `/` - go to the filter window from the current list window or logs window.
- `End`/`Ctrl+E` - go to the end of the log.
- `Home`/`Ctrl+A` - go to the top of the log.
- `Ctrl`+`X`/`Z` - change the number of log lines to output (range: `200-200000`, default: `50000`).
- `Ctrl`+`P`/`O` - change the auto refresh interval of the log output (range: `2-10`, default: `5`).
- `Ctrl`+`U` - disable streaming of new events (log is loaded once without automatic update).
- `Ctrl`+`R` - update the current log output manually (relevant in disable streaming mode).
- `Ctrl`+`Q` - update all log lists.
- `Ctrl`+`W` - switch color mode between `default`, `tailspin`, `bat` or `disable`.
- `Ctrl`+`D` - change read mode for docker logs (stream only or json from file system).
- `Ctrl`+`S` - change stream display mode for docker logs (all, stdout or stderr only).
- `Ctrl`+`T` - enable or disable built-in timestamp and stream type for docker logs.
- `Ctrl`+`C` - clear input text in the filter window or exit.

Mouse control is supported (but can also be disabled with the `-m` flag or configuration) for selecting window and the log from list, as well as lists and log scrolling. To copy text, use the `Alt+Shift` key combination while selecting.

### Remote mode

Supports access to logs on a remote system (no client installation required).

Standard `ssh` arguments are used to configure the connection (passed as a single argument in quotes), for example:

```bash
lazyjournal --ssh "lifailon@192.168.3.101"
# Passing arguments
lazyjournal --ssh "lifailon@192.168.3.101 -p 21 -o Compression=yes"
# If sudo is supported without entering a password
lazyjournal --ssh "lifailon@192.168.3.101 sudo"
```

> [!IMPORTANT]
> Remote access is only possible using an ssh key (password access is **not supported**, as each function request will require entering a password).

### Command-line mode

Coloring and/or filtering of output is supported in command line mode:

```shell
alias lj=lazyjournal # >> $HOME/.bashrc

# Coloring the output from stdin
cat /var/log/syslog | lj -c

# Filtering in fuzzy search
cat /var/log/syslog | lj -f "error"

# Filtering using regular expressions
cat /var/log/syslog | lj -r "error|fatal|fail|crash"

# Filtering with subsequent coloring of the output
cat /var/log/syslog | lj -r "error|fatal|fail|crash" -c
```

## Build

Clone the repository and use Make to run or build the binary:

```shell
git clone https://github.com/Lifailon/lazyjournal
cd lazyjournal
make run
# or
make build
```

## Testing

Unit tests cover all main functions and interface operation.

```shell
# Get a list of all tests
make list
# Run selected or all tests
make test n=TestMockInterface
make test-all
```

> [!NOTE]
> A detailed report on test coverage using CI Actions for Linux, macOS and Windows systems is available on the [Wiki](https://github.com/Lifailon/lazyjournal/wiki) page.

Check the source code on the base linters using [golangci-lint](https://github.com/golangci/golangci-lint) (including all [critic](https://github.com/go-critic/go-critic) and severity high in [security](https://github.com/securego/gosec)):

```shell
make lint-install
make lint
```

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