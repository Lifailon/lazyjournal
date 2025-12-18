# ROADMAP

Backlog tasks after `0.8.3` release:

<!--
- [ ] Изменить фильтрацию по времени и дате на динамическое изменение значения.
- [ ] Добавить список всех ресурсов Kubernetes для отображения events.
- [ ] Обновлять списки в фоне для актуализации статусов.
- [ ] Добавить вложенный список контейнеров в список стеков `compose`.
- [ ] Добавить размер в список лог-файлов и режим сортировки.
- [ ] Режим чтения нескольких файловых журналов (выбор по нажатию `space`) в реальном времени, без отображения истории.
- [ ] Режим `_all` для чтения все системных логов Linux из `journald`, без фильтрации по юниту.
- [ ] Фильтрация по priority для логов из `journald`.
- [ ] Добавить вывод `json` в табличном формате и режим фильтрации по `level` как в `Dozzle`.
- [ ] Активировать строки контекста как в `VSCode`, что бы выводить 1, 2 или больше строк сверху и снизу от найденого слова.
- [ ] Wrap режим для вывода.
- [ ] Интерфейс context manager для Docker и Kubernetes.
- [ ] Интерфейс ssh подключений со списком хостов в конфигурации (новая структура `ssh.hosts`) на `F2`.
- [ ] Переписать логику чтение логов (загружать журнал целиком только первый раз и добавлять изменения) и удалить `disableFastMode`.
- [ ] Управление docker containers и `compose` (перезапуск, остановка, доступ к терминалу, like `lazydocker`) и сервисами `systemd`.
- [ ] Изменение `compose` и `env` (like `Dockge`), а также unit файлов.
- [ ] Фильтрация лог-файлов по дате и размеру с поддержкой удаления.
- [ ] Редактирование ресурсов Kubernetes и перезапуск подов.
- [ ] Чтение системных логов macOS.
- [ ] Анализ логов с помощью ИИ.
- [ ] Покрытие тестами свыше 80%.
-->

- [ ] Change time and date filtering to dynamically change the value.
- [ ] Add a list of all Kubernetes resources to display events.
- [ ] Update lists in the background to keep statuses up-to-date.
- [ ] Add a nested list of containers to the `compose` stack list.
- [ ] Add size to the log file list and sorting mode.
- [ ] Read multiple file logs (select by pressing `space`) in real time, without displaying history.
- [ ] `_all` mode for reading all Linux system logs from `journald`, without filtering by unit.
- [ ] Filter by priority for logs from `journald`.
- [ ] Add `json` output in tabular format and filtering mode by `level`, like in `Dozzle`.
- [ ] Enable context lines like in `VSCode` to output 1, 2, or more lines above and below the found word.
- [ ] Wrap mode for output.
- [ ] Context manager interface for Docker and Kubernetes.
- [ ] SSH connection interface with a host list in the configuration (new `ssh.hosts` structure) on `F2`.
- [ ] Rewrite log reading logic (download the entire log only the first time and append changes) and remove `disableFastMode`.
- [ ] Manage Docker containers and `compose` (restart, stop, access the terminal, like `lazydocker`) and `systemd` services.
- [ ] Modify `compose` and `env` (like `Dockge`), as well as unit files.
- [ ] Filter log files by date and size with deletion support.
- [ ] Editing Kubernetes resources and restarting pods.
- [ ] Reading macOS system logs.
- [ ] AI-powered log analysis.
- [ ] Test coverage over 80%.