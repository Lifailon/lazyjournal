## Release 0.8.3

### New features

- Added HTTP response status codes coloring.
- Added simultaneous coloring and filtering in CLI mode.
- Added settings for color and startup parameter for filtering by date.
- Added color actions disable via settings.
- Added new settings for config and view mode for filtering by date in subtitle.
- Timestamp filter mode has been replaced with a date filter with a value switch.
- Added check connection to the Kubernetes cluster.
- Added minimum symbol in flag and config for filter.
- Added JSON coloring.
- Added bat mode and binary check.

### Fixes

- Fixed filter by date range functionality.
- Fixed update status for filter by date.
- Fixed frame and title color when loading.

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
