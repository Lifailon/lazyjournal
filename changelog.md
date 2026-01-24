## Release 0.8.4

### New features

- Added AI-powered pipelines for PR and commit review.
- Introduced automated AI analysis for releases and issues.
- Implemented the ability to send build, CI, release, and issue reports to Telegram.
- Added line reduction feature for tail mode in status view; updated default line value to 40,000 for tail.
- Provided additional flags in configuration, such as disabling wrap mode and timestamp, as well as handling list of SSH hosts and prefix options for container names.
- Enhanced container log retrieval capabilities, including for all containers and with improved pod/container name prefixing.
- Expanded support for redrawing windows when changing OS.
- Improved error handling for SSH connection and output processing.
- Added base interface and context handling for manager, including nextView and status management.
- Introduced hotkey support and functionality to fill out context and namespace for manager.
- Added new viewing options for build paths and status views.
- Enabled new jobs, run-names, and pipeline organization within workflows.
- Provided docker and compose context usage in SSH mode.
- Implemented timeout features for SSH mode and for docker/compose commands.
- Enhanced journalctl command handling with service name arguments and "all mode" for logs.
- Added priority indicators for commands and improved status color coding.
- Extended analysis with a specific tag parameter and updated AI model (GPT-4.1) for issue analysis.
- Added documentation on DeepWiki and improved config path handling.
- Added interface for managing host commands in SSH mode status.

### Fixes

- Resolved coloring issues for container names, statuses, and service name arguments in compose and journalctl commands.
- Fixed comment formatting for linter checks.
- Addressed processing and handling of SSH connection errors.
- Corrected Telegram notifications for issue alerts and improved messaging for HTML issues.
- Fixed switch behavior for Kubernetes namespace changes.
- Amended default key handling for tail and update features.
- Corrected status titles and fixed status logic for improved reliability.
- Removed outdated roadmap documentation.
- Ordered release report addition for better workflow clarity.
- Improved status updates and prefix handling in compose and Kubernetes environments.


## Release 0.8.3

### New features

- Added coloring for HTTP response status codes and improved HTTP path coloring.
- Introduced settings for color options and a startup parameter for date filtering.
- Added the ability to disable color actions via settings.
- Implemented new settings and configuration options for view mode and filtering by date in subtitles.
- Replaced the timestamp filter mode with a date filter that includes value switching.
- Simultaneous coloring and filtering now supported in CLI mode.
- Added a check for connection to Kubernetes clusters.
- Added 'bat' mode and binary checks.
- JSON coloring support added.
- Added minimal symbol options for flags and configs related to filtering.
- Improved keyword and status coloring.
- Enhanced CLI mode checks.
- Updated hotkey bindings for filtering and color mode.

### Fixes

- Fixed filtering by date range functionality.
- Updated status updates for date filtering.
- Fixed frame and title color rendering when loading.
- Improved tests and log file updates related to coloring.
- Removed tcpdump and refined keyword coloring.
- Revised color mode and bat mode testing.
- Checked and removed root directory references in coloring logic.

## Release 0.8.2

### New features

- Added all commit history for Git clone.
- Added Docker build support for old tags and latest version.
- Enabled kubeconfig support.
- Added license scan report and badge.
- Updated playground to fix compose and add k3s demo.
- Updated remote commands and added remote debug capability.
- Added support for ARM64 architecture.
- Added new arguments and options for containers.
- Added profiling ignore feature.
- Added new app options in containers.
- Added option to disable services in unit list.
- Added new settings for default flag values.
- Updated service status handling in unit list.
- Added use of custom context for compose services.
- Added display of current and count context/namespace in audit.
- Added selection of Docker context.
- Added switch namespace and context for Kubernetes logs.
- Added check for compose binary existence.
- Added custom coloring via configuration, updated related config options.
- Added custom path in configuration and as a flag.
- Updated playground to demonstrate compose and active logging.
- Updated bug report install method.
- Updated Docker commands.

### Fixes

- Fixed OPT path handling.
- Fixed and removed environment variables from Docker Compose configuration.
- Fixed compose: moved environment variables and added options.
- Fixed compose service name switching and cursor time updates.
- Fixed status coloring for compose and pods; updated status color in Docker/Compose.
- Fixed audit logic and added restart containers in compose counter.
- Fixed linters issues, updated golangci-lint configuration.
- Fixed forcetypeassert linter issue.
- Fixed default values for custom path flag.
- Updated audit example to handle contexts and namespaces.

## Release 0.8.1

### New features

- Added commands for container control.
- Introduced linters checks in the final report (also applied for wiki).
- Added verbose option for linters check.
- Added initialization for color map and update for color array in static compose configuration.
- Provided examples for kubeconfig and audit.
- Added support for Docker Compose information, stack logs, and filtering by timestamps.
- Enabled new log list for Docker Compose stacks.
- Added unique prefix coloring for containers and improved coloring for containers and pods status.
- Added playground scripts and configuration (Killercoda playground).
- Provided parameters for debugging and fast configuration options.
- Added force commit option for wiki and upload all report functionality.
- Enabled compose information in audit and forward kubectl config examples.
- Improved log output and clear filter functionality.
- Added return window for clear input events.

### Fixes

- Fixed mount for kubeconfig path.
- Resolved errors in Docker unit tests.
- Fixed kubectl issues in audit.
- Addressed error when changing log after compose operations.
- Fixed push and clone actions for wiki repository.
- Resolved branch and URL issues for wiki integration.
- Fixed path and merge problems during wiki updates.
- Resolved error when pulling and merging to the wiki.
- Fixed handling of nil elements in lists when using mouse.
- Improved coloring for prefix container names in compose logs.
- Updated version to 0.8.1 and improved linters execution.
- Corrected table reports and merging reports for testing.
- Improved error handling for pulling, pushing, cloning, and updating the wiki.
- Removed branch requirement for wiki commit operations.
- Updated report table formatting and params for various actions.

## Release 0.8.0

### New features

- Added debug mode for advanced report.  
- Coverage markdown report updated to include table and percent information.  
- Output in TTY for logging tests.  
- Docker Hub deploy merged into CI.  
- Build system updated to remove Snap package and use Docker buildx (multi-arch support).  
- Added download config functionality in install scripts.  
- Added mount config option.  
- Fast mode (demo) added.  
- Enabled copying source code via SSH and running code remotely.  
- Filtering lists by time enhanced and added last null line.  
- Multithreaded build introduced.  
- Added new flag for test and updated path logic.  
- Short output format for filtering by timestamp introduced.  
- Added notifications for info and error events.  
- Comprehensive settings for all flags and structure for configuration added.  
- Config file extended to manage hotkeys (switch, goto, up/down, left/right, closing with ESC, and Ctrl+C clear).  
- Hotkeys configuration now uses YAML and supports structure for future extensibility.  
- Color support for K3s logs and reading K8s pod logs across all namespaces.  
- SSH mode for Docker containers introduced (with args and options).  
- Stats for multiple files, remote OS detection, and custom stats from remote files added.

### Fixes

- TTYD and Docker CLI fixes for ARM architecture.
- Entrypoint logic fixed.
- Improved report table output presentation.
- Logging fixes including for testing scenarios.
- Timeout and shell reliability improved.
- Variable handling during hash operations fixed.
- Build process robustness increased and coverage info clarified.
- Linters corrected and improved.
- Install script paths corrected.
- Path priority logic for config improved.
- Flags priority logic fixed.
- Dockerfile updated and old Dockerfile removed.
- Improved visibility of config and info windows for tests.
- Config file priority logic refined and error handling for flags fixed.
- Reading Kubernetes pod logs from any namespace fixed.
- Other minor fixes for SSH mode and associated configuration.
