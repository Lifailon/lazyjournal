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
