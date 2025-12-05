# ROADMAP

Backlog tasks after `0.8.3` release:

<!--
- [ ] Режим `_all` для чтения все системных логов Linux из `journald`, без фильтрации по юниту.
- [ ] Режим чтение системных логов macOS в реальном времени.
- [ ] Активировать строки контекста как в VSCode, что бы выводить 1, 2 или больше строк сверху и снизу от найденого слова.
- [ ] Интерфейс context manager для Docker и Kubernetes.
- [ ] Интерфейс ssh подключений со списком хостов в конфигурации (новая структура `ssh.hosts`) на `F2`.
- [ ] Фильтрация по priority для логов из `journald`.
- [ ] Режим чтения нескольких журналов для файлов (выбор по нажатию `space`, без вывода истории).
- [ ] Управление docker containers и `compose` (перезапуск, остановка, доступ к терминалу, like `lazydocker`) и сервисами `systemd`.
- [ ] Изменение `compose` и `env` (like `Dockge`), а также unit файлов.
- [ ] Фильтрация лог-файлов по дате и размеру с поддержкой удаления.
- [ ] Переписать логику чтение логов (загружать журнал целиком только первый раз и добавлять изменения).
- [ ] Добавить режим фильтрации по `level` как в `Dozzle` (парсить тело `JSON`).
- [ ] Добавить режим поиска по журналу, вместо фильтрации (подводить к нужной строке).
- [ ] Анализ логов с помощью ИИ.
- [ ] Покрытие тестами до 80%+ (добавить `compose`, запустить Kubernetes pods, ssh mode, `auditd`, фильтрация по timestamp).
-->

- [ ] `_all` mode for reading all Linux system logs from `journald`, without filtering by unit.
- [ ] Real-time reading mode for macOS system logs.
- [ ] Activate context lines like in VSCode to display 1, 2, or more lines above and below a found word.
- [ ] Context manager interface for Docker and Kubernetes.
- [ ] SSH connection interface with a host list in the configuration (new `ssh.hosts` structure) on `F2`.
- [ ] Priority filtering for logs from `journald`.
- [ ] Reading multiple logs for files (selection by pressing `space`, without history display).
- [ ] Managing Docker containers and `compose` (restart, stop, access to the terminal, like `lazydocker`) and `systemd` services.
- [ ] Modified `compose` and `env` (like `Dockge`), as well as unit files.
- [ ] Filter log files by date and size with deletion support.
- [ ] Rewrite log reading logic (download entire logs only the first time and append changes).
- [ ] Add filtering by `level` like in `Dozzle` (parse the `JSON` body).
- [ ] Add log search mode instead of filtering (hover over the desired line).
- [ ] AI-powered log analysis.
- [ ] Increase test coverage to 80%+ (add `compose`, start Kubernetes pods, ssh mode, `auditd`, filter by timestamp).