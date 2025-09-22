package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/awesome-gocui/gocui"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"gopkg.in/yaml.v3"
)

var programVersion string = "0.8.0"

// Структура конфигурации
type Config struct {
	Hotkeys  Hotkeys  `yaml:"hotkeys"`
	Settings Settings `yaml:"settings"`
}

// Структура доступных сочетаний клавиш для переопределения (#23)
type Hotkeys struct {
	Help                 string `yaml:"help"`
	Up                   string `yaml:"up"`
	QuickUp              string `yaml:"quickUp"`
	VeryQuickUp          string `yaml:"veryQuickUp"`
	SwitchFilterMode     string `yaml:"switchFilterMode"`
	Down                 string `yaml:"down"`
	QuickDown            string `yaml:"quickDown"`
	VeryQuickDown        string `yaml:"veryQuickDown"`
	BackSwitchFilterMode string `yaml:"backSwitchFilterMode"`
	Left                 string `yaml:"left"`
	Right                string `yaml:"right"`
	SwitchWindow         string `yaml:"switchWindow"`
	BackSwitchWindows    string `yaml:"backSwitchWindows"`
	LoadJournal          string `yaml:"loadJournal"`
	GoToFilter           string `yaml:"goToFilter"`
	GoToEnd              string `yaml:"goToEnd"`
	GoToTop              string `yaml:"goToTop"`
	TailModeMore         string `yaml:"tailModeMore"`
	TailModeLess         string `yaml:"tailModeLess"`
	UpdateIntervalMore   string `yaml:"updateIntervalMore"`
	UpdateIntervalLess   string `yaml:"updateIntervalLess"`
	AutoUpdateJournal    string `yaml:"autoUpdateJournal"`
	UpdateJournal        string `yaml:"updateJournal"`
	UpdateLists          string `yaml:"updateLists"`
	ColorDisable         string `yaml:"colorDisable"`
	TailspinEnable       string `yaml:"tailspinEnable"`
	SwitchDockerMode     string `yaml:"switchDockerMode"`
	SwitchStreamMode     string `yaml:"switchStreamMode"`
	TimestampShow        string `yaml:"timestampShow"`
	Exit                 string `yaml:"exit"`
}

// Структура доступных параметров для переопределения значений по умолчанию при запуске (#27)
type Settings struct {
	TailMode          string `yaml:"tailMode"`
	UpdateInterval    string `yaml:"updateInterval"`
	DisableAutoUpdate string `yaml:"disableAutoUpdate"`
	DisableColor      string `yaml:"disableColor"`
	DisableMouse      string `yaml:"disableMouse"`
	DisableTimestamp  string `yaml:"disableTimestamp"`
	OnlyStream        string `yaml:"onlyStream"`
	DisableFastMode   string `yaml:"disableFastMode"`
}

// Структура хранения информации о журналах
type Journal struct {
	name    string // название журнала (имя службы) или дата загрузки
	boot_id string // id загрузки системы
}

type Logfile struct {
	name string
	path string
}

type DockerContainers struct {
	name      string
	id        string
	namespace string
}

// Структура для парсинга логов из docker cli
type dockerLogLines struct {
	isError   bool
	timestamp time.Time
	content   string
}

// Основная структура приложения (графический интерфейс и данные журналов)
type App struct {
	gui *gocui.Gui // графический интерфейс (gocui)

	sshMode             bool     // использовать вызов команд (exec.Command) через ssh
	sshOptions          []string // опции для ssh подключения
	fastMode            bool     // загрузка журналов в горутине (beta mode)
	testMode            bool     // исключаем вызовы к gocui при тестирование функций
	tailSpinMode        bool     // режим покраски через tailspin
	tailSpinBinName     string   // название исполняемого файла (tailspin/tspin)
	colorMode           bool     // отключение/включение покраски ключевых слов
	mouseSupport        bool     // отключение/включение поддержки мыши
	dockerStreamLogs    bool     // принудительное чтение журналов контейнеров Docker из потоков (по умолчанию, чтение происходит из файловой системы, если есть доступ)
	dockerStreamLogsStr string   // отображаемый режим чтения журнала Docker (в зависимости от прав доступа и флага)
	dockerStreamMode    string   // переменная для хранения режима чтения потоков (all, stdout или stderr)

	getOS         string   // название ОС
	getArch       string   // архитектура процессора
	hostName      string   // текущее имя хоста для покраски в логах
	userName      string   // текущее имя пользователя
	systemDisk    string   // порядковая буква системного диска для Windows
	userNameArray []string // список всех пользователей
	rootDirArray  []string // список всех корневых каталогов

	selectUnits                  string // название журнала (UNIT/USER_UNIT/kernel/audit)
	selectPath                   string // путь к логам (/var/log/)
	selectContainerizationSystem string // название системы контейнеризации (docker/compose/podman/kubernetes)
	selectFilterMode             string // режим фильтрации (default/fuzzy/regex)

	logViewCount     string   // количество логов для просмотра
	logUpdateSeconds int      // период фонового обновления журнала
	secondsChan      chan int // канал для изменения интервала обновления в горутине

	journals           []Journal // список (массив/срез) журналов для отображения
	maxVisibleServices int       // максимальное количество видимых элементов в окне списка служб
	startServices      int       // индекс первого видимого элемента
	selectedJournal    int       // индекс выбранного журнала

	logfiles        []Logfile
	maxVisibleFiles int
	startFiles      int
	selectedFile    int

	dockerContainers           []DockerContainers
	maxVisibleDockerContainers int
	startDockerContainers      int
	selectedDockerContainer    int

	// Фильтрация по времени
	timestampFilterView      bool   // отображение окон
	sinceTimestampFilterMode bool   // использовать режим фильтрации для since
	untilTimestampFilterMode bool   // использовать режим фильтрации для until
	sinceFilterText          string // начало отрезка времени
	untilFilterText          string // конец отрезка времени

	// Текст для фильтрации список журналов
	filterListText string

	// Массивы для хранения списка журналов без фильтрации
	journalsNotFilter         []Journal
	logfilesNotFilter         []Logfile
	dockerContainersNotFilter []DockerContainers

	// Переменные для отслеживания изменений размера окна
	windowWidth  int
	windowHeight int

	filterText       string   // текст для фильтрации записей журнала
	currentLogLines  []string // набор строк (срез) для хранения журнала без фильтрации
	filteredLogLines []string // набор строк (срез) для хранения журнала после фильтра
	logScrollPos     int      // позиция прокрутки для отображаемых строк журнала
	lastFilterText   string   // фиксируем содержимое последнего ввода текста для фильтрации

	autoScroll        bool   // используется для автоматического скроллинга вниз при обновлении (если это не ручной скроллинг)
	disableAutoScroll bool   // отключение автоматического обновления вывода
	lastUpdateLine    string // фиксируем предпоследнюю строку для делимитра
	updateTime        string // фиксируем время загрузки журнала для делимитра

	lastDateUpdateFile time.Time // последняя дата изменения файла
	lastSizeFile       int64     // размер файла
	updateFile         bool      // проверка для обновления вывода в горутине (отключение только если нет изменений в файле и для Windows Event)

	lastWindow   string // фиксируем последний используемый источник для вывода логов
	lastSelected string // фиксируем название последнего выбранного журнала или контейнера

	// Переменные для хранения значений автообновления вывода при смене окна
	lastSelectUnits            string
	lastBootId                 string
	lastLogPath                string
	lastContainerizationSystem string
	lastContainerId            string

	// Цвета окон по умолчанию (изменяется в зависимости от доступности журналов)
	journalListFrameColor gocui.Attribute
	fileSystemFrameColor  gocui.Attribute
	dockerFrameColor      gocui.Attribute

	// Фиксируем последнее время загрузки и покраски журнала
	debugStartTime time.Time
	debugLoadTime  string
	debugColorTime string

	// Отключение привязки горячих клавиш на время загрузки списка
	keybindingsEnabled bool

	// Отключение встроенных временных меток (timestamp) для логов Docker
	timestampDocker  bool
	streamTypeDocker bool

	// Регулярные выражения для покраски строк
	trimHttpRegex        *regexp.Regexp
	trimHttpsRegex       *regexp.Regexp
	trimPrefixPathRegex  *regexp.Regexp
	trimPostfixPathRegex *regexp.Regexp
	hexByteRegex         *regexp.Regexp
	dateTimeRegex        *regexp.Regexp
	integersInputRegex   *regexp.Regexp
	syslogUnitRegex      *regexp.Regexp

	lastCurrentView string // фиксируем последнее используемое окно для Esc после /
	backCurrentView bool   // отключаем/ключаем возврат

	uniquePrefixColorMap map[string]string // карта для хранения уникального цвета для каждого контейнера в стеках compose
	uniquePrefixColorArr []string          // массив для хранения уникальных цветов
}

func showHelp() {
	fmt.Println("lazyjournal - A TUI for reading logs from journald, auditd, file system, Docker containers, Podman and Kubernetes pods.")
	fmt.Println("Source code: https://github.com/Lifailon/lazyjournal")
	fmt.Println("If you have problems with the application, please open issue: https://github.com/Lifailon/lazyjournal/issues")
	fmt.Println("")
	fmt.Println("  Flags:")
	fmt.Println("    --help, -h                 Show help")
	fmt.Println("    --version, -v              Show version")
	fmt.Println("    --config, -g               Show configuration of hotkeys and settings (check values)")
	fmt.Println("    --audit, -a                Show audit information")
	fmt.Println("    --tail, -t                 Change the number of log lines to output (range: 200-200000, default: 50000)")
	fmt.Println("    --update, -u               Change the auto refresh interval of the log output (range: 2-10, default: 5)")
	fmt.Println("    --disable-autoupdate, -e   Disable streaming of new events (log is loaded once without automatic update)")
	fmt.Println("    --disable-color, -d        Disable output coloring")
	fmt.Println("    --disable-mouse, -m        Disable mouse control support")
	fmt.Println("    --disable-timestamp, -p    Disable timestamp for docker logs")
	fmt.Println("    --only-stream, -o          Force reading of docker container logs in stream mode (by default from the file system)")
	fmt.Println("    --command-color, -c        ANSI coloring in command line mode")
	fmt.Println("    --command-fuzzy, -f        Filtering using fuzzy search in command line mode")
	fmt.Println("    --command-regex, -r        Filtering using regular expression (regexp) in command line mode")
	fmt.Println("    --ssh, -s                  Connect to remote host (use standard ssh options, separated by spaces in quotes)")
	fmt.Println("                               Example: lazyjournal --ssh \"lifailon@192.168.3.101 -p 22\"")
	fmt.Println()
}

// Confi (#23)
func showConfig() {
	// Читаем конфигурацию (извлекаем путь и ошибки)
	configPath, err := config.getConfig()

	fmt.Println("path:", configPath)
	fmt.Println("---")

	// Проверяем конфигурацию на ошибки
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Выводим содержимое конфигурации
	// fmt.Println(string(configData))
	// Выводим полученные значения из конфигурации (форматированный вывод) с проверкой на пустые значения
	fmt.Println("hotkeys:")
	fmt.Printf("  help:                  %s\n", config.Hotkeys.Help)
	fmt.Printf("  up:                    %s\n", config.Hotkeys.Up)
	fmt.Printf("  quickUp:               %s\n", config.Hotkeys.QuickUp)
	fmt.Printf("  veryQuickUp:           %s\n", config.Hotkeys.VeryQuickUp)
	fmt.Printf("  switchFilterMode:      %s\n", config.Hotkeys.SwitchFilterMode)
	fmt.Printf("  down:                  %s\n", config.Hotkeys.Down)
	fmt.Printf("  quickDown:             %s\n", config.Hotkeys.QuickDown)
	fmt.Printf("  veryQuickDown:         %s\n", config.Hotkeys.VeryQuickDown)
	fmt.Printf("  backSwitchFilterMode:  %s\n", config.Hotkeys.BackSwitchFilterMode)
	fmt.Printf("  left:                  %s\n", config.Hotkeys.Left)
	fmt.Printf("  right:                 %s\n", config.Hotkeys.Right)
	fmt.Printf("  switchWindow:          %s\n", config.Hotkeys.SwitchWindow)
	fmt.Printf("  backSwitchWindows:     %s\n", config.Hotkeys.BackSwitchWindows)
	fmt.Printf("  loadJournal:           %s\n", config.Hotkeys.LoadJournal)
	fmt.Printf("  goToFilter:            %s\n", config.Hotkeys.GoToFilter)
	fmt.Printf("  goToEnd:               %s\n", config.Hotkeys.GoToEnd)
	fmt.Printf("  goToTop:               %s\n", config.Hotkeys.GoToTop)
	fmt.Printf("  tailModeMore:          %s\n", config.Hotkeys.TailModeMore)
	fmt.Printf("  tailModeLess:          %s\n", config.Hotkeys.TailModeLess)
	fmt.Printf("  updateIntervalMore:    %s\n", config.Hotkeys.UpdateIntervalMore)
	fmt.Printf("  updateIntervalLess:    %s\n", config.Hotkeys.UpdateIntervalLess)
	fmt.Printf("  autoUpdateJournal:     %s\n", config.Hotkeys.AutoUpdateJournal)
	fmt.Printf("  updateJournal:         %s\n", config.Hotkeys.UpdateJournal)
	fmt.Printf("  updateLists:           %s\n", config.Hotkeys.UpdateLists)
	fmt.Printf("  colorDisable:          %s\n", config.Hotkeys.ColorDisable)
	fmt.Printf("  tailspinEnable:        %s\n", config.Hotkeys.TailspinEnable)
	fmt.Printf("  switchDockerMode:      %s\n", config.Hotkeys.SwitchDockerMode)
	fmt.Printf("  switchStreamMode:      %s\n", config.Hotkeys.SwitchStreamMode)
	fmt.Printf("  timestampShow:         %s\n", config.Hotkeys.TimestampShow)
	fmt.Printf("  exit:                  %s\n", config.Hotkeys.Exit)

	fmt.Println("settings:")
	fmt.Printf("  tailMode:              %s\n", config.Settings.TailMode)
	fmt.Printf("  updateInterval:        %s\n", config.Settings.UpdateInterval)
	fmt.Printf("  disableColor:          %s\n", config.Settings.DisableColor)
	fmt.Printf("  disableAutoUpdate:     %s\n", config.Settings.DisableAutoUpdate)
	fmt.Printf("  disableMouse:          %s\n", config.Settings.DisableMouse)
	fmt.Printf("  disableTimestamp:      %s\n", config.Settings.DisableTimestamp)
	fmt.Printf("  onlyStream:            %s\n", config.Settings.OnlyStream)
	fmt.Printf("  disableFastMode:       %s\n", config.Settings.DisableFastMode)

	fmt.Println()
}

// Audit (#18) for homebrew
func (app *App) showAudit() {
	var auditText []string
	app.testMode = true
	app.getOS = runtime.GOOS

	auditText = append(auditText,
		"system:",
		"  date: "+time.Now().Format("02.01.2006 15:04:05"),
		"  go: "+strings.ReplaceAll(runtime.Version(), "go", ""),
	)

	data, err := os.ReadFile("/etc/os-release")
	// Если ошибка при чтении файла, то возвращаем только название ОС
	if err != nil {
		auditText = append(auditText, "  os: "+app.getOS)
	} else {
		var name, version string
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "NAME=") {
				name = strings.Trim(line[5:], "\"")
			}
			if strings.HasPrefix(line, "VERSION=") {
				version = strings.Trim(line[8:], "\"")
			}
		}
		auditText = append(auditText, "  os: "+app.getOS+" "+name+" "+version)
	}

	auditText = append(auditText, "  arch: "+app.getArch)

	currentUser, _ := user.Current()
	app.userName = currentUser.Username
	if strings.Contains(app.userName, "\\") {
		app.userName = strings.Split(app.userName, "\\")[1]
	}
	auditText = append(auditText, "  username: "+app.userName)

	if app.getOS != "windows" {
		auditText = append(auditText, "  privilege: "+(map[bool]string{true: "root", false: "user"})[os.Geteuid() == 0])
	}

	execPath, err := os.Executable()
	if err == nil {
		if strings.Contains(execPath, "tmp/go-build") || strings.Contains(execPath, "Temp\\go-build") {
			auditText = append(auditText, "  execType: source code")
		} else {
			auditText = append(auditText, "  execType: binary file")
		}
	}
	auditText = append(auditText, "  execPath: "+execPath)

	if app.getOS == "windows" {
		// Windows Event
		app.loadWinEvents()
		auditText = append(auditText,
			"winEvent:",
			"  logs: ",
			"  - count: "+strconv.Itoa(len(app.journals)),
		)
		// Filesystem
		if app.userName != "runneradmin" {
			app.systemDisk = os.Getenv("SystemDrive")
			if len(app.systemDisk) >= 1 {
				app.systemDisk = string(app.systemDisk[0])
			} else {
				app.systemDisk = "C"
			}
			auditText = append(auditText,
				"fileSystem:",
				"  systemDisk: "+app.systemDisk,
				"  files:",
			)
			paths := []struct {
				fullPath string
				path     string
			}{
				{"Program Files", "ProgramFiles"},
				{"Program Files (x86)", "ProgramFiles86"},
				{"ProgramData", "ProgramData"},
				{"/AppData/Local", "AppDataLocal"},
				{"/AppData/Roaming", "AppDataRoaming"},
			}
			// Создаем группу для ожидания выполнения всех горутин
			var wg sync.WaitGroup
			// Мьютекс для безопасного доступа к переменной auditText
			var mu sync.Mutex
			for _, path := range paths {
				// Увеличиваем счетчик горутин
				wg.Add(1)
				go func(path struct{ fullPath, path string }) {
					// Отнимаем счетчик горутин при завершении выполнения горутины
					defer wg.Done()
					var fullPath string
					if strings.HasPrefix(path.fullPath, "Program") {
						fullPath = "\"" + app.systemDisk + ":/" + path.fullPath + "\""
					} else {
						fullPath = "\"" + app.systemDisk + ":/Users/" + app.userName + path.fullPath + "\""
					}
					app.loadWinFiles(path.path)
					lenLogFiles := strconv.Itoa(len(app.logfiles))
					// Блокируем доступ на завись в переменную auditText
					mu.Lock()
					auditText = append(auditText,
						"  - path: "+fullPath,
						"    count: "+lenLogFiles,
					)
					// Разблокировать мьютекс
					mu.Unlock()
				}(path)
			}
			// Ожидаем завершения всех горутин
			wg.Wait()
		}
	} else {
		// systemd/journald
		auditText = append(auditText,
			"systemd:",
			"  journald:",
		)
		csCheck := exec.Command("journalctl", "--version")
		_, err := csCheck.Output()
		if err == nil {
			auditText = append(auditText,
				"  - installed: true",
				"    journals:",
			)
			journalList := []struct {
				name        string
				journalName string
			}{
				{"Unit list", "services"},
				{"System journals", "UNIT"},
				{"User journals", "USER_UNIT"},
				{"Kernel boot", "kernel"},
			}
			for _, journal := range journalList {
				app.loadServices(journal.journalName)
				lenJournals := strconv.Itoa(len(app.journals))
				auditText = append(auditText,
					"    - name: "+journal.name,
					"      count: "+lenJournals,
				)
			}
		} else {
			auditText = append(auditText, "  - installed: false")
		}
		// Filesystem
		auditText = append(auditText,
			"fileSystem:",
			"  files:",
		)
		paths := []struct {
			name string
			path string
		}{
			{"System var logs", "/var/log/"},
			{"Optional package logs", "/opt/"},
			{"Users home logs", "/home/"},
			{"Process descriptor logs", "descriptor"},
		}
		for _, path := range paths {
			app.loadFiles(path.path)
			lenLogFiles := strconv.Itoa(len(app.logfiles))
			auditText = append(auditText,
				"  - name: "+path.name,
				"    path: "+path.path,
				"    count: "+lenLogFiles,
			)
		}
	}
	auditText = append(auditText,
		"containerization: ",
		"  system: ",
	)
	containerizationSystems := []string{
		"docker",
		"podman",
		"kubernetes",
	}
	for _, cs := range containerizationSystems {
		auditText = append(auditText, "  - name: "+cs)
		if cs == "kubernetes" {
			csCheck := exec.Command("kubectl", "version")
			output, _ := csCheck.Output()
			// По умолчанию у version код возврата всегда 1, по этому проверяем вывод
			if strings.Contains(string(output), "Version:") {
				auditText = append(auditText, "    installed: true")
				// Преобразуем байты в строку и обрезаем пробелы
				csVersion := strings.TrimSpace(string(output))
				// Удаляем текст до номера версии
				csVersion = strings.Split(csVersion, "Version: ")[1]
				// Забираем первую строку
				csVersion = strings.Split(csVersion, "\n")[0]
				auditText = append(auditText, "    version: "+csVersion)
				cmd := exec.Command(
					cs, "get", "pods", "-A",
					"-o", "jsonpath={range .items[*]}{.metadata.uid} {.metadata.name} {.status.phase}{'\\n'}{end}",
				)
				_, err := cmd.Output()
				if err == nil {
					app.loadDockerContainer(cs)
					auditText = append(auditText, "    pods: "+strconv.Itoa(len(app.dockerContainers)))
				} else {
					auditText = append(auditText, "    pods: 0")
				}
			} else {
				auditText = append(auditText, "    installed: false")
			}
		} else {
			csCheck := exec.Command(cs, "--version")
			output, err := csCheck.Output()
			if err == nil {
				auditText = append(auditText, "    installed: true")
				csVersion := strings.TrimSpace(string(output))
				csVersion = strings.Split(csVersion, "version ")[1]
				auditText = append(auditText, "    version: "+csVersion)
				cmd := exec.Command(
					cs, "ps", "-a",
					"--format", "{{.ID}} {{.Names}} {{.State}}",
				)
				_, err := cmd.Output()
				if err == nil {
					app.loadDockerContainer(cs)
					auditText = append(auditText, "    containers: "+strconv.Itoa(len(app.dockerContainers)))
				} else {
					auditText = append(auditText, "    containers: 0")
				}
			} else {
				auditText = append(auditText, "    installed: false")
			}
		}
	}
	for _, line := range auditText {
		fmt.Println(line)
	}
}

// Объявляем конфигурацию
var config Config

// Читаем конфигурацию
func (config *Config) getConfig() (string, error) {
	// Читаем файл конфигурации из текущего каталога
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	configPath := filepath.Join(currentDir, "config.yml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		// Из каталога с исполняемым файлом
		execDir, err := os.Executable()
		if err != nil {
			return "", err
		}
		configPath = filepath.Join(execDir, "config.yml")
		configData, err = os.ReadFile(configPath)
		if err != nil {
			// Из каталога ~/.config/lazyjournal/
			homePath, _ := os.UserHomeDir()
			configPath = filepath.Join(homePath, ".config", "lazyjournal", "config.yml")
			configData, err = os.ReadFile(configPath)
			if err != nil {
				return configPath, ErrConfigNotFound
			}
		}
	}
	// Парсим yaml конфигурации
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		return configPath, fmt.Errorf("%w: %w", ErrYamlSyntax, err)
	}
	return configPath, err
}

// Предварительная компиляция регулярных выражений для покраски вывода и их доступности в тестах
var (
	// Исключаем все до http:// (включительно) в начале строки
	trimHttpRegex = regexp.MustCompile(`^.*http://|([^a-zA-Z0-9:/._?&=+-].*)$`)
	// И после любого символа, который не может содержать в себе url
	trimHttpsRegex = regexp.MustCompile(`^.*https://|([^a-zA-Z0-9:/._?&=+-].*)$`)
	// Иключаем все до первого символа слэша (не включительно)
	trimPrefixPathRegex = regexp.MustCompile(`^[^/]+`)
	// Исключаем все после первого символа, который не должен (но может) содержаться в пути
	trimPostfixPathRegex = regexp.MustCompile(`[=:'"(){}\[\]]+.*$`)
	// Байты или числа в шестнадцатеричном формате: 0x2 || 0xc0000001
	hexByteRegex = regexp.MustCompile(`\b0x[0-9A-Fa-f]+\b`)
	// DateTime: YYYY-MM-DDTHH:MM:SS.MS+HH:MM || YYYY-MM-DDTHH:MM:SS.MSZ
	dateTimeRegex = regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?([+-]\d{2}:\d{2}|Z))\b`)
	// Integers: Int only + Time + MAC address + Percentage (int%) + Date2 (20/03/2025)
	integersInputRegex = regexp.MustCompile(`^[^a-zA-Z]*\d+[^a-zA-Z]*$`)
	// Syslog UNIT
	syslogUnitRegex = regexp.MustCompile(`^[a-zA-Z-_.]+\[\d+\]:$`)
	// Замена пробелов на T для фильтрации по дате+время
	reSpace = regexp.MustCompile(`\s+`)
	// Проверка формата времени (короткий формат)
	filterTimeRegex = regexp.MustCompile(`^[+-]\d+[smhd]$`)
)

// Ошибки
var (
	ErrConfigNotFound = errors.New("configuration file not found")
	ErrYamlSyntax     = errors.New("error yaml syntax in config file")
	ErrSSHConnection  = errors.New("error connecting on SSH to")
	ErrInvalidStat    = errors.New("invalid stat output")
)

// Определяем название удаленной системы
func remoteGetOS(sshOptions []string) (string, error) {
	cmd := exec.Command("ssh", append(sshOptions, "uname", "-s")...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrSSHConnection, sshOptions[0])
	} else {
		return strings.ToLower(string(output)), nil
	}
}

var g *gocui.Gui

func runGoCui(mock bool) {
	// Инициализация значений по умолчанию + компиляция регулярных выражений для покраски
	app := &App{
		sshMode:                      false,
		fastMode:                     true,
		testMode:                     false,
		tailSpinMode:                 false,
		colorMode:                    true,
		mouseSupport:                 true,
		dockerStreamLogs:             false,
		dockerStreamMode:             "all",
		startServices:                0, // начальная позиция списка юнитов
		selectedJournal:              0, // начальный индекс выбранного журнала
		startFiles:                   0,
		selectedFile:                 0,
		startDockerContainers:        0,
		selectedDockerContainer:      0,
		debugLoadTime:                "0s",
		debugColorTime:               "0s",
		selectUnits:                  "services",  // "UNIT" || "USER_UNIT" || "kernel" || "audit"
		selectPath:                   "/var/log/", // "/opt/", "/home/" или "/Users/" (для macOS) + /root/ || "descriptor"
		selectContainerizationSystem: "docker",    // "compose" || "podman" || "kubernetes"
		selectFilterMode:             "default",   // "fuzzy" || "regex" || "timestamp"
		timestampFilterView:          false,
		sinceTimestampFilterMode:     false,
		untilTimestampFilterMode:     false,
		journalListFrameColor:        gocui.ColorDefault,
		fileSystemFrameColor:         gocui.ColorDefault,
		dockerFrameColor:             gocui.ColorDefault,
		autoScroll:                   true,
		trimHttpRegex:                trimHttpRegex,
		trimHttpsRegex:               trimHttpsRegex,
		trimPrefixPathRegex:          trimPrefixPathRegex,
		trimPostfixPathRegex:         trimPostfixPathRegex,
		hexByteRegex:                 hexByteRegex,
		dateTimeRegex:                dateTimeRegex,
		integersInputRegex:           integersInputRegex,
		syslogUnitRegex:              syslogUnitRegex,
		keybindingsEnabled:           true,
		timestampDocker:              true,
		streamTypeDocker:             true,
		lastCurrentView:              "services",
		backCurrentView:              false,
	}

	app.uniquePrefixColorMap = make(map[string]string)
	app.uniquePrefixColorArr = append(app.uniquePrefixColorArr,
		"\033[32m", // Зеленый
		"\033[33m", // Желный
		"\033[34m", // Синий
		"\033[35m", // Пурпурный
		"\033[36m", // Голубой
	)

	// Определяем используемую ОС (linux/darwin/*bsd/windows) и архитектуру
	app.getOS = runtime.GOOS
	app.getArch = runtime.GOARCH

	// Аргументы
	help := flag.Bool("help", false, "Show help")
	flag.BoolVar(help, "h", false, "Show help")
	version := flag.Bool("version", false, "Show version")
	flag.BoolVar(version, "v", false, "Show version")
	configFlag := flag.Bool("config", false, "Show configuration of hotkeys and settings (check values)")
	flag.BoolVar(configFlag, "g", false, "Show configuration of hotkeys and settings (check values)")
	audit := flag.Bool("audit", false, "Show audit information")
	flag.BoolVar(audit, "a", false, "Show audit information")
	tailFlag := flag.String("tail", "50000", "Change the number of log lines to output (range: 200-200000, default: 50000)")
	flag.StringVar(tailFlag, "t", "50000", "Change the number of log lines to output (range: 200-200000, default: 50000)")
	updateFlag := flag.Int("update", 5, "Change the auto refresh interval of the log output (range: 2-10, default: 5)")
	flag.IntVar(updateFlag, "u", 5, "Change the auto refresh interval of the log output (range: 2-10, default: 5)")
	disableScroll := flag.Bool("disable-autoupdate", false, "Disable streaming of new events (log is loaded once without automatic update)")
	flag.BoolVar(disableScroll, "e", false, "Disable streaming of new events (log is loaded once without automatic update)")
	disableColor := flag.Bool("disable-color", false, "Disable output coloring")
	flag.BoolVar(disableColor, "d", false, "Disable output coloring")
	disableMouse := flag.Bool("disable-mouse", false, "Disable mouse control support")
	flag.BoolVar(disableMouse, "m", false, "Disable mouse control support")
	disableTimeStamp := flag.Bool("disable-timestamp", false, "Disable timestamp for docker logs")
	flag.BoolVar(disableTimeStamp, "p", false, "Disable timestamp for docker logs")
	dockerStreamFlag := flag.Bool("only-stream", false, "Force reading of docker container logs in stream mode (by default from the file system)")
	flag.BoolVar(dockerStreamFlag, "o", false, "Force reading of docker container logs in stream mode (by default from the file system)")
	commandColor := flag.Bool("command-color", false, "ANSI coloring in command line mode")
	flag.BoolVar(commandColor, "c", false, "ANSI coloring in command line mode")
	commandFuzzy := flag.String("command-fuzzy", "", "Filtering using fuzzy search in command line mode")
	flag.StringVar(commandFuzzy, "f", "", "Filtering using fuzzy search in command line mode")
	commandRegex := flag.String("command-regex", "", "Filtering using regular expression (regexp) in command line mode")
	flag.StringVar(commandRegex, "r", "", "Filtering using regular expression (regexp) in command line mode")
	sshModeFlag := flag.String("ssh", "", "Connect to remote host (use standard SSH options, separated by spaces in quotes)")
	flag.StringVar(sshModeFlag, "s", "", "Connect to remote host (use standard SSH options, separated by spaces in quotes)")

	// Обработка аргументов
	flag.Parse()

	if *help {
		showHelp()
		os.Exit(0)
	}

	if *version {
		fmt.Println(programVersion)
		os.Exit(0)
	}

	if *configFlag {
		showConfig()
		os.Exit(0)
	}

	if *audit {
		app.showAudit()
		os.Exit(0)
	}

	// Проверяем и извлекаем значения настроек для флагов из конфигурации

	_, errConfig := config.getConfig()
	if errConfig != nil {
		fmt.Println(errConfig)
	}

	if config.Settings.TailMode != "" && *tailFlag == "50000" {
		tailFlag = &config.Settings.TailMode
	}

	if config.Settings.UpdateInterval != "" && *updateFlag == 5 {
		updateIntervalInt, err := strconv.Atoi(config.Settings.UpdateInterval)
		if err == nil {
			updateFlag = &updateIntervalInt
		}
	}

	if config.Settings.DisableAutoUpdate != "" && !*disableScroll {
		if strings.EqualFold(config.Settings.DisableAutoUpdate, "true") {
			trueFlag := true
			disableScroll = &trueFlag
		}
	}

	if config.Settings.DisableColor != "" && !*disableColor {
		if strings.EqualFold(config.Settings.DisableColor, "true") {
			trueFlag := true
			disableColor = &trueFlag
		}
	}

	if config.Settings.DisableMouse != "" && !*disableMouse {
		if strings.EqualFold(config.Settings.DisableMouse, "true") {
			trueFlag := true
			disableMouse = &trueFlag
		}
	}

	if config.Settings.DisableTimestamp != "" && !*disableTimeStamp {
		if strings.EqualFold(config.Settings.DisableTimestamp, "true") {
			trueFlag := true
			disableTimeStamp = &trueFlag
		}
	}

	if config.Settings.OnlyStream != "" && !*dockerStreamFlag {
		if strings.EqualFold(config.Settings.OnlyStream, "true") {
			trueFlag := true
			dockerStreamFlag = &trueFlag
		}
	}

	if config.Settings.DisableFastMode != "" {
		if strings.EqualFold(config.Settings.DisableFastMode, "true") {
			app.fastMode = false
		}
	}

	// Обработка остальных флагов с учетом полученных данных из конфигурации

	if *tailFlag == "200" || *tailFlag == "500" || *tailFlag == "1000" ||
		*tailFlag == "5000" || *tailFlag == "10000" || *tailFlag == "20000" ||
		*tailFlag == "30000" || *tailFlag == "50000" || *tailFlag == "100000" ||
		*tailFlag == "150000" || *tailFlag == "200000" {
		app.logViewCount = *tailFlag
	} else {
		// Если ошибка в конфигурации, задаем значение по умолчанию
		if config.Settings.TailMode != "" && *tailFlag == "" {
			app.logViewCount = "50000"
		} else {
			// Если ошибка в флаге, возвращяем ошибку
			fmt.Println("Available values: 200, 500, 1000, 5000, 10000, 20000, 30000 50000, 100000, 150000, 200000 (default: 50000 lines)")
			os.Exit(1)
		}
	}

	if *updateFlag >= 2 && *updateFlag <= 10 {
		app.logUpdateSeconds = *updateFlag
	} else {
		if config.Settings.UpdateInterval != "" && *updateFlag == 0 {
			app.logUpdateSeconds = 5
		} else {
			fmt.Println("Valid range: 2-10 (default: 5 seconds)")
			os.Exit(1)
		}
	}

	if *disableScroll {
		app.disableAutoScroll = true
		app.autoScroll = false
	}

	if *disableColor {
		app.colorMode = false
	}

	if *disableMouse {
		app.mouseSupport = false
	}

	if *disableTimeStamp {
		app.timestampDocker = false
	}

	if *dockerStreamFlag {
		app.dockerStreamLogs = true
		app.dockerStreamLogsStr = "stream"
	} else {
		// Проверяем доступность директории на чтение
		dir := "/var/lib/docker/containers"
		f, err := os.Open(dir)
		if err != nil {
			app.dockerStreamLogsStr = "stream"
		} else {
			// Пробуем прочитать имя первого элемента (проверить список файлов/директорий)
			_, err = f.Readdirnames(1)
			f.Close()
			if err != nil {
				app.dockerStreamLogsStr = "stream"
			} else {
				app.dockerStreamLogsStr = "json"
			}
		}
	}

	// Определяем переменные и массивы для покраски вывода

	// Текущее имя хоста
	app.hostName, _ = os.Hostname()
	// Удаляем доменную часть, если она есть
	if strings.Contains(app.hostName, ".") {
		app.hostName = strings.Split(app.hostName, ".")[0]
	}
	// Текущее имя пользователя
	currentUser, _ := user.Current()
	app.userName = currentUser.Username
	// Удаляем доменную часть, если она есть
	if strings.Contains(app.userName, "\\") {
		app.userName = strings.Split(app.userName, "\\")[1]
	}
	// Определяем букву системного диска с установленной ОС Windows
	app.systemDisk = os.Getenv("SystemDrive")
	if len(app.systemDisk) >= 1 {
		app.systemDisk = string(app.systemDisk[0])
	} else {
		app.systemDisk = "C"
	}
	// Имена пользователей
	passwd, _ := os.Open("/etc/passwd")
	scanner := bufio.NewScanner(passwd)
	for scanner.Scan() {
		line := scanner.Text()
		userName := strings.Split(line, ":")
		if len(userName) > 0 {
			app.userNameArray = append(app.userNameArray, userName[0])
		}
	}
	// Список корневых каталогов (ls -d /*/) с приставкой "/"
	files, _ := os.ReadDir("/")
	for _, file := range files {
		if file.IsDir() {
			app.rootDirArray = append(app.rootDirArray, "/"+file.Name())
		}
	}

	// Обработка покраски вывода в режиме командной строки
	if *commandColor {
		app.commandLineColor()
		os.Exit(0)
	}

	// Обработка фильтрации с неточным поиском в режиме командной строки
	if *commandFuzzy != "" {
		app.commandLineFuzzy(*commandFuzzy)
		os.Exit(0)
	}

	// Обработка фильтрации с поддержкой регулярных выражений в режиме командной строки
	if *commandRegex != "" {
		filter := strings.ToLower(*commandRegex)
		// Добавляем флаг для нечувствительности к регистру по умолчанию
		filter = "(?i)" + filter
		// Компилируем и проверяем регулярное выражение
		regex, err := regexp.Compile(filter)
		if err != nil {
			fmt.Println("Regular expression syntax error")
			os.Exit(1)
		}
		app.commandLineRegex(regex)
		os.Exit(0)
	}

	// Включаем режим ssh и заполняем параметры (включая sudo и другие стандартные опции ssh подключения, например, порт)
	if *sshModeFlag != "" {
		app.sshMode = true
		options := strings.Split(*sshModeFlag, " ")
		app.sshOptions = append(app.sshOptions, options...)
		getOS, err := remoteGetOS(app.sshOptions)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		} else {
			app.getOS = getOS
		}
	}

	// Создаем GUI
	var err error
	if mock {
		g, err = gocui.NewGui(gocui.OutputSimulator, true) // 1-й параметр для режима работы терминала (tcell) и 2-й параметр для форка
	} else {
		g, err = gocui.NewGui(gocui.OutputNormal, true)
	}
	if err != nil {
		log.Panicln(err)
	}
	// Закрываем GUI после завершения
	defer g.Close()

	app.gui = g
	// Функция, которая будет вызываться при обновлении интерфейса
	g.SetManagerFunc(app.layout)

	// Включить поддержку мыши
	if app.mouseSupport {
		g.Mouse = true
	}

	// Цветовая схема GUI
	g.FgColor = gocui.ColorDefault // поля всех окон и цвет текста
	g.BgColor = gocui.ColorDefault // фон

	// Привязка клавиш для работы с интерфейсом из функции setupKeybindings()
	if err := app.setupKeybindings(); err != nil {
		log.Panicln("Error key bindings", err)
	}

	// Выполняем layout для инициализации интерфейса
	if err := app.layout(g); err != nil {
		log.Panicln(err)
	}

	// Фиксируем текущее количество видимых строк в терминале (-1 заголовок)
	if v, err := g.View("services"); err == nil {
		_, viewHeight := v.Size()
		app.maxVisibleServices = viewHeight
	}
	// Загрузка списка служб или событий Windows
	if app.getOS == "windows" {
		v, err := g.View("services")
		if err != nil {
			log.Panicln(err)
		}
		v.Title = " < Windows Event Logs (0) > "
		// Загружаем список событий Windows в горутине
		go func() {
			app.loadWinEvents()
		}()
	} else {
		app.loadServices(app.selectUnits)
	}

	// Filesystem
	if v, err := g.View("varLogs"); err == nil {
		_, viewHeight := v.Size()
		app.maxVisibleFiles = viewHeight
	}

	// Определяем ОС и загружаем файловые журналы
	if app.getOS == "windows" {
		selectedVarLog, err := g.View("varLogs")
		if err != nil {
			log.Panicln(err)
		}
		g.Update(func(g *gocui.Gui) error {
			selectedVarLog.Clear()
			fmt.Fprintln(selectedVarLog, "Searching log files...")
			selectedVarLog.Highlight = false
			return nil
		})
		selectedVarLog.Title = " < Program Files (0) > "
		app.selectPath = "ProgramFiles"
		// Загружаем список файлов Windows в горутине
		go func() {
			app.loadWinFiles(app.selectPath)
		}()
	} else {
		app.loadFiles(app.selectPath)
	}

	// Docker
	if v, err := g.View("docker"); err == nil {
		_, viewHeight := v.Size()
		app.maxVisibleDockerContainers = viewHeight
	}
	app.loadDockerContainer(app.selectContainerizationSystem)

	// Устанавливаем фокус на окно с журналами по умолчанию
	if _, err := g.SetCurrentView("filterList"); err != nil {
		return
	}

	// Горутина для автоматического обновления вывода журнала каждые n (logUpdateSeconds) секунд
	app.secondsChan = make(chan int, app.logUpdateSeconds)
	go func() {
		app.updateLogBackground(app.secondsChan, false)
	}()

	// Горутина для отслеживания изменений размера окна и его перерисовки
	go func() {
		app.updateWindowSize(1)
	}()

	// Запус GUI
	if err := g.MainLoop(); err != nil && !errors.Is(err, gocui.ErrQuit) {
		log.Panicln(err)
	}
}

func main() {
	runGoCui(false)
}

// Структура интерфейса окон GUI
func (app *App) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()                // получаем текущий размер интерфейса терминала (ширина, высота)
	leftPanelWidth := maxX / 4            // ширина левой колонки
	inputHeight := 3                      // высота поля ввода для фильтрации список
	availableHeight := maxY - inputHeight // общая высота всех трех окон слева
	panelHeight := availableHeight / 3    // высота каждого окна

	// Поле ввода для фильтрации списков
	if v, err := g.SetView("filterList", 0, 0, leftPanelWidth-1, inputHeight-1, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Title = "Filtering lists"
		v.Editable = true
		v.Wrap = true
		v.FrameColor = gocui.ColorGreen // Цвет границ окна
		v.TitleColor = gocui.ColorGreen // Цвет заголовка
		v.Editor = app.createFilterEditor("lists")
	}

	// Окно для отображения списка доступных журналов (UNIT)
	// Размеры окна: заголовок, отступ слева, отступ сверху, ширина, высота, 5-й параметр из форка для продолжение окна (2)
	if v, err := g.SetView("services", 0, inputHeight, leftPanelWidth-1, inputHeight+panelHeight-1, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Title = " < Unit list (0) > " // заголовок окна
		v.Highlight = true              // выделение активного элемента в списке
		v.Wrap = false                  // отключаем перенос строк
		v.Autoscroll = true             // включаем автопрокрутку
		// Цветовая схема из форка awesome-gocui/gocui
		v.SelBgColor = gocui.ColorGreen // Цвет фона при выборе в списке
		v.SelFgColor = gocui.ColorBlack // Цвет текста
		app.updateServicesList()        // выводим список журналов в это окно
	}

	// Окно для списка логов из файловой системы
	if v, err := g.SetView("varLogs", 0, inputHeight+panelHeight, leftPanelWidth-1, inputHeight+2*panelHeight-1, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Title = " < System var logs (0) > "
		v.Highlight = true
		v.Wrap = false
		v.Autoscroll = true
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
		app.updateLogsList()
	}

	// Окно для списка контейнеров Docker и Podman
	if v, err := g.SetView("docker", 0, inputHeight+2*panelHeight, leftPanelWidth-1, maxY-1, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Title = " < Docker containers (0) > "
		v.Highlight = true
		v.Wrap = false
		v.Autoscroll = true
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
	}

	// Окно ввода текста для фильтрации
	if v, err := g.SetView("filter", leftPanelWidth+1, 0, maxX-1, 2, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Title = "Filter (Default)"
		v.Editable = true                         // включить окно редактируемым для ввода текста
		v.Editor = app.createFilterEditor("logs") // редактор для обработки ввода
		v.Wrap = true
	}

	// Интерфейс скролла в окне вывода лога (maxX-3 ширина окна - отступ слева)
	if v, err := g.SetView("scrollLogs", maxX-3, 3, maxX-1, maxY-1, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Wrap = true
		v.Autoscroll = false
		// Цвет текста (зеленый)
		v.FgColor = gocui.ColorGreen
		// Заполняем окно стрелками
		_, viewHeight := v.Size()
		fmt.Fprintln(v, "▲")
		for i := 1; i < viewHeight-1; i++ {
			fmt.Fprintln(v, " ")
		}
		fmt.Fprintln(v, "▼")
	}

	// Окно для вывода записей выбранного журнала (maxX-2 для отступа скролла и 8 для продолжения углов)
	if v, err := g.SetView("logs", leftPanelWidth+1, 3, maxX-1-2, maxY-1, 8); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Title = "Logs"
		v.Wrap = true
		v.Autoscroll = false
		v.Subtitle = fmt.Sprintf("[tail: %s lines | auto-update: %t (%d sec) | docker: %s (%s) | color: %t]", app.logViewCount, app.autoScroll, app.logUpdateSeconds, app.dockerStreamLogsStr, app.dockerStreamMode, app.colorMode)
	}

	// Включение курсора в режиме фильтра и отключение в остальных окнах
	currentView := g.CurrentView()
	if currentView != nil && (currentView.Name() == "filter" || currentView.Name() == "filterList" || currentView.Name() == "sinceFilter" || currentView.Name() == "untilFilter") {
		g.Cursor = true
	} else {
		g.Cursor = false
	}

	return nil
}

// ---------------------------------------- journalctl/Windows Event Logs ----------------------------------------

// Функция для удаления ANSI-символов покраски
func removeANSI(input string) string {
	ansiEscapeRegex := regexp.MustCompile(`\033\[[0-9;]*m`)
	return ansiEscapeRegex.ReplaceAllString(input, "")
}

// Функция для извлечения даты из строки для списка загрузок ядра
func parseDateFromName(name string) time.Time {
	cleanName := removeANSI(name)
	dateFormat := "02.01.2006 15:04:05"
	// Извлекаем дату, начиная с 22-го символа (после дефиса)
	parsedDate, _ := time.Parse(dateFormat, cleanName[22:])
	return parsedDate
}

// Функция для загрузки списка журналов служб или загрузок системы из journalctl
func (app *App) loadServices(journalName string) {
	app.journals = nil
	// Проверка, что в системе установлен/поддерживается утилита journalctl
	var checkJournald *exec.Cmd
	if app.sshMode {
		checkJournald = exec.Command("ssh", append(app.sshOptions, "journalctl", "--version")...)
	} else {
		checkJournald = exec.Command("journalctl", "--version")
	}
	// Проверяем на ошибки (очищаем список служб, отключаем курсор и выводим ошибку)
	_, err := checkJournald.Output()
	if err != nil && !app.testMode {
		vError, _ := app.gui.View("services")
		vError.Clear()
		app.journalListFrameColor = gocui.ColorRed
		vError.FrameColor = app.journalListFrameColor
		vError.Highlight = false
		fmt.Fprintln(vError, "\033[31msystemd-journald not supported\033[0m")
		return
	}
	if err != nil && app.testMode {
		log.Print("Error: systemd-journald not supported")
	}
	switch {
	// Services list from systemd
	case journalName == "services":
		// Получаем список всех юнитов в системе через systemctl в формате JSON
		var unitsList *exec.Cmd
		if app.sshMode {
			unitsList = exec.Command("ssh", append(app.sshOptions, "systemctl", "list-units", "--all", "--plain", "--no-legend", "--no-pager", "--output=json")...) // "--type=service"
		} else {
			unitsList = exec.Command("systemctl", "list-units", "--all", "--plain", "--no-legend", "--no-pager", "--output=json") // "--type=service"
		}
		output, err := unitsList.Output()
		if !app.testMode {
			if err != nil {
				vError, _ := app.gui.View("services")
				vError.Clear()
				app.journalListFrameColor = gocui.ColorRed
				vError.FrameColor = app.journalListFrameColor
				vError.Highlight = false
				fmt.Fprintln(vError, "\033[31mAccess denied in systemd via systemctl\033[0m")
				return
			}
			v, _ := app.gui.View("services")
			app.journalListFrameColor = gocui.ColorDefault
			if v.FrameColor != gocui.ColorDefault {
				v.FrameColor = gocui.ColorGreen
			}
			v.Highlight = true
		}
		if err != nil && app.testMode {
			log.Print("Error: access denied in systemd via systemctl")
		}
		// Чтение данных в формате JSON
		var units []map[string]interface{}
		err = json.Unmarshal(output, &units)
		// Если ошибка JSON, создаем массив вручную
		if err != nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				// Разбиваем строку на поля (эквивалентно: awk '{print $1,$3,$4}')
				fields := strings.Fields(line)
				// Пропускаем строки с недостаточным количеством полей
				if len(fields) < 3 {
					continue
				}
				// Заполняем временный массив из строки
				unit := map[string]interface{}{
					"unit":   fields[0],
					"active": fields[2],
					"sub":    fields[3],
				}
				// Добавляем временный массив строки в основной массив
				units = append(units, unit)
			}
		}
		serviceMap := make(map[string]bool)
		// Обработка записей
		for _, unit := range units {
			// Извлечение данных в формате JSON и проверка статуса для покраски
			unitName, _ := unit["unit"].(string)
			active, _ := unit["active"].(string)
			if active == "active" {
				active = "\033[32m" + active + "\033[0m"
			} else {
				active = "\033[31m" + active + "\033[0m"
			}
			sub, _ := unit["sub"].(string)
			if sub == "exited" || sub == "dead" {
				sub = "\033[31m" + sub + "\033[0m"
			} else {
				sub = "\033[32m" + sub + "\033[0m"
			}
			name := unitName + " (" + active + "/" + sub + ")"
			bootID := unitName
			// Уникальный ключ для проверки
			uniqueKey := name + ":" + bootID
			if !serviceMap[uniqueKey] {
				serviceMap[uniqueKey] = true
				// Добавление записи в массив
				app.journals = append(app.journals, Journal{
					name:    name,
					boot_id: bootID,
				})
			}
		}
	// Audit rules keys from auditd
	case journalName == "auditd":
		// Получаем список правил
		var auditRulesList *exec.Cmd
		if app.sshMode {
			auditRulesList = exec.Command("ssh", append(app.sshOptions, "auditctl", "-l")...)
		} else {
			auditRulesList = exec.Command("auditctl", "-l")
		}
		output, err := auditRulesList.Output()
		// Проверяем, что auditd установлен и на ошибку доступа
		if !app.testMode {
			if err != nil {
				var errorText string
				if err.Error() == "exit status 4" {
					errorText = "Access denied in auditd via auditctl (root only)"
				} else {
					errorText = "Auditd not installed"
				}
				vError, _ := app.gui.View("services")
				vError.Clear()
				app.journalListFrameColor = gocui.ColorRed
				vError.FrameColor = app.journalListFrameColor
				vError.Highlight = false
				fmt.Fprintln(vError, "\033[31m"+errorText+"\033[0m")
				return
			}
			v, _ := app.gui.View("services")
			app.journalListFrameColor = gocui.ColorDefault
			if v.FrameColor != gocui.ColorDefault {
				v.FrameColor = gocui.ColorGreen
			}
			v.Highlight = true
		}
		if err != nil && app.testMode {
			if strings.Contains(err.Error(), "root to run") {
				log.Print("Access denied in auditd via auditctl (root only)")
			} else {
				log.Print("Auditd not installed")
			}
		}
		// Заполняем список всех уникальный ключей
		keysMap := make(map[string]bool)
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			rule := scanner.Text()
			if strings.Contains(rule, "-k ") {
				// Разбиваем строку правила на 2 части (split) до ключа
				rulePart := strings.Split(rule, "-k ")
				if len(rulePart) > 1 {
					// Разбиваем на слова (fields) из второй части правила после ключа и извлекаем первое слово
					keyPart := strings.Fields(rulePart[1])[0]
					if !keysMap[keyPart] {
						keysMap[keyPart] = true
						app.journals = append(app.journals, Journal{
							name:    keyPart,
							boot_id: keyPart,
						})
					}
				}
			}
		}
	// Boots list from journald
	case journalName == "kernel":
		// Получаем список загрузок системы
		var bootCmd *exec.Cmd
		if app.sshMode {
			bootCmd = exec.Command("ssh", append(app.sshOptions, "journalctl", "--list-boots", "-o", "json")...)
		} else {
			bootCmd = exec.Command("journalctl", "--list-boots", "-o", "json")
		}
		bootOutput, err := bootCmd.Output()
		if !app.testMode {
			if err != nil {
				vError, _ := app.gui.View("services")
				vError.Clear()
				app.journalListFrameColor = gocui.ColorRed
				vError.FrameColor = app.journalListFrameColor
				vError.Highlight = false
				fmt.Fprintln(vError, "\033[31mError getting boot information from journald\033[0m")
				return
			} else {
				vError, _ := app.gui.View("services")
				app.journalListFrameColor = gocui.ColorDefault
				if vError.FrameColor != gocui.ColorDefault {
					vError.FrameColor = gocui.ColorGreen
				}
				vError.Highlight = true
			}
		}
		if err != nil && app.testMode {
			log.Print("Error: getting boot information from journald")
		}
		// Структура для парсинга JSON
		type BootInfo struct {
			BootID     string `json:"boot_id"`
			FirstEntry int64  `json:"first_entry"`
			LastEntry  int64  `json:"last_entry"`
		}
		var bootRecords []BootInfo
		err = json.Unmarshal(bootOutput, &bootRecords)
		// Если JSON невалидный или режим тестирования (Ubuntu 20.04 не поддерживает вывод в формате json)
		if err != nil || app.testMode {
			// Парсим вывод построчно
			lines := strings.Split(string(bootOutput), "\n")
			for _, line := range lines {
				// Разбиваем строку на массив
				wordsArray := strings.Fields(line)
				// 0 d914ebeb67c6428a87f9cfe3861c295d Mon 2024-11-25 12:15:07 MSK—Mon 2024-11-25 18:34:53 MSK
				if len(wordsArray) >= 8 {
					bootId := wordsArray[1]
					// Забираем дату, проверяем и изменяем формат
					var parseDate []string
					var bootDate string
					parseDate = strings.Split(wordsArray[3], "-")
					if len(parseDate) == 3 {
						bootDate = fmt.Sprintf("%s.%s.%s", parseDate[2], parseDate[1], parseDate[0])
					} else {
						continue
					}
					var stopDate string
					parseDate = strings.Split(wordsArray[6], "-")
					if len(parseDate) == 3 {
						stopDate = fmt.Sprintf("%s.%s.%s", parseDate[2], parseDate[1], parseDate[0])
					} else {
						continue
					}
					// Заполняем массив
					bootDateTime := bootDate + " " + wordsArray[4]
					stopDateTime := stopDate + " " + wordsArray[7]
					app.journals = append(app.journals, Journal{
						name:    fmt.Sprintf("\033[34m%s\033[0m - \033[34m%s\033[0m", bootDateTime, stopDateTime),
						boot_id: bootId,
					})
				}
			}
		}
		if err == nil {
			// Очищаем массив, если он был заполнен в режиме тестирования
			app.journals = []Journal{}
			// Добавляем информацию о загрузках в app.journals
			for _, bootRecord := range bootRecords {
				// Преобразуем наносекунды в секунды
				firstEntryTime := time.Unix(bootRecord.FirstEntry/1000000, bootRecord.FirstEntry%1000000)
				lastEntryTime := time.Unix(bootRecord.LastEntry/1000000, bootRecord.LastEntry%1000000)
				// Форматируем строку в формате "DD.MM.YYYY HH:MM:SS"
				const dateFormat = "02.01.2006 15:04:05"
				name := fmt.Sprintf("\033[34m%s\033[0m - \033[34m%s\033[0m", firstEntryTime.Format(dateFormat), lastEntryTime.Format(dateFormat))
				// Добавляем в массив
				app.journals = append(app.journals, Journal{
					name:    name,
					boot_id: bootRecord.BootID,
				})
			}
		}
		// Сортируем по второй дате
		sort.Slice(app.journals, func(i, j int) bool {
			date1 := parseDateFromName(app.journals[i].name)
			date2 := parseDateFromName(app.journals[j].name)
			// Сравниваем по второй дате в обратном порядке (After для сортировки по убыванию)
			return date1.After(date2)
		})
	// Journals list from journald
	default:
		var cmd *exec.Cmd
		if app.sshMode {
			cmd = exec.Command("ssh", append(app.sshOptions, "journalctl", "--no-pager", "-F", journalName)...)
		} else {
			cmd = exec.Command("journalctl", "--no-pager", "-F", journalName)
		}
		output, err := cmd.Output()
		if !app.testMode {
			if err != nil {
				vError, _ := app.gui.View("services")
				vError.Clear()
				app.journalListFrameColor = gocui.ColorRed
				vError.FrameColor = app.journalListFrameColor
				vError.Highlight = false
				fmt.Fprintln(vError, "\033[31mError getting services from journald via journalctl\033[0m")
				return
			} else {
				vError, _ := app.gui.View("services")
				app.journalListFrameColor = gocui.ColorDefault
				if vError.FrameColor != gocui.ColorDefault {
					vError.FrameColor = gocui.ColorGreen
				}
				vError.Highlight = true
			}
		}
		if err != nil && app.testMode {
			log.Print("Error: getting services from journald via journalctl")
		}
		// Создаем массив (хеш-таблица с доступом по ключу) для уникальных имен служб
		serviceMap := make(map[string]bool)
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			serviceName := strings.TrimSpace(scanner.Text())
			if serviceName != "" && !serviceMap[serviceName] {
				serviceMap[serviceName] = true
				app.journals = append(app.journals, Journal{
					name:    serviceName,
					boot_id: "",
				})
			}
		}
		// Сортируем список служб по алфавиту
		sort.Slice(app.journals, func(i, j int) bool {
			return app.journals[i].name < app.journals[j].name
		})
	}
	if !app.testMode {
		// Сохраняем неотфильтрованный список
		app.journalsNotFilter = app.journals
		// Применяем фильтр при загрузки и обновляем список служб в интерфейсе через updateServicesList() внутри функции
		app.applyFilterList()
	}
}

// Функция для загрузки списка всех журналов событий Windows через PowerShell
func (app *App) loadWinEvents() {
	app.debugStartTime = time.Now()
	app.journals = nil
	// Получаем список, игнорируем ошибки, фильтруем пустые журналы, забираем нужные параметры, сортируем и выводим в формате JSON
	cmd := exec.Command("powershell", "-Command",
		"Get-WinEvent -ListLog * -ErrorAction Ignore | "+
			"Where-Object RecordCount -ne 0 | "+
			"Where-Object RecordCount -ne $null | "+
			"Select-Object LogName,RecordCount | "+
			"Sort-Object -Descending RecordCount | "+
			"ConvertTo-Json")
	eventsJson, _ := cmd.Output()
	var events []map[string]interface{}
	_ = json.Unmarshal(eventsJson, &events)
	for _, event := range events {
		// Извлечение названия журнала и количество записей
		LogName, _ := event["LogName"].(string)
		RecordCount, _ := event["RecordCount"].(float64)
		RecordCountInt := int(RecordCount)
		RecordCountString := strconv.Itoa(RecordCountInt)
		// Удаляем приставку
		LogView := strings.ReplaceAll(LogName, "Microsoft-Windows-", "")
		// Разбивает строку на 2 части для покраски
		LogViewSplit := strings.SplitN(LogView, "/", 2)
		if len(LogViewSplit) == 2 {
			LogView = "\033[33m" + LogViewSplit[0] + "\033[0m" + ": " + "\033[36m" + LogViewSplit[1] + "\033[0m"
		} else {
			LogView = "\033[36m" + LogView + "\033[0m"
		}
		LogView = LogView + " (" + RecordCountString + ")"
		app.journals = append(app.journals, Journal{
			name:    LogView,
			boot_id: LogName,
		})
	}
	if !app.testMode {
		app.journalsNotFilter = app.journals
		app.applyFilterList()
	}
}

// Функция для обновления окна со списком служб
func (app *App) updateServicesList() {
	// Выбираем окно для заполнения в зависимости от используемого журнала
	v, err := app.gui.View("services")
	if err != nil {
		return
	}
	// Очищаем окно
	v.Clear()
	// Вычисляем конечную позицию видимой области (стартовая позиция + максимальное количество видимых строк)
	visibleEnd := app.startServices + app.maxVisibleServices
	if visibleEnd > len(app.journals) {
		visibleEnd = len(app.journals)
	}
	// Отображаем только элементы в пределах видимой области
	for i := app.startServices; i < visibleEnd; i++ {
		fmt.Fprintln(v, app.journals[i].name)
	}
}

// Функция для перемещения по списку журналов вниз
func (app *App) nextService(v *gocui.View, step int) error {
	// Обновляем текущее количество видимых строк в терминале (-1 заголовок)
	_, viewHeight := v.Size()
	app.maxVisibleServices = viewHeight
	// Если список журналов пустой, ничего не делаем
	if len(app.journals) == 0 {
		return nil
	}
	// Переходим к следующему, если текущий выбранный журнал не последний
	if app.selectedJournal < len(app.journals)-1 {
		// Увеличиваем индекс выбранного журнала
		app.selectedJournal += step
		// Проверяем, чтобы не выйти за пределы списка
		if app.selectedJournal >= len(app.journals) {
			app.selectedJournal = len(app.journals) - 1
		}
		// Проверяем, вышли ли за пределы видимой области (увеличиваем стартовую позицию видимости, только если дошли до 0 + maxVisibleServices)
		if app.selectedJournal >= app.startServices+app.maxVisibleServices {
			// Сдвигаем видимую область вниз
			app.startServices += step
			// Проверяем, чтобы не выйти за пределы списка
			if app.startServices > len(app.journals)-app.maxVisibleServices {
				app.startServices = len(app.journals) - app.maxVisibleServices
			}
			// Обновляем отображение списка служб
			app.updateServicesList()
		}
		// Если сдвинули видимую область, корректируем индекс для смещения курсора в интерфейсе
		if app.selectedJournal < app.startServices+app.maxVisibleServices {
			// Выбираем журнал по скорректированному индексу
			return app.selectServiceByIndex(app.selectedJournal - app.startServices)
		}
	}
	return nil
}

// Функция для перемещения по списку журналов вверх
func (app *App) prevService(v *gocui.View, step int) error {
	_, viewHeight := v.Size()
	app.maxVisibleServices = viewHeight
	if len(app.journals) == 0 {
		return nil
	}
	// Переходим к предыдущему, если текущий выбранный журнал не первый
	if app.selectedJournal > 0 {
		app.selectedJournal -= step
		// Если ушли в минус (за начало журнала), приводим к нулю
		if app.selectedJournal < 0 {
			app.selectedJournal = 0
		}
		// Проверяем, вышли ли за пределы видимой области
		if app.selectedJournal < app.startServices {
			app.startServices -= step
			if app.startServices < 0 {
				app.startServices = 0
			}
			app.updateServicesList()
		}
		if app.selectedJournal >= app.startServices {
			return app.selectServiceByIndex(app.selectedJournal - app.startServices)
		}
	}
	return nil
}

// Функция для визуального выбора журнала по индексу (смещение курсора выделения)
func (app *App) selectServiceByIndex(index int) error {
	// Получаем доступ к представлению списка служб
	v, err := app.gui.View("services")
	if err != nil {
		return err
	}
	// Обновляем счетчик в заголовке
	re := regexp.MustCompile(`\s\(.+\) >`)
	updateTitle := " (0) >"
	if len(app.journals) != 0 {
		updateTitle = " (" + strconv.Itoa(app.selectedJournal+1) + "/" + strconv.Itoa(len(app.journals)) + ") >"
	}
	v.Title = re.ReplaceAllString(v.Title, updateTitle)
	// Устанавливаем курсор на нужный индекс (строку)
	// Первый столбец (0), индекс строки
	if err := v.SetCursor(0, index); err != nil {
		return nil
	}
	return nil
}

// Функция для выбора журнала в списке сервисов по нажатию Enter
func (app *App) selectService(g *gocui.Gui, v *gocui.View) error {
	// Проверка, что есть доступ к представлению и список журналов не пустой
	if v == nil || len(app.journals) == 0 {
		return nil
	}
	// Получаем текущую позицию курсора
	_, cy := v.Cursor()
	// Читаем строку, на которой находится курсор
	line, err := v.Line(cy)
	if err != nil {
		return err
	}
	// Загружаем журналы выбранной службы, обрезая пробелы в названии
	if app.fastMode {
		go func() {
			app.loadJournalLogs(strings.TrimSpace(line), true)
		}()
	} else {
		app.loadJournalLogs(strings.TrimSpace(line), true)
	}
	// Включаем загрузку журнала (только при ручном выборе для Windows)
	app.updateFile = true
	// Фиксируем для ручного или автоматического обновления вывода журнала
	app.lastWindow = "services"
	app.lastSelected = strings.TrimSpace(line)
	return nil
}

// Функция для загрузки записей журнала выбранной службы через journalctl
// Второй параметр для обнолвения позиции делимитра нового вывода лога а также сброса автоскролл
func (app *App) loadJournalLogs(serviceName string, newUpdate bool) {
	app.debugStartTime = time.Now()
	var output []byte
	var err error
	selectUnits := app.selectUnits
	if newUpdate {
		app.lastSelectUnits = app.selectUnits
	} else {
		selectUnits = app.lastSelectUnits
	}
	switch {
	// Читаем журналы Windows
	case app.getOS == "windows":
		if !app.updateFile {
			return
		}
		// Отключаем чтение в горутине
		app.updateFile = false
		// Извлекаем полное имя события
		var eventName string
		for _, journal := range app.journals {
			journalBootName := removeANSI(journal.name)
			if journalBootName == serviceName {
				eventName = journal.boot_id
				break
			}
		}
		output = app.loadWinEventLog(eventName)
		if len(output) == 0 && !app.testMode {
			v, _ := app.gui.View("logs")
			v.Clear()
			return
		}
		if len(output) == 0 && app.testMode {
			app.currentLogLines = []string{}
			return
		}
	// Читаем лог выбранного по ключу журнала аудита
	case selectUnits == "auditd":
		if newUpdate {
			app.lastBootId = serviceName
		} else {
			serviceName = app.lastBootId
		}
		var cmd *exec.Cmd
		if app.sshMode {
			cmd = exec.Command("ssh", append(app.sshOptions, "ausearch", "-k", serviceName, "--format", "interpret")...)
		} else {
			cmd = exec.Command("ausearch", "-k", serviceName, "--format", "interpret")
		}
		output, err = cmd.Output()
		if err != nil && !app.testMode {
			v, _ := app.gui.View("logs")
			v.Clear()
			fmt.Fprintln(v, "\033[31mError getting auditd logs:", err, "\033[0m")
			return
		}
		if err != nil && app.testMode {
			log.Print("Error: getting auditd logs. ", err)
		}
	// Читаем лог ядра загрузки системы
	case selectUnits == "kernel":
		// Извлекаем id журнала из названия
		var boot_id string
		for _, journal := range app.journals {
			journalBootName := removeANSI(journal.name)
			if journalBootName == serviceName {
				boot_id = journal.boot_id
				break
			}
		}
		// Сохраняем название для обновления вывода журнала при фильтрации списков
		if newUpdate {
			app.lastBootId = boot_id
		} else {
			boot_id = app.lastBootId
		}
		var cmd *exec.Cmd
		if app.sshMode {
			cmd = exec.Command("ssh", append(app.sshOptions, "journalctl", "-k", "-b", boot_id, "--no-pager", "-n", app.logViewCount)...)
		} else {
			cmd = exec.Command("journalctl", "-k", "-b", boot_id, "--no-pager", "-n", app.logViewCount)
		}
		output, err = cmd.Output()
		if err != nil && !app.testMode {
			v, _ := app.gui.View("logs")
			v.Clear()
			fmt.Fprintln(v, "\033[31mError getting kernal logs:", err, "\033[0m")
			return
		}
		if err != nil && app.testMode {
			log.Print("Error: getting kernal logs. ", err)
		}
	// Для юнитов systemd и других журналов по названию (--unit=UNIT)
	default:
		if selectUnits == "services" {
			// Удаляем статусы с покраской из навзания
			var ansiEscape = regexp.MustCompile(`\s\(.+\)`)
			serviceName = ansiEscape.ReplaceAllString(serviceName, "")
		}
		var cmd *exec.Cmd
		if app.sshMode {
			switch {
			case app.sinceTimestampFilterMode && app.untilTimestampFilterMode:
				cmd = exec.Command("ssh", append(app.sshOptions, "journalctl", "-u", serviceName, "--no-pager", "--since", app.sinceFilterText, "--until", app.untilFilterText, "-n", app.logViewCount)...)
			case app.sinceTimestampFilterMode && !app.untilTimestampFilterMode:
				cmd = exec.Command("ssh", append(app.sshOptions, "journalctl", "-u", serviceName, "--no-pager", "--since", app.sinceFilterText, "-n", app.logViewCount)...)
			case !app.sinceTimestampFilterMode && app.untilTimestampFilterMode:
				cmd = exec.Command("ssh", append(app.sshOptions, "journalctl", "-u", serviceName, "--no-pager", "--until", app.untilFilterText, "-n", app.logViewCount)...)
			default:
				cmd = exec.Command("ssh", append(app.sshOptions, "journalctl", "-u", serviceName, "--no-pager", "-n", app.logViewCount)...)
			}
		} else {
			switch {
			case app.sinceTimestampFilterMode && app.untilTimestampFilterMode:
				cmd = exec.Command("journalctl", "-u", serviceName, "--no-pager", "--since", app.sinceFilterText, "--until", app.untilFilterText, "-n", app.logViewCount)
			case app.sinceTimestampFilterMode && !app.untilTimestampFilterMode:
				cmd = exec.Command("journalctl", "-u", serviceName, "--no-pager", "--since", app.sinceFilterText, "-n", app.logViewCount)
			case !app.sinceTimestampFilterMode && app.untilTimestampFilterMode:
				cmd = exec.Command("journalctl", "-u", serviceName, "--no-pager", "--until", app.untilFilterText, "-n", app.logViewCount)
			default:
				cmd = exec.Command("journalctl", "-u", serviceName, "--no-pager", "-n", app.logViewCount)
			}
		}
		output, err = cmd.Output()
		if err != nil && !app.testMode {
			v, _ := app.gui.View("logs")
			v.Clear()
			fmt.Fprintln(v, "\033[31mError getting journald logs:", err, "\033[0m")
			return
		}
		if err != nil && app.testMode {
			log.Print("Error: getting journald logs.  ", err)
		}
	}
	// Сохраняем строки журнала в массив
	app.currentLogLines = strings.Split(string(output), "\n")
	if !app.testMode {
		app.updateDelimiter(newUpdate)
		// Очищаем поле ввода для фильтрации, что бы не применять фильтрацию к новому журналу
		// app.filterText = ""
		// Применяем текущий фильтр к записям для обновления вывода
		app.applyFilter(false)
	}
}

// Функция для чтения и парсинга содержимого события Windows через wevtutil
func (app *App) loadWinEventLog(eventName string) (output []byte) {
	cmd := exec.Command("powershell", "-Command",
		"wevtutil qe "+eventName+" /f:text -l:en /c:"+app.logViewCount+
			" /q:'*[System[TimeCreated[timediff(@SystemTime) <= 2592000000]]]'")
	eventData, _ := cmd.Output()
	// Декодирование вывода из Windows-1251 в UTF-8
	decoder := charmap.Windows1251.NewDecoder()
	decodeEventData, decodeErr := decoder.Bytes(eventData)
	if decodeErr == nil {
		eventData = decodeEventData
	}
	// Разбиваем вывод на массив
	eventStrings := strings.Split(string(eventData), "Event[")
	var eventMessage []string
	for _, eventString := range eventStrings {
		var dateTime, eventID, level, description string
		// Разбиваем элемент массива на строки
		lines := strings.Split(eventString, "\n")
		// Флаг для обработки последней строки Description с содержимым Message
		isDescription := false
		for _, line := range lines {
			// Удаляем проблемы во всех строках
			trimmedLine := strings.TrimSpace(line)
			switch {
			// Обновляем формат даты
			case strings.HasPrefix(trimmedLine, "Date:"):
				dateTime = strings.ReplaceAll(trimmedLine, "Date: ", "")
				dateTimeParse := strings.Split(dateTime, "T")
				dateParse := strings.Split(dateTimeParse[0], "-")
				timeParse := strings.Split(dateTimeParse[1], ".")
				dateTime = fmt.Sprintf("%s.%s.%s %s", dateParse[2], dateParse[1], dateParse[0], timeParse[0])
			case strings.HasPrefix(trimmedLine, "Event ID:"):
				eventID = strings.ReplaceAll(trimmedLine, "Event ID: ", "")
			case strings.HasPrefix(trimmedLine, "Level:"):
				level = strings.ReplaceAll(trimmedLine, "Level: ", "")
			case strings.HasPrefix(trimmedLine, "Description:"):
				// Фиксируем и пропускаем Description
				isDescription = true
			case isDescription:
				// Добавляем до конца текущего массива все не пустые строки
				if trimmedLine != "" {
					description += "\n" + trimmedLine
				}
			}
		}
		if dateTime != "" && eventID != "" && level != "" && description != "" {
			eventMessage = append(eventMessage, fmt.Sprintf("%s %s (%s): %s", dateTime, level, eventID, strings.TrimSpace(description)))
		}
	}
	fullMessage := strings.Join(eventMessage, "\n")
	return []byte(fullMessage)
}

// ---------------------------------------- Filesystem ----------------------------------------

// Базовая структура os.Stat
type fileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

// Дочерние методы os.Stat
func (fi *fileInfo) Name() string       { return fi.name }
func (fi *fileInfo) Size() int64        { return fi.size }
func (fi *fileInfo) ModTime() time.Time { return fi.modTime }
func (fi *fileInfo) Mode() os.FileMode  { return 0o644 } // default rights
func (fi *fileInfo) IsDir() bool        { return false } // only file
func (fi *fileInfo) Sys() any           { return nil }

// Имитация метода os.Stat через exec.Command
func (app *App) statFile(path string) (os.FileInfo, error) {
	if app.sshMode {
		// Аргументы для команды stats. Ключи для перехода по символическим ссылкам
		// для получения информации о целевых файлах (для проверки доступа) и форматирования вывода
		statArgs := app.sshOptions
		statArgs = append(statArgs, "stat", "-L", "-c", "'%n|%s|%Y'", path)
		cmd := exec.Command("ssh", statArgs...)
		output, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		// Парсим вывод stat (пример вывода: /var/log/syslog|8744995|1756116219)
		line := strings.TrimSpace(string(output))
		parts := strings.Split(line, "|")
		if len(parts) != 3 {
			return nil, fmt.Errorf("%w: %s", ErrInvalidStat, line)
		}
		// Преобразуем размер и время в int
		size, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, err
		}
		modTimeUnix, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return nil, err
		}
		modTime := time.Unix(modTimeUnix, 0)
		// Создаем кастомный FileInfo
		return &fileInfo{
			name:    parts[0],
			size:    size,
			modTime: modTime,
		}, nil
	} else {
		// В локальном режиме возвращяем стандартный os.Stat
		return os.Stat(path)
	}
}

// Получение массива статистики по всем файлам
func (app *App) statFiles(paths []string) (map[string]os.FileInfo, error) {
	if len(paths) == 0 {
		return make(map[string]os.FileInfo), nil
	}
	args := make([]string, len(app.sshOptions))
	copy(args, app.sshOptions)
	args = append(args, "stat", "-L", "-c", "'%n|%s|%Y'")
	args = append(args, paths...)
	cmd := exec.Command("ssh", args...)
	output, _ := cmd.Output()
	results := make(map[string]os.FileInfo)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 3 {
			return nil, fmt.Errorf("%w: %s", ErrInvalidStat, line)
		}
		size, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, err
		}
		modTimeUnix, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return nil, err
		}
		modTime := time.Unix(modTimeUnix, 0)
		results[parts[0]] = &fileInfo{
			name:    parts[0],
			size:    size,
			modTime: modTime,
		}
	}
	return results, nil
}

func (app *App) loadFiles(logPath string) {
	app.logfiles = nil // сбрасываем (очищаем) массив перед загрузкой новых журналов
	var output []byte
	switch logPath {
	case "descriptor":
		var cmd *exec.Cmd
		if app.sshMode {
			cmd = exec.Command("ssh", append(app.sshOptions, "lsof", "-Fn")...)
		} else {
			cmd = exec.Command("lsof", "-Fn")
		}
		// Подавить вывод ошибок при отсутствиее прав доступа (opendir: Permission denied)
		cmd.Stderr = nil
		output, _ = cmd.Output()
		// Разбиваем вывод на строки
		files := strings.Split(strings.TrimSpace(string(output)), "\n")
		// Если список файлов пустой, возвращаем ошибку Permission denied
		if !app.testMode {
			if len(files) == 0 || (len(files) == 1 && files[0] == "") {
				vError, _ := app.gui.View("varLogs")
				vError.Clear()
				// Меняем цвет окна на красный
				app.fileSystemFrameColor = gocui.ColorRed
				vError.FrameColor = app.fileSystemFrameColor
				// Отключаем курсор и выводим сообщение об ошибке
				vError.Highlight = false
				fmt.Fprintln(vError, "\033[31mPermission denied (files not found)\033[0m")
				return
			} else {
				vError, _ := app.gui.View("varLogs")
				app.fileSystemFrameColor = gocui.ColorDefault
				if vError.FrameColor != gocui.ColorDefault {
					vError.FrameColor = gocui.ColorGreen
				}
				vError.Highlight = true
			}
		} else {
			if len(files) == 0 || (len(files) == 1 && files[0] == "") {
				log.Print("Error: permission denied (files not found from descriptor)")
			}
		}
		// Очищаем массив перед добавлением отфильтрованных файлов
		output = []byte{}
		// Фильтруем строки, которые заканчиваются на ".log" и удаляем префикс (имя файла)
		for _, file := range files {
			if strings.HasSuffix(file, ".log") {
				file = strings.TrimPrefix(file, "n")
				output = append(output, []byte(file+"\n")...)
			}
		}
	case "/var/log/":
		var cmd *exec.Cmd
		// Загрузка системных журналов для macOS
		if app.getOS == "darwin" {
			args := []string{
				logPath, "/Library/Logs",
				"-type", "f",
				"-name", "*.asl", "-o",
				"-name", "*.log", "-o",
				"-name", "*log*", "-o",
				"-name", "*.[0-9]*", "-o",
				"-name", "*.[0-9].*", "-o",
				"-name", "*.pcap", "-o",
				"-name", "*.pcap.gz", "-o",
				"-name", "*.pcapng", "-o",
				"-name", "*.pcapng.gz",
			}
			if app.sshMode {
				sshArgs := app.sshOptions
				sshArgs = append(sshArgs, "find")
				sshArgs = append(sshArgs, args...)
				cmd = exec.Command("ssh", sshArgs...)
			} else {
				cmd = exec.Command("find", args...)
			}
		} else {
			// Загрузка системных журналов для Linux: все файлы, которые содержат log в расширение или названии (архивы включительно), а также расширение с цифрой (архивные) и pcap/pcapng
			args := []string{
				logPath,
				"-type", "f",
				"-name", "*.log", "-o",
				"-name", "*log*", "-o",
				"-name", "*.[0-9]*", "-o",
				"-name", "*.[0-9].*", "-o",
				"-name", "*.pcap", "-o",
				"-name", "*.pcap", "-o",
				"-name", "*.pcap.gz", "-o",
				"-name", "*.pcapng", "-o",
				"-name", "*.pcapng.gz",
			}
			if app.sshMode {
				sshArgs := app.sshOptions
				sshArgs = append(sshArgs, "find")
				sshArgs = append(sshArgs, args...)
				cmd = exec.Command("ssh", sshArgs...)
			} else {
				cmd = exec.Command("find", args...)
			}
		}
		output, _ = cmd.Output()
		// Преобразуем вывод команды в строку и делим на массив строк
		files := strings.Split(strings.TrimSpace(string(output)), "\n")
		// Если список файлов пустой, возвращаем ошибку Permission denied
		if !app.testMode {
			if len(files) == 0 || (len(files) == 1 && files[0] == "") {
				vError, _ := app.gui.View("varLogs")
				vError.Clear()
				// Меняем цвет окна на красный
				app.fileSystemFrameColor = gocui.ColorRed
				vError.FrameColor = app.fileSystemFrameColor
				// Отключаем курсор и выводим сообщение об ошибке
				vError.Highlight = false
				fmt.Fprintln(vError, "\033[31mPermission denied (files not found)\033[0m")
				return
			} else {
				vError, _ := app.gui.View("varLogs")
				app.fileSystemFrameColor = gocui.ColorDefault
				if vError.FrameColor != gocui.ColorDefault {
					vError.FrameColor = gocui.ColorGreen
				}
				vError.Highlight = true
			}
		} else {
			if len(files) == 0 || (len(files) == 1 && files[0] == "") {
				log.Print("Error: files not found in /var/log")
			}
		}
		// Добавляем пути по умолчанию для /var/log
		logPaths := []string{
			// Ядро
			"/var/log/dmesg\n",
			// Информация о входах и выходах пользователей, перезагрузках и остановках системы
			"/var/log/wtmp\n",
			// Информация о неудачных попытках входа в систему (например, неправильные пароли)
			"/var/log/btmp\n",
			// Информация о текущих пользователях, их сеансах и входах в систему
			"/var/run/utmp\n",
			"/run/utmp\n",
			// macOS/BSD/RHEL
			"/var/log/secure\n",
			"/var/log/messages\n",
			"/var/log/daemon\n",
			"/var/log/lpd-errs\n",
			"/var/log/security.out\n",
			"/var/log/daily.out\n",
			// Службы
			"/var/log/cron\n",
			"/var/log/ftpd\n",
			"/var/log/ntpd\n",
			"/var/log/named\n",
			"/var/log/dhcpd\n",
		}
		for _, path := range logPaths {
			output = append([]byte(path), output...)
		}
	case "/opt/":
		var cmd *exec.Cmd
		if app.sshMode {
			cmd = exec.Command(
				"ssh", append(app.sshOptions,
					"find", logPath,
					"-type", "f",
					"-name", "*.log", "-o",
					"-name", "*.log.*",
				)...,
			)
		} else {
			cmd = exec.Command(
				"find", logPath,
				"-type", "f",
				"-name", "*.log", "-o",
				"-name", "*.log.*",
			)
		}
		output, _ = cmd.Output()
		files := strings.Split(strings.TrimSpace(string(output)), "\n")
		if !app.testMode {
			if len(files) == 0 || (len(files) == 1 && files[0] == "") {
				vError, _ := app.gui.View("varLogs")
				vError.Clear()
				// Меняем цвет окна на красный
				app.fileSystemFrameColor = gocui.ColorRed
				vError.FrameColor = app.fileSystemFrameColor
				// Отключаем курсор и выводим сообщение об ошибке
				vError.Highlight = false
				fmt.Fprintln(vError, "\033[31mFiles not found\033[0m")
				return
			} else {
				vError, _ := app.gui.View("varLogs")
				app.fileSystemFrameColor = gocui.ColorDefault
				if vError.FrameColor != gocui.ColorDefault {
					vError.FrameColor = gocui.ColorGreen
				}
				vError.Highlight = true
			}
		} else {
			if len(files) == 0 || (len(files) == 1 && files[0] == "") {
				log.Print("Error: files not found in /opt/")
			}
		}
	default:
		// Домашние каталоги пользователей: /home/ для Linux и /Users/ для macOS
		if app.getOS == "darwin" {
			logPath = "/Users/"
		}
		// Ищем файлы с помощью системной утилиты find
		var cmd *exec.Cmd
		args := []string{
			logPath,
			"-type", "d",
			"-name", "Library", "-o",
			"-name", "Pictures", "-o",
			"-name", "Movies", "-o",
			"-name", "Music", "-o",
			"-name", ".Trash", "-o",
			"-name", ".cache",
			"-prune", "-o",
			"-type", "f",
			"-name", "*.log", "-o",
			"-name", "*.asl", "-o",
			"-name", "*.pcap", "-o",
			"-name", "*.pcap.gz", "-o",
			"-name", "*.pcapng", "-o",
			"-name", "*.pcapng.gz",
		}
		if app.sshMode {
			sshArgs := app.sshOptions
			sshArgs = append(sshArgs, "find")
			sshArgs = append(sshArgs, args...)
			cmd = exec.Command("ssh", sshArgs...)
		} else {
			cmd = exec.Command("find", args...)
		}
		output, _ = cmd.Output()
		files := strings.Split(strings.TrimSpace(string(output)), "\n")
		if !app.testMode {
			if len(files) == 0 || (len(files) == 1 && files[0] == "") {
				vError, _ := app.gui.View("varLogs")
				vError.Clear()
				vError.Highlight = false
				fmt.Fprintln(vError, "\033[32mFiles not found\033[0m")
				return
			} else {
				vError, _ := app.gui.View("varLogs")
				app.fileSystemFrameColor = gocui.ColorDefault
				if vError.FrameColor != gocui.ColorDefault {
					vError.FrameColor = gocui.ColorGreen
				}
				vError.Highlight = true
			}
		} else {
			if len(files) == 0 || (len(files) == 1 && files[0] == "") {
				log.Print("Error: files not found in home directories")
			}
		}
		// Получаем содержимое файлов из домашнего каталога пользователя root
		var cmdRootDir *exec.Cmd
		args = []string{
			"/root/",
			"-type", "f",
			"-name", "*.log", "-o",
			"-name", "*.pcap", "-o",
			"-name", "*.pcap.gz", "-o",
			"-name", "*.pcapng", "-o",
			"-name", "*.pcapng.gz",
		}
		if app.sshMode {
			sshArgs := app.sshOptions
			sshArgs = append(sshArgs, "find")
			sshArgs = append(sshArgs, args...)
			cmdRootDir = exec.Command("ssh", sshArgs...)
		} else {
			cmdRootDir = exec.Command("find", args...)
		}
		outputRootDir, err := cmdRootDir.Output()
		// Добавляем содержимое директории /root/ в общий массив, если есть доступ
		if err == nil {
			output = append(output, outputRootDir...)
		}
		if app.fileSystemFrameColor == gocui.ColorRed && !app.testMode {
			vError, _ := app.gui.View("varLogs")
			app.fileSystemFrameColor = gocui.ColorDefault
			if vError.FrameColor != gocui.ColorDefault {
				vError.FrameColor = gocui.ColorGreen
			}
			vError.Highlight = true
		}
	}
	// Формируем массив путей
	logFullPaths := strings.Split(strings.TrimSpace(string(output)), "\n")
	// Получаем статистику по всем файлам одним вызовом в режиме ssh
	var statFiles map[string]os.FileInfo
	if app.sshMode {
		statFiles, _ = app.statFiles(logFullPaths)
	}
	// Карта уникальных путей
	serviceMap := make(map[string]bool)
	// Основной цикл
	for _, logFullPath := range logFullPaths {
		// Удаляем префикс пути и расширение файла в конце
		logName := logFullPath
		if logPath != "descriptor" {
			logName = strings.TrimPrefix(logFullPath, logPath)
		}
		logName = strings.TrimSuffix(logName, ".log")
		logName = strings.TrimSuffix(logName, ".asl")
		logName = strings.TrimSuffix(logName, ".gz")
		logName = strings.TrimSuffix(logName, ".xz")
		logName = strings.TrimSuffix(logName, ".bz2")
		logName = strings.ReplaceAll(logName, "/", " ")
		logName = strings.ReplaceAll(logName, ".log.", ".")
		logName = strings.TrimPrefix(logName, " ")
		if logPath == "/home/" || logPath == "/Users/" {
			// Разбиваем строку на слова
			words := strings.Fields(logName)
			// Берем первое и последнее слово
			firstWord := words[0]
			lastWord := words[len(words)-1]
			logName = "\x1b[0;33m" + firstWord + "\033[0m" + ": " + lastWord
		}
		// Получаем информацию о файле
		var fileInfo os.FileInfo
		var exists bool
		var err error
		if app.sshMode {
			// Извлекаем статистику из массива
			fileInfo, exists = statFiles[logFullPath]
			// Пропускаем файл, если он не найден в результатах
			if !exists {
				continue
			}
		} else {
			// Запрашиваем статистику для каждого файла в локальном режиме
			fileInfo, err = os.Stat(logFullPath)
			if err != nil {
				// Пропускаем файл, если к нему нет доступа (актуально для статических файлов из переменной logPath)
				continue
			}
		}
		// Проверяем, что файл не пустой
		if fileInfo.Size() == 0 {
			// Пропускаем пустой файл
			continue
		}
		// Получаем дату изменения
		modTime := fileInfo.ModTime()
		// Форматирование даты в формат DD.MM.YYYY HH:MM
		formattedDate := modTime.Format("02.01.2006 15:04")
		// Проверяем, что полного пути до файла еще нет в списке
		if logName != "" && !serviceMap[logFullPath] {
			// Добавляем путь в массив для проверки уникальных путей
			serviceMap[logFullPath] = true
			// Получаем имя процесса для файла дескриптора
			if logPath == "descriptor" {
				var cmd *exec.Cmd
				if app.sshMode {
					cmd = exec.Command("ssh", append(app.sshOptions, "lsof", "-Fc", logFullPath)...)
				} else {
					cmd = exec.Command("lsof", "-Fc", logFullPath)
				}
				cmd.Stderr = nil
				outputLsof, _ := cmd.Output()
				processLines := strings.Split(strings.TrimSpace(string(outputLsof)), "\n")
				// Ищем строку, которая содержит имя процесса (только первый процесс)
				for _, line := range processLines {
					if strings.HasPrefix(line, "c") {
						// Удаляем префикс
						processName := line[1:]
						logName = "\x1b[0;33m" + processName + "\033[0m" + ": " + logName
						break
					}
				}
			}
			// Выделение цветом подов и контейнеров k3s из файловой системы
			if strings.HasPrefix(logName, "pods") {
				logName = strings.Replace(logName, "pods", "\033[33mpod\033[0m", 1)
			}
			if strings.HasPrefix(logName, "containers") {
				logName = strings.Replace(logName, "containers", "\033[32mcontainer\033[0m", 1)
			}
			// Добавляем в список
			app.logfiles = append(app.logfiles, Logfile{
				name: "[" + "\033[34m" + formattedDate + "\033[0m" + "] " + logName,
				path: logFullPath,
			})
		}
	}
	// Сортируем по дате
	sort.Slice(app.logfiles, func(i, j int) bool {
		// Извлечение дат из имени
		layout := "02.01.2006 15:04"
		dateI, _ := time.Parse(layout, extractDate(app.logfiles[i].name))
		dateJ, _ := time.Parse(layout, extractDate(app.logfiles[j].name))
		// return dateI.Before(dateJ)
		// Сортировка в обратном порядке
		return dateI.After(dateJ)
	})
	if !app.testMode {
		app.logfilesNotFilter = app.logfiles
		app.applyFilterList()
	}
}

func (app *App) loadWinFiles(logPath string) {
	app.logfiles = nil
	// Определяем путь по параметру
	switch logPath {
	case "ProgramFiles":
		logPath = app.systemDisk + ":\\Program Files"
	case "ProgramFiles86":
		logPath = app.systemDisk + ":\\Program Files (x86)"
	case "ProgramData":
		logPath = app.systemDisk + ":\\ProgramData"
	case "AppDataLocal":
		logPath = app.systemDisk + ":\\Users\\" + app.userName + "\\AppData\\Local"
	case "AppDataRoaming":
		logPath = app.systemDisk + ":\\Users\\" + app.userName + "\\AppData\\Roaming"
	}
	// Ищем файлы с помощью WalkDir
	var files []string
	// Доступ к срезу files из нескольких горутин
	var mu sync.Mutex
	// Группа ожидания для отслеживания завершения всех горутин
	var wg sync.WaitGroup
	// Получаем список корневых директорий
	rootDirs, _ := os.ReadDir(logPath)
	for _, rootDir := range rootDirs {
		// Проверяем, является ли текущий элемент директорие
		if rootDir.IsDir() {
			// Увеличиваем счетчик ожидаемых горутин
			wg.Add(1)
			go func(dir string) {
				// Уменьшаем счетчик горутин после завершения текущей
				defer wg.Done()
				// Рекурсивно обходим все файлы и подкаталоги в текущей директории
				err := filepath.WalkDir(filepath.Join(logPath, dir), func(path string, d os.DirEntry, err error) error {
					if err != nil {
						// Игнорируем ошибки, чтобы не прерывать поиск
						return nil
					}
					// Проверяем, что текущий элемент не является директорией и имеет расширение .log
					if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".log") {
						// Получаем относительный путь (без корневого пути logPath)
						relPath, _ := filepath.Rel(logPath, path)
						// Используем мьютекс для добавления файла в срез
						mu.Lock()
						files = append(files, relPath)
						mu.Unlock()
					}
					return nil
				})
				if err != nil {
					return
				}
			}(
				// Передаем имя текущей директории в горутину
				rootDir.Name(),
			)
		}
	}
	// Ждем завершения всех запущенных горутин
	wg.Wait()
	// Объединяем все пути в одну строку, разделенную символом новой строки
	output := strings.Join(files, "\n")
	if !app.testMode {
		// Если список файлов пустой, возвращаем ошибку
		if len(files) == 0 || (len(files) == 1 && files[0] == "") {
			vError, _ := app.gui.View("varLogs")
			vError.Clear()
			app.fileSystemFrameColor = gocui.ColorRed
			vError.FrameColor = app.fileSystemFrameColor
			vError.Highlight = false
			fmt.Fprintln(vError, "\033[31mPermission denied (files not found)\033[0m")
			return
		} else {
			vError, _ := app.gui.View("varLogs")
			app.fileSystemFrameColor = gocui.ColorDefault
			if vError.FrameColor != gocui.ColorDefault {
				vError.FrameColor = gocui.ColorGreen
			}
			vError.Highlight = true
		}
	} else {
		if len(files) == 0 || (len(files) == 1 && files[0] == "") {
			log.Print("Error: files not found in ", logPath)
		}
	}
	serviceMap := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		// Формируем полный путь к файлу
		logFullPath := logPath + "\\" + scanner.Text()
		// Формируем имя файла для списка
		logName := scanner.Text()
		logName = strings.TrimSuffix(logName, ".log")
		logName = strings.ReplaceAll(logName, "\\", " ")
		// Получаем информацию о файле
		fileInfo, err := os.Stat(logFullPath)
		// Пропускаем файлы, к которым нет доступа
		if err != nil {
			continue
		}
		// Пропускаем пустые файлы
		if fileInfo.Size() == 0 {
			continue
		}
		// Получаем дату изменения
		modTime := fileInfo.ModTime()
		// Форматирование даты в формат DD.MM.YYYY HH:MM
		formattedDate := modTime.Format("02.01.2006 15:04")
		// Проверяем, что полного пути до файла еще нет в списке
		if logName != "" && !serviceMap[logFullPath] {
			// Добавляем путь в массив для проверки уникальных путей
			serviceMap[logFullPath] = true
			// Добавляем в список
			app.logfiles = append(app.logfiles, Logfile{
				name: "[" + "\033[34m" + formattedDate + "\033[0m" + "] " + logName,
				path: logFullPath,
			})
		}
	}
	// Сортируем по дате
	sort.Slice(app.logfiles, func(i, j int) bool {
		layout := "02.01.2006 15:04"
		dateI, _ := time.Parse(layout, extractDate(app.logfiles[i].name))
		dateJ, _ := time.Parse(layout, extractDate(app.logfiles[j].name))
		return dateI.After(dateJ)
	})
	if !app.testMode {
		app.logfilesNotFilter = app.logfiles
		app.applyFilterList()
	}
}

// Функция для извлечения первой втречающейся даты в формате DD.MM.YYYY HH:MM
func extractDate(name string) string {
	re := regexp.MustCompile(`\d{2}\.\d{2}\.\d{4}\s\d{2}:\d{2}`)
	return re.FindString(name)
}

func (app *App) updateLogsList() {
	v, err := app.gui.View("varLogs")
	if err != nil {
		return
	}
	v.Clear()
	visibleEnd := app.startFiles + app.maxVisibleFiles
	if visibleEnd > len(app.logfiles) {
		visibleEnd = len(app.logfiles)
	}
	for i := app.startFiles; i < visibleEnd; i++ {
		fmt.Fprintln(v, app.logfiles[i].name)
	}
}

func (app *App) nextFileName(v *gocui.View, step int) error {
	_, viewHeight := v.Size()
	app.maxVisibleFiles = viewHeight
	if len(app.logfiles) == 0 {
		return nil
	}
	if app.selectedFile < len(app.logfiles)-1 {
		app.selectedFile += step
		if app.selectedFile >= len(app.logfiles) {
			app.selectedFile = len(app.logfiles) - 1
		}
		if app.selectedFile >= app.startFiles+app.maxVisibleFiles {
			app.startFiles += step
			if app.startFiles > len(app.logfiles)-app.maxVisibleFiles {
				app.startFiles = len(app.logfiles) - app.maxVisibleFiles
			}
			app.updateLogsList()
		}
		if app.selectedFile < app.startFiles+app.maxVisibleFiles {
			return app.selectFileByIndex(app.selectedFile - app.startFiles)
		}
	}
	return nil
}

func (app *App) prevFileName(v *gocui.View, step int) error {
	_, viewHeight := v.Size()
	app.maxVisibleFiles = viewHeight
	if len(app.logfiles) == 0 {
		return nil
	}
	if app.selectedFile > 0 {
		app.selectedFile -= step
		if app.selectedFile < 0 {
			app.selectedFile = 0
		}
		if app.selectedFile < app.startFiles {
			app.startFiles -= step
			if app.startFiles < 0 {
				app.startFiles = 0
			}
			app.updateLogsList()
		}
		if app.selectedFile >= app.startFiles {
			return app.selectFileByIndex(app.selectedFile - app.startFiles)
		}
	}
	return nil
}

func (app *App) selectFileByIndex(index int) error {
	v, err := app.gui.View("varLogs")
	if err != nil {
		return err
	}
	// Обновляем счетчик в заголовке
	re := regexp.MustCompile(`\s\(.+\) >`)
	updateTitle := " (0) >"
	if len(app.logfiles) != 0 {
		updateTitle = " (" + strconv.Itoa(app.selectedFile+1) + "/" + strconv.Itoa(len(app.logfiles)) + ") >"
	}
	v.Title = re.ReplaceAllString(v.Title, updateTitle)
	if err := v.SetCursor(0, index); err != nil {
		return nil
	}
	return nil
}

func (app *App) selectFile(g *gocui.Gui, v *gocui.View) error {
	if v == nil || len(app.logfiles) == 0 {
		return nil
	}
	_, cy := v.Cursor()
	line, err := v.Line(cy)
	if err != nil {
		return err
	}
	if app.fastMode {
		go func() {
			app.loadFileLogs(strings.TrimSpace(line), true)
		}()
	} else {
		app.loadFileLogs(strings.TrimSpace(line), true)
	}
	app.lastWindow = "varLogs"
	app.lastSelected = strings.TrimSpace(line)
	return nil
}

// Функция для чтения файла
func (app *App) loadFileLogs(logName string, newUpdate bool) {
	app.debugStartTime = time.Now()
	// В параметре logName имя файла при выборе возвращяется без символов покраски
	// Получаем путь из массива по имени
	var logFullPath string
	var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	for _, logfile := range app.logfiles {
		// Удаляем покраску из имени файла в сохраненном массиве
		logFileName := ansiEscape.ReplaceAllString(logfile.name, "")
		// Ищем переданное в функцию имя файла и извлекаем путь
		if logFileName == logName {
			logFullPath = logfile.path
			break
		}
	}
	if newUpdate {
		app.lastLogPath = logFullPath
		// Фиксируем новую дату изменения и размер для выбранного файла
		fileInfo, err := app.statFile(logFullPath)
		if err != nil {
			return
		}
		fileModTime := fileInfo.ModTime()
		fileSize := fileInfo.Size()
		app.lastDateUpdateFile = fileModTime
		app.lastSizeFile = fileSize
		app.updateFile = true
	} else {
		logFullPath = app.lastLogPath
		// Проверяем дату изменения
		fileInfo, err := app.statFile(logFullPath)
		if err != nil {
			return
		}
		fileModTime := fileInfo.ModTime()
		fileSize := fileInfo.Size()
		// Обновлять файл в горутине, только если есть изменения (проверяем дату модификации и размер)
		if fileModTime != app.lastDateUpdateFile || fileSize != app.lastSizeFile {
			app.lastDateUpdateFile = fileModTime
			app.lastSizeFile = fileSize
			app.updateFile = true
		} else {
			app.updateFile = false
		}
	}
	// Читаем файл, толькое если были изменения
	if app.updateFile {
		// Читаем логи в системе Windows
		if app.getOS == "windows" {
			decodedOutput, stringErrors := app.loadWinFileLog(logFullPath)
			if stringErrors != "nil" && !app.testMode {
				v, _ := app.gui.View("logs")
				v.Clear()
				fmt.Fprintln(v, "\033[31mError", stringErrors, "\033[0m")
				return
			}
			if stringErrors != "nil" && app.testMode {
				log.Print("Error: ", stringErrors)
			}
			app.currentLogLines = strings.Split(string(decodedOutput), "\n")
		} else {
			var cmd *exec.Cmd
			// Читаем логи в системах UNIX (Linux/Darwin/*BSD)
			switch {
			// Читаем файлы в формате ASL (Apple System Log)
			case strings.HasSuffix(logFullPath, "asl"):
				if app.sshMode {
					cmd = exec.Command("ssh", append(app.sshOptions, "syslog", "-f", logFullPath)...)
				} else {
					cmd = exec.Command("syslog", "-f", logFullPath)
				}
				output, err := cmd.Output()
				if err != nil && !app.testMode {
					v, _ := app.gui.View("logs")
					v.Clear()
					fmt.Fprintln(v, " \033[31mError reading log using syslog tool in ASL (Apple System Log) format.\n", err, "\033[0m")
					return
				}
				if err != nil && app.testMode {
					log.Print("Error: reading log using syslog tool in ASL (Apple System Log) format. ", err)
				}
				app.currentLogLines = strings.Split(string(output), "\n")
			// Читаем журналы Packet Capture в формате pcap/pcapng
			case strings.HasSuffix(logFullPath, "pcap") || strings.HasSuffix(logFullPath, "pcapng"):
				if app.sshMode {
					cmd = exec.Command("ssh", append(app.sshOptions, "tcpdump", "-n", "-r", logFullPath)...)
				} else {
					cmd = exec.Command("tcpdump", "-n", "-r", logFullPath)
				}
				output, err := cmd.Output()
				if err != nil && !app.testMode {
					v, _ := app.gui.View("logs")
					v.Clear()
					fmt.Fprintln(v, " \033[31mError reading log using tcpdump tool.\n", err, "\033[0m")
					return
				}
				if err != nil && app.testMode {
					log.Print("Error: reading log using tcpdump tool. ", err)
				}
				app.currentLogLines = strings.Split(string(output), "\n")
			// Packet Filter (PF) Firewall OpenBSD
			case strings.HasSuffix(logFullPath, "pflog"):
				if app.sshMode {
					cmd = exec.Command("ssh", append(app.sshOptions, "tcpdump", "-e", "-n", "-r", logFullPath)...)
				} else {
					cmd = exec.Command("tcpdump", "-e", "-n", "-r", logFullPath)
				}
				output, err := cmd.Output()
				if err != nil && !app.testMode {
					v, _ := app.gui.View("logs")
					v.Clear()
					fmt.Fprintln(v, " \033[31mError reading log using tcpdump tool.\n", err, "\033[0m")
					return
				}
				app.currentLogLines = strings.Split(string(output), "\n")
			// Читаем архивные логи в формате pcap/pcapng (macOS)
			case strings.HasSuffix(logFullPath, "pcap.gz") || strings.HasSuffix(logFullPath, "pcapng.gz"):
				var unpacker string = "gzip"
				// Создаем временный файл
				tmpFile, err := os.CreateTemp("", "temp-*.pcap")
				if err != nil && !app.testMode {
					vError, _ := app.gui.View("logs")
					vError.Clear()
					fmt.Fprintln(vError, " \033[31mError create temp file.\n", err, "\033[0m")
					return
				}
				// Удаляем временный файл после обработки
				defer os.Remove(tmpFile.Name())
				var cmdUnzip *exec.Cmd
				if app.sshMode {
					cmdUnzip = exec.Command("ssh", append(app.sshOptions, unpacker, "-dc", logFullPath)...)
				} else {
					cmdUnzip = exec.Command(unpacker, "-dc", logFullPath)
				}
				cmdUnzip.Stdout = tmpFile
				if err := cmdUnzip.Start(); err != nil && !app.testMode {
					vError, _ := app.gui.View("logs")
					vError.Clear()
					fmt.Fprintln(vError, " \033[31mError starting", unpacker, "tool.\n", err, "\033[0m")
					return
				}
				if err := cmdUnzip.Wait(); err != nil && !app.testMode {
					vError, _ := app.gui.View("logs")
					vError.Clear()
					fmt.Fprintln(vError, " \033[31mError decompressing file with", unpacker, "tool.\n", err, "\033[0m")
					return
				}
				// Закрываем временный файл, чтобы tcpdump мог его открыть
				if err := tmpFile.Close(); err != nil && !app.testMode {
					vError, _ := app.gui.View("logs")
					vError.Clear()
					fmt.Fprintln(vError, " \033[31mError closing temp file.\n", err, "\033[0m")
					return
				}
				// Создаем команду для tcpdump
				var cmdTcpdump *exec.Cmd
				if app.sshMode {
					cmdTcpdump = exec.Command("ssh", append(app.sshOptions, "tcpdump", "-n", "-r", tmpFile.Name())...)
				} else {
					cmdTcpdump = exec.Command("tcpdump", "-n", "-r", tmpFile.Name())
				}
				tcpdumpOut, err := cmdTcpdump.StdoutPipe()
				if err != nil && !app.testMode {
					vError, _ := app.gui.View("logs")
					vError.Clear()
					fmt.Fprintln(vError, " \033[31mError creating stdout pipe for tcpdump.\n", err, "\033[0m")
					return
				}
				// Запускаем tcpdump
				if err := cmdTcpdump.Start(); err != nil && !app.testMode {
					vError, _ := app.gui.View("logs")
					vError.Clear()
					fmt.Fprintln(vError, " \033[31mError starting tcpdump.\n", err, "\033[0m")
					return
				}
				// Читаем вывод tcpdump построчно
				scanner := bufio.NewScanner(tcpdumpOut)
				var lines []string
				for scanner.Scan() {
					lines = append(lines, scanner.Text())
				}
				if err := scanner.Err(); err != nil && !app.testMode {
					vError, _ := app.gui.View("logs")
					vError.Clear()
					fmt.Fprintln(vError, " \033[31mError reading output from tcpdump.\n", err, "\033[0m")
					return
				}
				// Ожидаем завершения tcpdump
				if err := cmdTcpdump.Wait(); err != nil && !app.testMode {
					vError, _ := app.gui.View("logs")
					vError.Clear()
					fmt.Fprintln(vError, " \033[31mError finishing tcpdump.\n", err, "\033[0m")
					return
				}
				app.currentLogLines = lines
			// Читаем архивные логи (unpack + stdout) в формате: gz/xz/bz2
			case strings.HasSuffix(logFullPath, ".gz") || strings.HasSuffix(logFullPath, ".xz") || strings.HasSuffix(logFullPath, ".bz2"):
				var unpacker string
				switch {
				case strings.HasSuffix(logFullPath, ".gz"):
					unpacker = "gzip"
				case strings.HasSuffix(logFullPath, ".xz"):
					unpacker = "xz"
				case strings.HasSuffix(logFullPath, ".bz2"):
					unpacker = "bzip2"
				}
				var cmdUnzip *exec.Cmd
				var cmdTail *exec.Cmd
				if app.sshMode {
					cmdUnzip = exec.Command("ssh", append(app.sshOptions, unpacker, "-dc", logFullPath)...)
					cmdTail = exec.Command("ssh", append(app.sshOptions, "tail", "-n", app.logViewCount)...)
				} else {
					cmdUnzip = exec.Command(unpacker, "-dc", logFullPath)
					cmdTail = exec.Command("tail", "-n", app.logViewCount)
				}
				pipe, err := cmdUnzip.StdoutPipe()
				if err != nil && !app.testMode {
					vError, _ := app.gui.View("logs")
					vError.Clear()
					fmt.Fprintln(vError, " \033[31mError creating pipe for", unpacker, "tool.\n", err, "\033[0m")
					return
				}
				// Стандартный вывод программы передаем в stdin tail
				cmdTail.Stdin = pipe
				out, err := cmdTail.StdoutPipe()
				if err != nil && !app.testMode {
					vError, _ := app.gui.View("logs")
					vError.Clear()
					fmt.Fprintln(vError, " \033[31mError creating stdout pipe for tail.\n", err, "\033[0m")
					return
				}
				// Запуск команд
				if err := cmdUnzip.Start(); err != nil && !app.testMode {
					vError, _ := app.gui.View("logs")
					vError.Clear()
					fmt.Fprintln(vError, " \033[31mError starting", unpacker, "tool.\n", err, "\033[0m")
					return
				}
				if err := cmdTail.Start(); err != nil && !app.testMode {
					vError, _ := app.gui.View("logs")
					vError.Clear()
					fmt.Fprintln(vError, " \033[31mError starting tail from", unpacker, "stdout.\n", err, "\033[0m")
					return
				}
				// Чтение вывода
				output, err := io.ReadAll(out)
				if err != nil && !app.testMode {
					vError, _ := app.gui.View("logs")
					vError.Clear()
					fmt.Fprintln(vError, " \033[31mError reading output from tail.\n", err, "\033[0m")
					return
				}
				// Ожидание завершения команд
				if err := cmdUnzip.Wait(); err != nil && !app.testMode {
					vError, _ := app.gui.View("logs")
					vError.Clear()
					fmt.Fprintln(vError, " \033[31mError reading archive log using", unpacker, "tool.\n", err, "\033[0m")
					return
				}
				if err := cmdTail.Wait(); err != nil && !app.testMode {
					vError, _ := app.gui.View("logs")
					vError.Clear()
					fmt.Fprintln(vError, " \033[31mError reading log using tail tool.\n", err, "\033[0m")
					return
				}
				// Выводим содержимое
				app.currentLogLines = strings.Split(string(output), "\n")
			// Читаем бинарные файлы с помощью last для wtmp, а также utmp (OpenBSD) и utx.log (FreeBSD)
			case strings.Contains(logFullPath, "wtmp") || strings.Contains(logFullPath, "utmp") || strings.Contains(logFullPath, "utx.log"):
				if app.sshMode {
					cmd = exec.Command("ssh", append(app.sshOptions, "last", "-f", logFullPath)...)
				} else {
					cmd = exec.Command("last", "-f", logFullPath)
				}
				output, err := cmd.Output()
				if err != nil && !app.testMode {
					v, _ := app.gui.View("logs")
					v.Clear()
					fmt.Fprintln(v, " \033[31mError reading log using last tool.\n", err, "\033[0m")
					return
				}
				// Разбиваем вывод на строки
				lines := strings.Split(string(output), "\n")
				var filteredLines []string
				// Фильтруем строки, исключая последнюю строку и пустые строки
				for _, line := range lines {
					trimmedLine := strings.TrimSpace(line)
					if trimmedLine != "" && !strings.Contains(trimmedLine, "begins") {
						filteredLines = append(filteredLines, trimmedLine)
					}
				}
				// Переворачиваем порядок строк
				for i, j := 0, len(filteredLines)-1; i < j; i, j = i+1, j-1 {
					filteredLines[i], filteredLines[j] = filteredLines[j], filteredLines[i]
				}
				app.currentLogLines = filteredLines
			// lastb for btmp
			case strings.Contains(logFullPath, "btmp"):
				if app.sshMode {
					cmd = exec.Command("ssh", append(app.sshOptions, "lastb", "-f", logFullPath)...)
				} else {
					cmd = exec.Command("lastb", "-f", logFullPath)
				}
				output, err := cmd.Output()
				if err != nil && !app.testMode {
					v, _ := app.gui.View("logs")
					v.Clear()
					fmt.Fprintln(v, " \033[31mError reading log using lastb tool.\n", err, "\033[0m")
					return
				}
				lines := strings.Split(string(output), "\n")
				var filteredLines []string
				for _, line := range lines {
					trimmedLine := strings.TrimSpace(line)
					if trimmedLine != "" && !strings.Contains(trimmedLine, "begins") {
						filteredLines = append(filteredLines, trimmedLine)
					}
				}
				for i, j := 0, len(filteredLines)-1; i < j; i, j = i+1, j-1 {
					filteredLines[i], filteredLines[j] = filteredLines[j], filteredLines[i]
				}
				app.currentLogLines = filteredLines
			// Выводим содержимое из команды lastlog
			case strings.HasSuffix(logFullPath, "lastlog"):
				if app.sshMode {
					cmd = exec.Command("ssh", append(app.sshOptions, "lastlog")...)
				} else {
					cmd = exec.Command("lastlog")
				}
				output, err := cmd.Output()
				if err != nil && !app.testMode {
					v, _ := app.gui.View("logs")
					v.Clear()
					fmt.Fprintln(v, " \033[31mError reading log using lastlog tool.\n", err, "\033[0m")
					return
				}
				app.currentLogLines = strings.Split(string(output), "\n")
			// lastlogin for FreeBSD
			case strings.HasSuffix(logFullPath, "lastlogin"):
				if app.sshMode {
					cmd = exec.Command("ssh", append(app.sshOptions, "lastlogin")...)
				} else {
					cmd = exec.Command("lastlogin")
				}
				output, err := cmd.Output()
				if err != nil && !app.testMode {
					v, _ := app.gui.View("logs")
					v.Clear()
					fmt.Fprintln(v, " \033[31mError reading log using lastlogin tool.\n", err, "\033[0m")
					return
				}
				app.currentLogLines = strings.Split(string(output), "\n")
			default:
				if app.sshMode {
					cmd = exec.Command("ssh", append(app.sshOptions, "tail", "-n", app.logViewCount, logFullPath)...)
				} else {
					cmd = exec.Command("tail", "-n", app.logViewCount, logFullPath)
				}
				output, err := cmd.Output()
				if err != nil && !app.testMode {
					v, _ := app.gui.View("logs")
					v.Clear()
					fmt.Fprintln(v, " \033[31mError reading log using tail tool.\n", err, "\033[0m")
					return
				}
				app.currentLogLines = strings.Split(string(output), "\n")
			}
		}
		if !app.testMode {
			app.updateDelimiter(newUpdate)
			app.applyFilter(false)
		}
	}
}

// Функция для чтения файла с опредилением кодировки в Windows
func (app *App) loadWinFileLog(filePath string) (output []byte, stringErrors string) {
	app.debugStartTime = time.Now()
	// Открываем файл
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Sprintf("open file: %v", err)
	}
	defer file.Close()
	// Получаем информацию о файле
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Sprintf("get file stat: %v", err)
	}
	// Получаем размер файла
	fileSize := stat.Size()
	// Буфер для хранения последних строк
	var buffer []byte
	lineCount := 0
	// Размер буфера чтения (читаем по 1КБ за раз)
	readSize := int64(1024)
	// Преобразуем строку с максимальным количеством строк в int
	logViewCountInt, _ := strconv.Atoi(app.logViewCount)
	// Читаем файл с конца
	for fileSize > 0 && lineCount < logViewCountInt {
		if fileSize < readSize {
			readSize = fileSize
		}
		_, err := file.Seek(fileSize-readSize, 0)
		if err != nil {
			return nil, fmt.Sprintf("detect the end of a file via seek: %v", err)
		}
		tempBuffer := make([]byte, readSize)
		_, err = file.Read(tempBuffer)
		if err != nil {
			return nil, fmt.Sprintf("read file: %v", err)
		}
		buffer = append(tempBuffer, buffer...)
		lineCount = strings.Count(string(buffer), "\n")
		fileSize -= int64(readSize)
	}
	// Проверка на UTF-16 с BOM
	utf16withBOM := func(data []byte) bool {
		return len(data) >= 2 && ((data[0] == 0xFF && data[1] == 0xFE) || (data[0] == 0xFE && data[1] == 0xFF))
	}
	// Проверка на UTF-16 LE без BOM
	utf16withoutBOM := func(data []byte) bool {
		if len(data)%2 != 0 {
			return false
		}
		for i := 1; i < len(data); i += 2 {
			if data[i] != 0x00 {
				return false
			}
		}
		return true
	}
	var decodedOutput []byte
	switch {
	case utf16withBOM(buffer):
		// Декодируем UTF-16 с BOM
		decodedOutput, err = unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder().Bytes(buffer)
		if err != nil {
			return nil, fmt.Sprintf("decoding from UTF-16 with BOM: %v", err)
		}
	case utf16withoutBOM(buffer):
		// Декодируем UTF-16 LE без BOM
		decodedOutput, err = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder().Bytes(buffer)
		if err != nil {
			return nil, fmt.Sprintf("decoding from UTF-16 LE without BOM: %v", err)
		}
	case utf8.Valid(buffer):
		// Декодируем UTF-8
		decodedOutput = buffer
	default:
		// Декодируем Windows-1251
		decodedOutput, err = charmap.Windows1251.NewDecoder().Bytes(buffer)
		if err != nil {
			return nil, fmt.Sprintf("decoding from Windows-1251: %v", err)
		}
	}
	return decodedOutput, "nil"
}

// ---------------------------------------- Docker/Compose/Podman/k8s ----------------------------------------

func (app *App) loadDockerContainer(containerizationSystem string) {
	app.dockerContainers = nil
	// Получаем версию для проверки, что система контейнеризации установлена
	var cmd *exec.Cmd
	if app.sshMode {
		// Для compose передаем два аргумента команды (проверяем compose как плагин docker)
		if containerizationSystem == "compose" {
			cmd = exec.Command("ssh", append(app.sshOptions,
				"docker", "compose", "version",
			)...)
		} else {
			cmd = exec.Command("ssh", append(app.sshOptions,
				containerizationSystem, "version",
			)...)
		}
	} else {
		if containerizationSystem == "compose" {
			cmd = exec.Command(
				"docker", "compose", "version",
			)
		} else {
			cmd = exec.Command(
				containerizationSystem, "version",
			)
		}
	}
	_, err := cmd.Output()
	if err != nil && !app.testMode {
		vError, _ := app.gui.View("docker")
		vError.Clear()
		app.dockerFrameColor = gocui.ColorRed
		vError.FrameColor = app.dockerFrameColor
		vError.Highlight = false
		fmt.Fprintln(vError, "\033[31m"+containerizationSystem+" not installed (environment not found)\033[0m")
		return
	}
	if err != nil && app.testMode {
		log.Print("Error:", containerizationSystem+" not installed (environment not found)")
	}
	switch containerizationSystem {
	case "kubectl":
		// Получаем список подов из k8s
		if app.sshMode {
			cmd = exec.Command("ssh", append(app.sshOptions,
				containerizationSystem, "get", "pods", "-A",
				"-o", "'jsonpath={range .items[*]}{.metadata.uid} {.metadata.name} {.status.phase} {.metadata.namespace}{\"\\n\"}{end}'",
			)...)
		} else {
			cmd = exec.Command(
				containerizationSystem, "get", "pods", "-A", // -A/--all-namespaces
				"-o", "jsonpath={range .items[*]}{.metadata.uid} {.metadata.name} {.status.phase} {.metadata.namespace}{\"\\n\"}{end}",
			)
		}
	case "compose":
		if app.sshMode {
			cmd = exec.Command("ssh", append(app.sshOptions,
				"docker", "compose", "ls", "-a",
			)...)
		} else {
			cmd = exec.Command(
				"docker", "compose", "ls", "-a",
			)
		}
	default:
		// Получаем список контейнеров из Docker или Podman
		if app.sshMode {
			cmd = exec.Command("ssh", append(app.sshOptions,
				containerizationSystem, "ps", "-a",
				"--format", "'{{.ID}} {{.Names}} {{.State}}'", // добавляем кавычки для передаваемых через пробел параметров в ssh
			)...)
		} else {
			cmd = exec.Command(
				containerizationSystem, "ps", "-a",
				"--format", "{{.ID}} {{.Names}} {{.State}}",
			)
		}
	}
	output, err := cmd.Output()
	if !app.testMode {
		if err != nil {
			vError, _ := app.gui.View("docker")
			vError.Clear()
			app.dockerFrameColor = gocui.ColorRed
			vError.FrameColor = app.dockerFrameColor
			vError.Highlight = false
			fmt.Fprintln(vError, "\033[31mAccess denied or "+containerizationSystem+" not running\033[0m")
			return
		} else {
			vError, _ := app.gui.View("docker")
			app.dockerFrameColor = gocui.ColorDefault
			vError.Highlight = true
			if vError.FrameColor != gocui.ColorDefault {
				vError.FrameColor = gocui.ColorGreen
			}
		}
	}
	if err != nil && app.testMode {
		log.Print("Error: access denied or " + containerizationSystem + " not running")
	}
	var containers []string
	var stringOutput string
	// Парсим вывод compose
	if containerizationSystem == "compose" {
		stacks := strings.Split(strings.TrimSpace(string(output)), "\n")
		// Удаляем первую строку (элемент массива)
		stacks = stacks[1:]
		if len(stacks) != 0 {
			// Удаляем путь к конфигурационному файлу compose для каждой строки (элемента)
			for i, e := range stacks {
				line := strings.Split(e, "/")
				stacks[i] = line[0]
			}
		}
		containers = stacks
		stringOutput = strings.Join(containers, "\n")
	} else {
		containers = strings.Split(strings.TrimSpace(string(output)), "\n")
		stringOutput = string(output)
	}
	// Проверяем, что список контейнеров не пустой
	if !app.testMode {
		if len(containers) == 0 || (len(containers) == 1 && containers[0] == "") {
			vError, _ := app.gui.View("docker")
			vError.Clear()
			vError.Highlight = false
			fmt.Fprintln(vError, "\033[32mNo running containers\033[0m")
			return
		} else {
			vError, _ := app.gui.View("docker")
			app.fileSystemFrameColor = gocui.ColorDefault
			if vError.FrameColor != gocui.ColorDefault {
				vError.FrameColor = gocui.ColorGreen
			}
			vError.Highlight = true
		}
	}
	// Заполняем структуру dockerContainers (название и статус)
	serviceMap := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(stringOutput))
	for scanner.Scan() {
		idName := scanner.Text()
		parts := strings.Fields(idName)
		if idName != "" && !serviceMap[idName] {
			serviceMap[idName] = true
			var containerName string
			var containerStatus string
			if containerizationSystem == "compose" {
				// Извлекаем имя стеке из первого параметра (в compose отсутствует id)
				containerName = parts[0]
				// Собираем все статусы в одну строку
				containerStatus = strings.Join(parts[1:], "\n")
			} else {
				containerName = parts[1]
				containerStatus = parts[2]
			}
			// Проверяем статус для покраски
			switch {
			case strings.HasPrefix(containerStatus, "running") || strings.EqualFold(containerStatus, "running"):
				containerStatus = "\033[32m" + containerStatus + "\033[0m"
			case strings.EqualFold(containerStatus, "succeeded"):
				containerStatus = "\033[33m" + containerStatus + "\033[0m"
			default:
				containerStatus = "\033[31m" + containerStatus + "\033[0m"
			}
			containerName = "[" + containerStatus + "] " + containerName
			// Фиксируем название namespace для k8s
			var namespace string
			if containerizationSystem != "kubectl" || parts[3] == "" {
				namespace = ""
			} else {
				namespace = parts[3]
			}
			app.dockerContainers = append(app.dockerContainers, DockerContainers{
				name:      containerName,
				id:        parts[0],
				namespace: namespace,
			})
		}
	}
	sort.Slice(app.dockerContainers, func(i, j int) bool {
		return app.dockerContainers[i].name < app.dockerContainers[j].name
	})
	if !app.testMode {
		app.dockerContainersNotFilter = app.dockerContainers
		app.applyFilterList()
	}
	// Заполняем карту уникальных цветов для контейнеров (используется для покраски префиксов в compose)
	if containerizationSystem == "docker" {
		for _, dc := range app.dockerContainers {
			cn := strings.SplitN(dc.name, "] ", 2)[1]
			if cn != "" {
				newColor := app.uniquePrefixColorArr[len(app.uniquePrefixColorMap)%len(app.uniquePrefixColorArr)]
				app.uniquePrefixColorMap[cn] = newColor
			}
		}
	}
}

func (app *App) updateDockerContainerList() {
	v, err := app.gui.View("docker")
	if err != nil {
		return
	}
	v.Clear()
	visibleEnd := app.startDockerContainers + app.maxVisibleDockerContainers
	if visibleEnd > len(app.dockerContainers) {
		visibleEnd = len(app.dockerContainers)
	}
	for i := app.startDockerContainers; i < visibleEnd; i++ {
		fmt.Fprintln(v, app.dockerContainers[i].name)
	}
}

func (app *App) nextDockerContainer(v *gocui.View, step int) error {
	_, viewHeight := v.Size()
	app.maxVisibleDockerContainers = viewHeight
	if len(app.dockerContainers) == 0 {
		return nil
	}
	if app.selectedDockerContainer < len(app.dockerContainers)-1 {
		app.selectedDockerContainer += step
		if app.selectedDockerContainer >= len(app.dockerContainers) {
			app.selectedDockerContainer = len(app.dockerContainers) - 1
		}
		if app.selectedDockerContainer >= app.startDockerContainers+app.maxVisibleDockerContainers {
			app.startDockerContainers += step
			if app.startDockerContainers > len(app.dockerContainers)-app.maxVisibleDockerContainers {
				app.startDockerContainers = len(app.dockerContainers) - app.maxVisibleDockerContainers
			}
			app.updateDockerContainerList()
		}
		if app.selectedDockerContainer < app.startDockerContainers+app.maxVisibleDockerContainers {
			return app.selectDockerByIndex(app.selectedDockerContainer - app.startDockerContainers)
		}
	}
	return nil
}

func (app *App) prevDockerContainer(v *gocui.View, step int) error {
	_, viewHeight := v.Size()
	app.maxVisibleDockerContainers = viewHeight
	if len(app.dockerContainers) == 0 {
		return nil
	}
	if app.selectedDockerContainer > 0 {
		app.selectedDockerContainer -= step
		if app.selectedDockerContainer < 0 {
			app.selectedDockerContainer = 0
		}
		if app.selectedDockerContainer < app.startDockerContainers {
			app.startDockerContainers -= step
			if app.startDockerContainers < 0 {
				app.startDockerContainers = 0
			}
			app.updateDockerContainerList()
		}
		if app.selectedDockerContainer >= app.startDockerContainers {
			return app.selectDockerByIndex(app.selectedDockerContainer - app.startDockerContainers)
		}
	}
	return nil
}

func (app *App) selectDockerByIndex(index int) error {
	v, err := app.gui.View("docker")
	if err != nil {
		return err
	}
	// Обновляем счетчик в заголовке
	re := regexp.MustCompile(`\s\(.+\) >`)
	updateTitle := " (0) >"
	if len(app.dockerContainers) != 0 {
		updateTitle = " (" + strconv.Itoa(app.selectedDockerContainer+1) + "/" + strconv.Itoa(len(app.dockerContainers)) + ") >"
	}
	v.Title = re.ReplaceAllString(v.Title, updateTitle)
	if err := v.SetCursor(0, index); err != nil {
		return nil
	}
	return nil
}

func (app *App) selectDocker(g *gocui.Gui, v *gocui.View) error {
	if v == nil || len(app.dockerContainers) == 0 {
		return nil
	}
	_, cy := v.Cursor()
	line, err := v.Line(cy)
	if err != nil {
		return err
	}
	if app.fastMode {
		go func() {
			app.loadDockerLogs(strings.TrimSpace(line), true)
		}()
	} else {
		app.loadDockerLogs(strings.TrimSpace(line), true)
	}
	app.lastWindow = "docker"
	app.lastSelected = strings.TrimSpace(line)
	return nil
}

func (app *App) loadDockerLogs(containerName string, newUpdate bool) {
	app.debugStartTime = time.Now()
	containerizationSystem := app.selectContainerizationSystem
	// Сохраняем систему контейнеризации для автообновления при смене окна
	if newUpdate {
		app.lastContainerizationSystem = app.selectContainerizationSystem
	} else {
		containerizationSystem = app.lastContainerizationSystem
	}
	var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	// Извлекаем id контейнера и namespace для подов k8s
	var containerId string
	var namespace string
	for _, dockerContainer := range app.dockerContainers {
		dockerContainerName := ansiEscape.ReplaceAllString(dockerContainer.name, "")
		if dockerContainerName == containerName {
			containerId = dockerContainer.id
			namespace = dockerContainer.namespace
		}
	}
	// Сохраняем id контейнера для автообновления при смене окна
	if newUpdate {
		app.lastContainerId = containerId
	} else {
		containerId = app.lastContainerId
	}
	// Читаем журналы Docker из файловой системы в формате JSON (если не отключено флагом и есть доступ)
	var readFileContainer bool
	if containerizationSystem == "docker" && !app.dockerStreamLogs {
		// Получаем путь к журналу контейнера в файловой системе по id с помощью метода docker cli
		var cmd *exec.Cmd
		if app.sshMode {
			cmd = exec.Command("ssh", append(app.sshOptions, "docker", "inspect", "--format", "{{.LogPath}}", containerId)...)
		} else {
			cmd = exec.Command("docker", "inspect", "--format", "{{.LogPath}}", containerId)
		}
		logFilePathBytes, err := cmd.Output()
		if err != nil && !app.testMode {
			v, _ := app.gui.View("logs")
			v.Clear()
			fmt.Fprintln(v, "\033[31mError get log path via docker inspect:", err, "\033[0m")
			return
		}
		if err != nil && app.testMode {
			log.Print("Error: get log path via docker inspect. ", err)
		}
		logFilePath := strings.TrimSpace(string(logFilePathBytes))
		// Читаем файл с конца с помощью tail
		if app.sshMode {
			cmd = exec.Command("ssh", append(app.sshOptions, "tail", "-n", app.logViewCount, logFilePath)...)
		} else {
			cmd = exec.Command("tail", "-n", app.logViewCount, logFilePath)
		}
		output, err := cmd.Output()
		// Если ошибка чтения, значит нет доступа и переходим к чтению из потока
		if err != nil && app.dockerStreamLogsStr == "json" {
			readFileContainer = false
			app.dockerStreamLogsStr = "stream"
			app.dockerStreamLogs = true
			if !app.testMode {
				go func() {
					text := "Access denied to json logs (use root)"
					app.showInterfaceInfo(g, true, text)
					time.Sleep(3 * time.Second)
					app.closeInfo(g)
				}()
			}
		} else {
			readFileContainer = true
			app.dockerStreamLogsStr = "json"
		}
		if readFileContainer {
			// Проверяем, что есть изменения в файле при повторном считывание
			if newUpdate {
				// Фиксируем новую дату изменения и размер для выбранного файла
				fileInfo, err := app.statFile(logFilePath)
				if err != nil {
					return
				}
				fileModTime := fileInfo.ModTime()
				fileSize := fileInfo.Size()
				app.lastDateUpdateFile = fileModTime
				app.lastSizeFile = fileSize
				app.updateFile = true
			} else {
				// Проверяем дату изменения
				fileInfo, err := app.statFile(logFilePath)
				if err != nil {
					return
				}
				fileModTime := fileInfo.ModTime()
				fileSize := fileInfo.Size()
				// Обновлять файл, только если есть изменения (проверяем дату модификации и размер)
				if fileModTime != app.lastDateUpdateFile || fileSize != app.lastSizeFile {
					app.lastDateUpdateFile = fileModTime
					app.lastSizeFile = fileSize
					app.updateFile = true
				} else {
					app.updateFile = false
				}
			}
			// Читаем файл, толькое если были изменения
			if app.updateFile {
				// Разбиваем строки на массив
				lines := strings.Split(strings.TrimSpace(string(output)), "\n")
				var formattedLines []string
				// Обрабатываем вывод в формате JSON построчно
				for _, line := range lines {
					// JSON-структура для парсинга
					var jsonData map[string]interface{}
					err := json.Unmarshal([]byte(line), &jsonData)
					if err != nil {
						continue
					}
					// Извлекаем JSON данные
					stream, _ := jsonData["stream"].(string)
					timeStr, _ := jsonData["time"].(string)
					logMessage, _ := jsonData["log"].(string)
					// Проверяем режим вывода потоков и пропускаем лишние строки
					// Если текущий режим соответствует стандартному выводу и текущая строка содержит поток ошибки (или наоборот), пропускаем интерацию
					if app.dockerStreamMode == "stdout" && stream == "stderr" {
						continue
					}
					if app.dockerStreamMode == "stderr" && stream == "stdout" {
						continue
					}
					// Удаляем встроенный экранированный символ переноса строки
					logMessage = strings.TrimSuffix(logMessage, "\n")
					// Парсим строку времени в объект time.Time
					parsedTime, err := time.Parse(time.RFC3339Nano, timeStr)
					if err == nil {
						// Форматируем дату в формате: YYYY-MM-DDTHH:MM:SS.MS(x9)Z
						timeStr = parsedTime.Format("2006-01-02T15:04:05.000000000Z")
					}
					var formattedLine string
					// Заполняем строку в формате
					switch {
					case app.timestampDocker && app.streamTypeDocker:
						// stream time log
						formattedLine = fmt.Sprintf("%s %s %s", stream, timeStr, logMessage)
					case app.timestampDocker && !app.streamTypeDocker:
						// time log
						formattedLine = fmt.Sprintf("%s %s", timeStr, logMessage)
					case !app.timestampDocker && app.streamTypeDocker:
						// stream log
						formattedLine = fmt.Sprintf("%s %s", stream, logMessage)
					case !app.timestampDocker && !app.streamTypeDocker:
						// log only
						formattedLine = logMessage
					}
					formattedLines = append(formattedLines, formattedLine)
					// Если это последняя строка в выводе, добавляем перенос строки
				}
				app.currentLogLines = formattedLines
			}
		}
	}
	// Читаем лог через docker cli (если файл не найден или к нему нет доступа) или это compose/podman/kubectl
	if !readFileContainer || containerizationSystem != "docker" {
		// Извлекаем имя без статуса в containerId для k8s и docker compose
		if containerizationSystem == "kubectl" || containerizationSystem == "compose" {
			parts := strings.Split(containerName, "] ")
			containerId = parts[1]
		}
		var cmd *exec.Cmd
		switch containerizationSystem {
		case "kubectl":
			// Формируем команду kubectl с нужными ключами
			if app.sshMode {
				cmd = exec.Command("ssh", append(app.sshOptions,
					containerizationSystem, "logs", "-n", namespace, "--timestamps=true", "--tail", app.logViewCount, containerId,
				)...)
			} else {
				cmd = exec.Command(
					containerizationSystem, "logs", "-n", namespace, "--timestamps=true", "--tail", app.logViewCount, containerId,
				)
			}
		case "compose":
			sinceFilterTextNotSpace := reSpace.ReplaceAllString(app.sinceFilterText, "T")
			untilFilterTextNotSpace := reSpace.ReplaceAllString(app.untilFilterText, "T")
			if app.sshMode {
				switch {
				case app.sinceTimestampFilterMode && app.untilTimestampFilterMode:
					cmd = exec.Command(
						"ssh", append(app.sshOptions,
							"docker", "compose", "-p", containerId, "logs", "--timestamps", "--no-color", // "--no-log-prefix",
							"--since", sinceFilterTextNotSpace,
							"--until", untilFilterTextNotSpace,
							"--tail", app.logViewCount,
						)...,
					)
				case app.sinceTimestampFilterMode && !app.untilTimestampFilterMode:
					cmd = exec.Command(
						"ssh", append(app.sshOptions,
							"docker", "compose", "-p", containerId, "logs", "--timestamps", "--no-color", // "--no-log-prefix",
							"--since", sinceFilterTextNotSpace,
							"--tail", app.logViewCount,
						)...,
					)
				case !app.sinceTimestampFilterMode && app.untilTimestampFilterMode:
					cmd = exec.Command(
						"ssh", append(app.sshOptions,
							"docker", "compose", "-p", containerId, "logs", "--timestamps", "--no-color", // "--no-log-prefix",
							"--until", untilFilterTextNotSpace,
							"--tail", app.logViewCount,
						)...,
					)
				default:
					cmd = exec.Command(
						"ssh", append(app.sshOptions,
							"docker", "compose", "-p", containerId, "logs", "--timestamps", "--no-color", // "--no-log-prefix",
							"--tail", app.logViewCount,
						)...,
					)
				}
			} else {
				switch {
				case app.sinceTimestampFilterMode && app.untilTimestampFilterMode:
					cmd = exec.Command(
						"docker", "compose", "-p", containerId, "logs", "--timestamps", "--no-color", // "--no-log-prefix",
						"--since", sinceFilterTextNotSpace,
						"--until", untilFilterTextNotSpace,
						"--tail", app.logViewCount,
					)
				case app.sinceTimestampFilterMode && !app.untilTimestampFilterMode:
					cmd = exec.Command(
						"docker", "compose", "-p", containerId, "logs", "--timestamps", "--no-color", // "--no-log-prefix",
						"--since", sinceFilterTextNotSpace,
						"--tail", app.logViewCount,
					)
				case !app.sinceTimestampFilterMode && app.untilTimestampFilterMode:
					cmd = exec.Command(
						"docker", "compose", "-p", containerId, "logs", "--timestamps", "--no-color", // "--no-log-prefix",
						"--until", untilFilterTextNotSpace,
						"--tail", app.logViewCount,
					)
				default:
					cmd = exec.Command(
						"docker", "compose", "-p", containerId, "logs", "--timestamps", "--no-color", // "--no-log-prefix",
						"--tail", app.logViewCount,
					)
				}
			}
		default:
			// docker/podman cli
			sinceFilterTextNotSpace := reSpace.ReplaceAllString(app.sinceFilterText, "T")
			untilFilterTextNotSpace := reSpace.ReplaceAllString(app.untilFilterText, "T")
			if app.sshMode {
				switch {
				case app.sinceTimestampFilterMode && app.untilTimestampFilterMode:
					cmd = exec.Command(
						"ssh", append(app.sshOptions,
							containerizationSystem, "logs", "--timestamps",
							"--since", sinceFilterTextNotSpace,
							"--until", untilFilterTextNotSpace,
							"--tail", app.logViewCount,
							containerId,
						)...,
					)
				case app.sinceTimestampFilterMode && !app.untilTimestampFilterMode:
					cmd = exec.Command(
						"ssh", append(app.sshOptions,
							containerizationSystem, "logs", "--timestamps",
							"--since", sinceFilterTextNotSpace,
							"--tail", app.logViewCount,
							containerId,
						)...,
					)
				case !app.sinceTimestampFilterMode && app.untilTimestampFilterMode:
					cmd = exec.Command(
						"ssh", append(app.sshOptions,
							containerizationSystem, "logs", "--timestamps",
							"--until", untilFilterTextNotSpace,
							"--tail", app.logViewCount,
							containerId,
						)...,
					)
				default:
					cmd = exec.Command(
						"ssh", append(app.sshOptions,
							containerizationSystem, "logs", "--timestamps",
							"--tail", app.logViewCount,
							containerId,
						)...,
					)
				}
			} else {
				switch {
				case app.sinceTimestampFilterMode && app.untilTimestampFilterMode:
					cmd = exec.Command(
						containerizationSystem, "logs", "--timestamps",
						"--since", sinceFilterTextNotSpace,
						"--until", untilFilterTextNotSpace,
						"--tail", app.logViewCount,
						containerId,
					)
				case app.sinceTimestampFilterMode && !app.untilTimestampFilterMode:
					cmd = exec.Command(
						containerizationSystem, "logs", "--timestamps",
						"--since", sinceFilterTextNotSpace,
						"--tail", app.logViewCount,
						containerId,
					)
				case !app.sinceTimestampFilterMode && app.untilTimestampFilterMode:
					cmd = exec.Command(
						containerizationSystem, "logs", "--timestamps",
						"--until", untilFilterTextNotSpace,
						"--tail", app.logViewCount,
						containerId,
					)
				default:
					cmd = exec.Command(
						containerizationSystem, "logs", "--timestamps",
						"--tail", app.logViewCount,
						containerId,
					)
				}
			}
		}
		// Храним байты вывода
		var stdoutBytes, stderrBytes []byte
		var stdoutErr, stderrErr error
		// Храним комбинированный вывод двух потоков
		var combined []dockerLogLines
		switch {
		// Читаем только один поток в режиме stdout или compose
		case app.dockerStreamMode == "stdout" || containerizationSystem == "compose":
			// Читаем стандартный вывод
			stdoutPipe, _ := cmd.StdoutPipe()
			_ = cmd.Start()
			stdoutBytes, stdoutErr = io.ReadAll(stdoutPipe)
			stdoutLines := strings.Split(string(stdoutBytes), "\n")
			// Формируем итоговый массив
			for _, line := range stdoutLines {
				// Пропускаем пустые строки
				if strings.TrimSpace(line) == "" {
					continue
				}
				var ts time.Time
				var err error
				// Извлекаем время из compose
				if containerizationSystem == "compose" {
					// Сначала извлекаем имя сервиса
					parts1 := strings.SplitN(line, " | ", 2)
					// Затем извлекаем timestamp
					parts2 := strings.SplitN(parts1[1], " ", 2)
					tsStr := strings.TrimSpace(parts2[0])
					ts, err = time.Parse(time.RFC3339Nano, tsStr)
				} else {
					// Извлекаем время из префикса docker/podman
					ts, err = parseTimestamp(line)
				}
				if err != nil {
					continue
				}
				combined = append(combined, dockerLogLines{
					isError:   false,
					timestamp: ts,
					content:   line,
				})
			}
			// Сортируем вывод по timestamp для compose
			if containerizationSystem == "compose" {
				sort.Slice(
					combined,
					func(i, j int) bool {
						return combined[i].timestamp.Before(combined[j].timestamp)
					},
				)
			}
		case app.dockerStreamMode == "stderr":
			// Читаем вывод ошибок
			stderrPipe, _ := cmd.StderrPipe()
			_ = cmd.Start()
			stderrBytes, stderrErr = io.ReadAll(stderrPipe)
			stderrLines := strings.Split(string(stderrBytes), "\n")
			// Формируем итоговый массив
			for _, line := range stderrLines {
				if strings.TrimSpace(line) == "" {
					continue
				}
				ts, err := parseTimestamp(line)
				if err != nil {
					continue
				}
				combined = append(combined, dockerLogLines{
					isError:   true,
					timestamp: ts,
					content:   line,
				})
			}
		default:
			// Читаем стандартный вывод
			stdoutPipe, _ := cmd.StdoutPipe()
			// Читаем вывод ошибок
			stderrPipe, _ := cmd.StderrPipe()
			// Запускаем команду
			_ = cmd.Start()
			// Читаем два потока параллельно, чтобы не блокировать
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				stdoutBytes, stdoutErr = io.ReadAll(stdoutPipe)
			}()
			go func() {
				defer wg.Done()
				stderrBytes, stderrErr = io.ReadAll(stderrPipe)
			}()
			wg.Wait()
			_ = cmd.Wait()
			// Обработка ошибок чтения
			if stdoutErr != nil || stderrErr != nil {
				if !app.testMode {
					v, _ := app.gui.View("logs")
					v.Clear()
					fmt.Fprintln(v, "\033[31mError getting logs from", containerName, "(id:", containerId, ")", "container.\033[0m")
					return
				} else {
					log.Print("Error: getting logs from ", containerName, " (id:", containerId, ")", " container.")
				}
			}
			// Получаем 2 массива вывода
			stdoutLines := strings.Split(string(stdoutBytes), "\n")
			stderrLines := strings.Split(string(stderrBytes), "\n")
			// Объединяем два массив
			for _, line := range stdoutLines {
				if strings.TrimSpace(line) == "" {
					continue
				}
				ts, err := parseTimestamp(line)
				if err != nil {
					continue
				}
				combined = append(combined, dockerLogLines{
					isError:   false,
					timestamp: ts,
					content:   line,
				})
			}
			for _, line := range stderrLines {
				if strings.TrimSpace(line) == "" {
					continue
				}
				ts, err := parseTimestamp(line)
				if err != nil {
					continue
				}
				combined = append(combined, dockerLogLines{
					isError:   true,
					timestamp: ts,
					content:   line,
				})
			}
			// Cортируем итоговый массив по timestamp
			sort.Slice(
				combined,
				func(i, j int) bool {
					return combined[i].timestamp.Before(combined[j].timestamp)
				},
			)
		}
		// Добавляем префиксы с типом данных (stdout или stderr) в зависимости от режима флагов
		var finalLines []string
		for _, entry := range combined {
			entryLine := entry.content
			// Удаляем из строки timestamp
			if !app.timestampDocker {
				entryLine = removeTimestamp(entry.content)
			}
			// Не добавляем профексы в отключенном режиме и для compose
			if !app.streamTypeDocker || containerizationSystem == "compose" {
				finalLines = append(finalLines, entryLine)
			} else {
				prefix := "stdout "
				if entry.isError {
					prefix = "stderr "
				}
				finalLine := prefix + entryLine
				finalLines = append(finalLines, finalLine)
			}
		}
		app.currentLogLines = finalLines
	}
	// Обновляем фильтр и делиметр всегда для потоков ИЛИ если есть изменения в файле при его чтение
	if !readFileContainer || (readFileContainer && app.updateFile) || containerizationSystem != "docker" {
		app.updateDelimiter(newUpdate)
		app.applyFilter(false)
	}
}

// Функция извлечения parseTimestamp для сортировки
func parseTimestamp(line string) (time.Time, error) {
	// Делим строку на две части по первому пробелу
	parts := strings.SplitN(line, " ", 2)
	// Удаляем лишние пробелы
	tsStr := strings.TrimSpace(parts[0])
	// Парсим строку (извлекаем временную метку)
	return time.Parse(time.RFC3339Nano, tsStr)
}

// Функция для удаления timestamp из строки (первого слова до первого пробела)
func removeTimestamp(line string) string {
	// Находим индекс первого пробела
	spaceIndex := strings.Index(line, " ")
	if spaceIndex == -1 {
		// Если пробела нет, возвращаем строку как есть
		return line
	}
	// Возвращаем строку начиная с символа после первого пробела
	return line[spaceIndex+1:]
}

// ---------------------------------------- Filter ----------------------------------------

// Редактор обработки ввода текста для фильтрации
func (app *App) createFilterEditor(window string) gocui.Editor {
	return gocui.EditorFunc(func(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
		switch {
		// Добавляем символ в поле ввода
		case ch != 0 && mod == 0:
			v.EditWrite(ch)
		// Добавляем пробел
		case key == gocui.KeySpace:
			v.EditWrite(' ')
		// Удаляем символ слева от курсора
		case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
			v.EditDelete(true)
		// Удаляем символ справа от курсора
		case key == gocui.KeyDelete:
			v.EditDelete(false)
		// Перемещение курсора влево
		case key == gocui.KeyArrowLeft:
			v.MoveCursor(-1, 0) // удалить 3-й булевой параметр для форка
		// Перемещение курсора вправо
		case key == gocui.KeyArrowRight:
			v.MoveCursor(1, 0)
		}
		switch window {
		case "logs":
			// Обновляем текст в буфере
			app.filterText = strings.TrimSpace(v.Buffer())
			// Применяем функцию фильтрации к выводу записей журнала
			app.applyFilter(true)
		case "lists":
			app.filterListText = strings.TrimSpace(v.Buffer())
			app.applyFilterList()
		}
	})
}

// Функция для обработки фильтрации по временной метке
func (app *App) timestampFilterEditor(window string) gocui.Editor {
	return gocui.EditorFunc(func(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
		switch {
		// Пропускаем только цифры (0-9)
		case ch >= '0' && ch <= '9':
			v.EditWrite(ch)
		// Пропускаем ":" для времени, "-" для даты, а также [+-] и [smh] для сокращенного формата
		case ch == ':' || ch == '-' || ch == '+' || ch == 's' || ch == 'm' || ch == 'h' || ch == 'd':
			v.EditWrite(ch)
		// Пропускаем пробел (работает в journalctl для разделения времени, но необходимо обновить в docker logs на T)
		case key == gocui.KeySpace:
			v.EditWrite(' ')
		case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
			v.EditDelete(true)
		case key == gocui.KeyDelete:
			v.EditDelete(false)
		case key == gocui.KeyArrowLeft:
			v.MoveCursor(-1, 0)
		case key == gocui.KeyArrowRight:
			v.MoveCursor(1, 0)
		}
		switch window {
		case "sinceFilter":
			// Обновляем текст в буфере
			app.sinceFilterText = strings.TrimSpace(v.Buffer())
			// Если фильтр пустой, отключаем фильтрацию
			switch {
			case strings.TrimSpace(v.Buffer()) == "":
				v.FrameColor = gocui.ColorGreen
				app.sinceTimestampFilterMode = false
				// Проверяем формат и активируем фильтрацию
			case app.timestampCheckFormat(strings.TrimSpace(v.Buffer())):
				v.FrameColor = gocui.ColorGreen
				app.sinceTimestampFilterMode = true
			default:
				v.FrameColor = gocui.ColorRed
				app.sinceTimestampFilterMode = false
			}
		case "untilFilter":
			app.untilFilterText = strings.TrimSpace(v.Buffer())
			switch {
			case strings.TrimSpace(v.Buffer()) == "":
				v.FrameColor = gocui.ColorGreen
				app.untilTimestampFilterMode = false
				// Проверяем формат и активируем фильтрацию
			case app.timestampCheckFormat(strings.TrimSpace(v.Buffer())):
				v.FrameColor = gocui.ColorGreen
				app.untilTimestampFilterMode = true
			default:
				v.FrameColor = gocui.ColorRed
				app.untilTimestampFilterMode = false
			}
		}
	})
}

// Функция проверки формата времени для фильтрации
func (app *App) timestampCheckFormat(input string) bool {
	formats := []string{
		"15:04",               // 00:00
		"15:04:05",            // 00:00:00
		"2006-01-02",          // 2025-04-14
		"2006-01-02 15:04",    // 2025-04-14 00:00
		"2006-01-02 15:04:05", // 2025-04-14 00:00:00
	}
	if filterTimeRegex.MatchString(input) {
		return true
	} else {
		for _, layout := range formats {
			if _, err := time.Parse(layout, input); err == nil {
				return true
			}
		}
	}
	return false
}

// Функция для фильтрации всех списоков журналов
func (app *App) applyFilterList() {
	filter := strings.ToLower(app.filterListText)
	// Временные массивы для отфильтрованных журналов
	var filteredJournals []Journal
	var filteredLogFiles []Logfile
	var filteredDockerContainers []DockerContainers
	for _, j := range app.journalsNotFilter {
		if strings.Contains(strings.ToLower(j.name), filter) {
			filteredJournals = append(filteredJournals, j)
		}
	}
	for _, j := range app.logfilesNotFilter {
		if strings.Contains(strings.ToLower(j.name), filter) {
			filteredLogFiles = append(filteredLogFiles, j)
		}
	}
	for _, j := range app.dockerContainersNotFilter {
		if strings.Contains(strings.ToLower(j.name), filter) {
			filteredDockerContainers = append(filteredDockerContainers, j)
		}
	}
	// Сбрасываем индексы выбранного журнала для правильного позиционирования
	app.selectedJournal = 0
	app.selectedFile = 0
	app.selectedDockerContainer = 0
	app.startServices = 0
	app.startFiles = 0
	app.startDockerContainers = 0
	// Сохраняем отфильтрованные и отсортированные данные
	app.journals = filteredJournals
	app.logfiles = filteredLogFiles
	app.dockerContainers = filteredDockerContainers
	// Обновляем статус количества служб
	if !app.testMode {
		// Обновляем списки в интерфейсе
		app.updateServicesList()
		app.updateLogsList()
		app.updateDockerContainerList()
		v, _ := app.gui.View("services")
		// Обновляем счетчик в заголовке
		re := regexp.MustCompile(`\s\(.+\) >`)
		updateTitle := " (0) >"
		if len(app.journals) != 0 {
			updateTitle = " (" + strconv.Itoa(app.selectedJournal+1) + "/" + strconv.Itoa(len(app.journals)) + ") >"
		}
		v.Title = re.ReplaceAllString(v.Title, updateTitle)
		// Обновляем статус количества файлов
		v, _ = app.gui.View("varLogs")
		// Обновляем счетчик в заголовке
		re = regexp.MustCompile(`\s\(.+\) >`)
		updateTitle = " (0) >"
		if len(app.logfiles) != 0 {
			updateTitle = " (" + strconv.Itoa(app.selectedFile+1) + "/" + strconv.Itoa(len(app.logfiles)) + ") >"
		}
		v.Title = re.ReplaceAllString(v.Title, updateTitle)
		// Обновляем статус количества контейнеров
		v, _ = app.gui.View("docker")
		// Обновляем счетчик в заголовке
		re = regexp.MustCompile(`\s\(.+\) >`)
		updateTitle = " (0) >"
		if len(app.dockerContainers) != 0 {
			updateTitle = " (" + strconv.Itoa(app.selectedDockerContainer+1) + "/" + strconv.Itoa(len(app.dockerContainers)) + ") >"
		}
		v.Title = re.ReplaceAllString(v.Title, updateTitle)
	}
}

// Функция для фильтрации записей текущего журнала + покраска
func (app *App) applyFilter(color bool) {
	filter := app.filterText
	var skip bool = false
	var size int
	var viewHeight int
	var err error
	if !app.testMode {
		v, err := app.gui.View("filter")
		if err != nil {
			return
		}
		if color {
			v.FrameColor = gocui.ColorGreen
		}
		// Если текст фильтра не менялся и позиция курсора не в самом конце журнала, то пропускаем фильтрацию и покраску при пролистывании
		vLogs, _ := app.gui.View("logs")
		_, viewHeight := vLogs.Size()
		size = app.logScrollPos + viewHeight + 1
		if app.lastFilterText == filter && size < len(app.filteredLogLines) {
			skip = true
		}
		// Фиксируем текущий текст из фильтра
		app.lastFilterText = filter
	}
	// Фильтруем и красим, только если это не скроллинг
	if !skip {
		// Debug end load time
		endLoadTime := time.Since(app.debugStartTime)
		// Фиксируем время окончания загрузки журнала
		app.debugLoadTime = endLoadTime.Truncate(time.Millisecond).String()
		// Debug start color time
		// Фиксируем время начала покраски журнала
		startTime := time.Now()
		// Debug: если текст фильтра пустой или равен любому символу для regex, возвращяем вывод без фильтрации
		if filter == "" || (filter == "." && app.selectFilterMode == "regex") {
			app.filteredLogLines = app.currentLogLines
		} else {
			app.filteredLogLines = make([]string, 0)
			// Опускаем регистр ввода текста для фильтра
			filter = strings.ToLower(filter)
			// Проверка регулярного выражения
			var regex *regexp.Regexp
			if app.selectFilterMode == "regex" {
				// Добавляем флаг для нечувствительности к регистру по умолчанию
				filter = "(?i)" + filter
				// Компилируем регулярное выражение
				regex, err = regexp.Compile(filter)
				// В случае синтаксической ошибки регулярного выражения, красим окно красным цветом и завершаем цикл
				if err != nil && !app.testMode {
					v, _ := app.gui.View("filter")
					v.FrameColor = gocui.ColorRed
					return
				}
				if err != nil && !app.testMode {
					log.Print("Error: regex syntax")
					return
				}
			}
			// Проходимся по каждой строке
			for _, line := range app.currentLogLines {
				switch app.selectFilterMode {
				// Fuzzy (неточный поиск без учета регистра)
				case "fuzzy":
					outputLine := app.fuzzyFilter(line, filter)
					if outputLine != "" {
						app.filteredLogLines = append(app.filteredLogLines, outputLine)
					}
				// Regex (с использованием регулярных выражений и без учета регистра по умолчанию)
				case "regex":
					outputLine := app.regexFilter(line, regex)
					if outputLine != "" {
						app.filteredLogLines = append(app.filteredLogLines, outputLine)
					}
				// Default (точный поиск с учетом регистра)
				default:
					filter = app.filterText
					if filter == "" || strings.Contains(line, filter) {
						lineColor := strings.ReplaceAll(line, filter, "\x1b[0;44m"+filter+"\033[0m")
						app.filteredLogLines = append(app.filteredLogLines, lineColor)
					}
				}
			}
		}
		// Если последняя строка не содержит пустую строку, то добавляем две пустые строки или одну по умолчанию
		if len(app.filteredLogLines) > 0 && app.filteredLogLines[len(app.filteredLogLines)-1] != "" {
			app.filteredLogLines = append(app.filteredLogLines, "", "")
		} else {
			app.filteredLogLines = append(app.filteredLogLines, "")
		}
		// Отключаем покраску в режиме colorMode
		if app.colorMode {
			// Режим покраски через tailspin
			if app.tailSpinMode {
				cmd := exec.Command(app.tailSpinBinName)
				logLines := strings.Join(app.filteredLogLines, "\n")
				// Создаем пайп для передачи данных
				cmd.Stdin = bytes.NewBufferString(logLines)
				var out bytes.Buffer
				cmd.Stdout = &out
				if err := cmd.Run(); err != nil {
					fmt.Println(err)
				}
				colorLogLines := strings.Split(out.String(), "\n")
				app.filteredLogLines = colorLogLines
			} else {
				app.filteredLogLines = app.mainColor(app.filteredLogLines)
			}
		}
		// Debug end time
		endTime := time.Since(startTime)
		app.debugColorTime = endTime.Truncate(time.Millisecond).String()
	}
	// Debug: корректируем текущую позицию скролла, если размер массива стал меньше
	if size > len(app.filteredLogLines) {
		newScrollPos := len(app.filteredLogLines) - viewHeight
		if newScrollPos > 0 {
			app.logScrollPos = newScrollPos
		} else {
			app.logScrollPos = 0
		}
	}
	// Обновляем автоскролл (всегда опускаем вывод в самый низ) для отображения отфильтрованных записей
	if !app.testMode {
		// Включаем автоскролл и сбрасываем позицию
		if !app.disableAutoScroll {
			app.autoScroll = true
		} else {
			app.autoScroll = false
		}
		vLog, _ := app.gui.View("logs")
		vLog.Subtitle = fmt.Sprintf("[tail: %s lines | auto-update: %t (%d sec) | docker: %s (%s) | color: %t]", app.logViewCount, app.autoScroll, app.logUpdateSeconds, app.dockerStreamLogsStr, app.dockerStreamMode, app.colorMode)
		app.logScrollPos = 0
		app.updateLogsView(true)
	}
}

// Fyzzy: Функция для неточного поиска (параметры: строка из цикла и текст фильтрации)
func (app *App) fuzzyFilter(inputLine, filter string) string {
	// Разбиваем текст фильтра на массив из строк
	filterWords := strings.Fields(filter)
	// Опускаем регистр текущей строки цикла
	lineLower := strings.ToLower(inputLine)
	var match bool = true
	// Проверяем, если строка не содержит хотя бы одно слово из фильтра, то пропускаем строку
	for _, word := range filterWords {
		if !strings.Contains(lineLower, word) {
			match = false
			break
		}
	}
	// Если строка подходит под фильтр, возвращаем ее с покраской
	if match {
		// Временные символы для обозначения начала и конца покраски найденных символов
		startColor := "►"
		endColor := "◄"
		originalLine := inputLine
		// Проходимся по всем словосочетаниям фильтра (массив через пробел) для позиционирования покраски
		for _, word := range filterWords {
			wordLower := strings.ToLower(word)
			start := 0
			// Ищем все вхождения слова в строке с учетом регистра
			for {
				// Находим индекс вхождения с учетом регистра
				idx := strings.Index(strings.ToLower(originalLine[start:]), wordLower)
				if idx == -1 {
					break // Если больше нет вхождений, выходим
				}
				start += idx // корректируем индекс с учетом текущей позиции
				// Вставляем временные символы для покраски
				originalLine = originalLine[:start] + startColor + originalLine[start:start+len(word)] + endColor + originalLine[start+len(word):]
				// Сдвигаем индекс для поиска в оставшейся части строки
				start += len(startColor) + len(word) + len(endColor)
			}
		}
		// Заменяем временные символы на ANSI escape-последовательности
		originalLine = strings.ReplaceAll(originalLine, startColor, "\x1b[0;44m")
		originalLine = strings.ReplaceAll(originalLine, endColor, "\033[0m")
		return originalLine
	} else {
		return ""
	}
}

// Regex: Функция для поска с использованием регулярных выражений (параметры: строка из цикла и скомпилированное регулярное выражение)
func (app *App) regexFilter(inputLine string, regex *regexp.Regexp) string {
	// Проверяем, что строка подходит под регулярное выражение
	if regex.MatchString(inputLine) {
		// Находим все найденные совпадени
		matches := regex.FindAllString(inputLine, -1)
		// Красим только первое найденное совпадение
		inputLine = strings.ReplaceAll(inputLine, matches[0], "\x1b[0;44m"+matches[0]+"\033[0m")
		return inputLine
	} else {
		return ""
	}
}

// -f/--command-fuzzy
func (app *App) commandLineFuzzy(filter string) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		fmt.Fprintln(os.Stderr, "No data. Use pipe to transfer data.")
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	var inputLines []string
	for scanner.Scan() {
		inputLines = append(inputLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	if len(inputLines) == 0 {
		fmt.Fprintln(os.Stderr)
		return
	}
	for _, line := range inputLines {
		outputLine := app.fuzzyFilter(line, filter)
		if outputLine != "" {
			app.filteredLogLines = append(app.filteredLogLines, outputLine)
		}
	}
	for _, line := range app.filteredLogLines {
		fmt.Println(line)
	}
}

// --command-regex/-r
func (app *App) commandLineRegex(regex *regexp.Regexp) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		fmt.Fprintln(os.Stderr, "No data. Use pipe to transfer data.")
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	var inputLines []string
	for scanner.Scan() {
		inputLines = append(inputLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	if len(inputLines) == 0 {
		fmt.Fprintln(os.Stderr)
		return
	}
	for _, line := range inputLines {
		outputLine := app.regexFilter(line, regex)
		if outputLine != "" {
			app.filteredLogLines = append(app.filteredLogLines, outputLine)
		}
	}
	for _, line := range app.filteredLogLines {
		fmt.Println(line)
	}
}

// ---------------------------------------- Coloring ----------------------------------------

// Функция для покраски вывода в режиме командной строки
func (app *App) commandLineColor() {
	// Проверяем, подключен ли stdin через pipe или перенаправлен
	stat, err := os.Stdin.Stat()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	// Проверяем, пуст ли stdin (например, если нет pipe или перенаправления)
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		fmt.Fprintln(os.Stderr, "No data. Use pipe to transfer data.")
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	var inputLines []string
	for scanner.Scan() {
		inputLines = append(inputLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	if len(inputLines) == 0 {
		fmt.Fprintln(os.Stderr)
		return
	}
	inputColoring := app.mainColor(inputLines)
	for _, line := range inputColoring {
		fmt.Println(line)
	}
}

// Основная функция покраски
func (app *App) mainColor(inputText []string) []string {
	// Максимальное количество потоков
	const maxWorkers = 10
	// Канал для передачи индексов всех строк
	tasks := make(chan int, len(inputText))
	// Срез для хранения обработанных строк
	colorLogLines := make([]string, len(inputText))
	// Объявляем группу ожидания для синхронизации всех горутин (воркеров)
	var wg sync.WaitGroup
	// Создаем maxWorkers горутин, где каждая будет обрабатывать задачи из канала tasks
	for i := 0; i < maxWorkers; i++ {
		go func() {
			// Горутина будет работать, пока в канале tasks есть задачи
			for index := range tasks {
				// Обрабатываем строку и сохраняем результат по соответствующему индексу
				colorLogLines[index] = app.lineColor(inputText[index])
				// Уменьшаем счетчик задач в группе ожидания.
				wg.Done()
			}
		}()
	}
	// Добавляем задачи в канал
	for i := range inputText {
		// Увеличиваем счетчик задач в группе ожидания
		wg.Add(1)
		// Передаем индекс строки в канал tasks
		tasks <- i
	}
	// Закрываем канал задач, чтобы воркеры завершили работу после обработки всех задач
	close(tasks)
	// Ждем завершения всех задач
	wg.Wait()
	return colorLogLines
}

func (app *App) lineColor(inputLine string) string {
	// Если строка пустая, пропускаем ее сразу
	if inputLine == "" {
		return ""
	}
	var colorLine string
	var filterColor bool = false
	// Извлекаем название контейнера в логах стека compose
	var containerName string
	if app.lastContainerizationSystem == "compose" {
		// Исключаем строку с делиметром
		if !strings.HasPrefix(inputLine, "⎯") {
			splitLine := strings.SplitN(inputLine, " | ", 2)
			if splitLine[0] != "" && splitLine[1] != "" {
				containerName = splitLine[0]
				// Удаляем название контейнера из покраски
				inputLine = splitLine[1]
			}
		}
	}
	// Разбиваем строку по пробелам, сохраняя их
	words := strings.Split(inputLine, " ")
	for i, word := range words {
		// Исключаем строки с покраской при поиске (Background)
		if strings.Contains(word, "\x1b[0;44m") {
			filterColor = true
		}
		// Красим слово в функции
		if !filterColor {
			word = app.wordColor(word)
		}
		// Возобновляем покраску
		if strings.Contains(word, "\033[0m") {
			filterColor = false
		}
		// Добавляем слово обратно с пробелами
		if i != len(words)-1 {
			colorLine += word + " "
		} else {
			colorLine += word
		}
	}
	if app.selectContainerizationSystem == "compose" && containerName != "" {
		// Возвращяем название контейнера с уникальной покраской
		if app.uniquePrefixColorMap[strings.TrimSpace(containerName)] != "" {
			return app.uniquePrefixColorMap[strings.TrimSpace(containerName)] + containerName + " |\033[0m " + colorLine
		} else {
			return containerName + " | " + colorLine
		}
	} else {
		return colorLine
	}
}

// Игнорируем регистр и проверяем, что слово окружено границами (не буквы и цифры)
func (app *App) replaceWordLower(word, keyword, color string) string {
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(keyword) + `\b`)
	return re.ReplaceAllStringFunc(word, func(match string) string {
		return color + match + "\033[0m"
	})
}

// Поиск пользователей
func (app *App) containsUser(searchWord string) bool {
	for _, user := range app.userNameArray {
		if user == searchWord {
			return true
		}
	}
	return false
}

// Поиск корневых директорий
func (app *App) containsPath(searchWord string) bool {
	for _, dir := range app.rootDirArray {
		if strings.Contains(searchWord, dir) {
			return true
		}
	}
	return false
}

// Покраска url путей
func (app *App) urlPathColor(cleanedWord string) string {
	// Используем Builder для объединения строк
	var sb strings.Builder
	// Начинаем с желтого цвета
	sb.WriteString("\033[33m")
	for _, char := range cleanedWord {
		switch {
		// Пурпурный цвет для символов и возвращяем желтый
		case char == '/' || char == '?' || char == '&' || char == '=' || char == ':' || char == '.':
			sb.WriteString("\033[35m")
			sb.WriteRune(char)
			sb.WriteString("\033[33m")
		// Синий цвет для цифр
		// case unicode.IsDigit(char):
		case char >= '0' && char <= '9':
			sb.WriteString("\033[34m")
			sb.WriteRune(char)
			sb.WriteString("\033[33m")
		default:
			sb.WriteRune(char)
		}
	}
	// Сброс цвета
	sb.WriteString("\033[0m")
	return sb.String()
}

// Функция для покраски словосочетаний
func (app *App) wordColor(inputWord string) string {
	// Опускаем регистр слова
	inputWordLower := strings.ToLower(inputWord)
	// Значение по умолчанию
	var coloredWord string = inputWord
	switch {
	// URL
	case strings.Contains(inputWord, "http://"):
		cleanedWord := app.trimHttpRegex.ReplaceAllString(inputWord, "")
		coloredChars := app.urlPathColor(cleanedWord)
		// Красный для http
		coloredWord = strings.ReplaceAll(inputWord, "http://"+cleanedWord, "\033[31mhttp\033[35m://"+coloredChars)
	case strings.Contains(inputWord, "https://"):
		cleanedWord := app.trimHttpsRegex.ReplaceAllString(inputWord, "")
		coloredChars := app.urlPathColor(cleanedWord)
		// Зеленый для https
		coloredWord = strings.ReplaceAll(inputWord, "https://"+cleanedWord, "\033[32mhttps\033[35m://"+coloredChars)
	// UNIX file paths
	case app.containsPath(inputWord):
		cleanedWord := app.trimPrefixPathRegex.ReplaceAllString(inputWord, "")
		cleanedWord = app.trimPostfixPathRegex.ReplaceAllString(cleanedWord, "")
		// Начинаем с желтого цвета
		coloredChars := "\033[33m"
		for _, char := range cleanedWord {
			// Красим символы разделителя путей в пурпурный и возвращяем цвет
			if char == '/' {
				coloredChars += "\033[35m" + string(char) + "\033[33m"
			} else {
				coloredChars += string(char)
			}
		}
		coloredWord = strings.ReplaceAll(inputWord, cleanedWord, "\033[35m"+coloredChars+"\033[0m")
	// Желтый (известные имена: hostname и username) [33m]
	case strings.Contains(inputWord, app.hostName):
		coloredWord = strings.ReplaceAll(inputWord, app.hostName, "\033[33m"+app.hostName+"\033[0m")
	case strings.Contains(inputWord, app.userName):
		coloredWord = strings.ReplaceAll(inputWord, app.userName, "\033[33m"+app.userName+"\033[0m")
	// Список пользователей из passwd
	case app.containsUser(inputWord):
		coloredWord = app.replaceWordLower(inputWord, inputWord, "\033[33m")
	case strings.Contains(inputWordLower, "warn"):
		words := []string{"warnings", "warning", "warn"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[33m")
				break
			}
		}
	// UNIX processes
	case app.syslogUnitRegex.MatchString(inputWord):
		unitSplit := strings.Split(inputWord, "[")
		unitName := unitSplit[0]
		unitId := strings.ReplaceAll(unitSplit[1], "]:", "")
		coloredWord = strings.ReplaceAll(inputWord, inputWord, "\033[36m"+unitName+"\033[0m"+"\033[33m"+"["+"\033[0m"+"\033[34m"+unitId+"\033[0m"+"\033[33m"+"]"+"\033[0m"+":")
	case strings.HasPrefix(inputWordLower, "kernel:"):
		coloredWord = app.replaceWordLower(inputWord, "kernel", "\033[36m")
	case strings.HasPrefix(inputWordLower, "rsyslogd:"):
		coloredWord = app.replaceWordLower(inputWord, "rsyslogd", "\033[36m")
	case strings.HasPrefix(inputWordLower, "sudo:"):
		coloredWord = app.replaceWordLower(inputWord, "sudo", "\033[36m")
	// Исключения
	case strings.Contains(inputWordLower, "unblock"):
		words := []string{"unblocking", "unblocked", "unblock"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	// Красный (ошибки) [31m]
	case strings.Contains(inputWordLower, "err"):
		words := []string{"stderr", "errors", "error", "erro", "err"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "dis"):
		words := []string{"disconnected", "disconnection", "disconnects", "disconnect", "disabled", "disabling", "disable"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "crash"):
		words := []string{"crashed", "crashing", "crash"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "delet"):
		words := []string{"deletion", "deleted", "deleting", "deletes", "delete"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "remov"):
		words := []string{"removing", "removed", "removes", "remove"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "stop"):
		words := []string{"stopping", "stopped", "stoped", "stops", "stop"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "invalid"):
		words := []string{"invalidation", "invalidating", "invalidated", "invalidate", "invalid"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "abort"):
		words := []string{"aborted", "aborting", "abort"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "block"):
		words := []string{"blocked", "blocker", "blocking", "blocks", "block"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "activ"):
		words := []string{"inactive", "deactivated", "deactivating", "deactivate"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "exit"):
		words := []string{"exited", "exiting", "exits", "exit"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "crit"):
		words := []string{"critical", "critic", "crit"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "fail"):
		words := []string{"failed", "failure", "failing", "fails", "fail"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "reject"):
		words := []string{"rejecting", "rejection", "rejected", "reject"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "fatal"):
		words := []string{"fatality", "fataling", "fatals", "fatal"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "clos"):
		words := []string{"closed", "closing", "close"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "drop"):
		words := []string{"dropped", "droping", "drops", "drop"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "kill"):
		words := []string{"killer", "killing", "kills", "kill"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "cancel"):
		words := []string{"cancellation", "cancelation", "canceled", "cancelling", "canceling", "cancel"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "refus"):
		words := []string{"refusing", "refused", "refuses", "refuse"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "restrict"):
		words := []string{"restricting", "restricted", "restriction", "restrict"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "panic"):
		words := []string{"panicked", "panics", "panic"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[31m")
				break
			}
		}
	case strings.Contains(inputWordLower, "unknown"):
		coloredWord = app.replaceWordLower(inputWord, "unknown", "\033[31m")
	case strings.Contains(inputWordLower, "unavailable"):
		coloredWord = app.replaceWordLower(inputWord, "unavailable", "\033[31m")
	case strings.Contains(inputWordLower, "unsuccessful"):
		coloredWord = app.replaceWordLower(inputWord, "unsuccessful", "\033[31m")
	case strings.Contains(inputWordLower, "found"):
		coloredWord = app.replaceWordLower(inputWord, "found", "\033[31m")
	case strings.Contains(inputWordLower, "denied"):
		coloredWord = app.replaceWordLower(inputWord, "denied", "\033[31m")
	case strings.Contains(inputWordLower, "conflict"):
		coloredWord = app.replaceWordLower(inputWord, "conflict", "\033[31m")
	case strings.Contains(inputWordLower, "false"):
		coloredWord = app.replaceWordLower(inputWord, "false", "\033[31m")
	case strings.Contains(inputWordLower, "none"):
		coloredWord = app.replaceWordLower(inputWord, "none", "\033[31m")
	case strings.Contains(inputWordLower, "null"):
		coloredWord = app.replaceWordLower(inputWord, "null", "\033[31m")
	// Исключения
	case strings.Contains(inputWordLower, "res"):
		words := []string{"resolved", "resolving", "resolve", "restarting", "restarted", "restart"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	// Зеленый (успех) [32m]
	case strings.Contains(inputWordLower, "succe"):
		words := []string{"successfully", "successful", "succeeded", "succeed", "success"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "complet"):
		words := []string{"completed", "completing", "completion", "completes", "complete"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "accept"):
		words := []string{"accepted", "accepting", "acception", "acceptance", "acceptable", "acceptably", "accepte", "accepts", "accept"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "connect"):
		words := []string{"connected", "connecting", "connection", "connects", "connect"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "finish"):
		words := []string{"finished", "finishing", "finish"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "start"):
		words := []string{"started", "starting", "startup", "start"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "creat"):
		words := []string{"created", "creating", "creates", "create"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "enable"):
		words := []string{"enabled", "enables", "enable"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "allow"):
		words := []string{"allowed", "allowing", "allow"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "post"):
		words := []string{"posting", "posted", "postrouting", "post"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "rout"):
		words := []string{"prerouting", "routing", "routes", "route"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "forward"):
		words := []string{"forwarding", "forwards", "forward"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "pass"):
		words := []string{"passed", "passing", "password"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "run"):
		words := []string{"running", "runs", "run"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "add"):
		words := []string{"added", "add"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "open"):
		words := []string{"opening", "opened", "open"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[32m")
				break
			}
		}
	case strings.Contains(inputWordLower, "ok"):
		coloredWord = app.replaceWordLower(inputWord, "ok", "\033[32m")
	case strings.Contains(inputWordLower, "available"):
		coloredWord = app.replaceWordLower(inputWord, "available", "\033[32m")
	case strings.Contains(inputWordLower, "accessible"):
		coloredWord = app.replaceWordLower(inputWord, "accessible", "\033[32m")
	case strings.Contains(inputWordLower, "done"):
		coloredWord = app.replaceWordLower(inputWord, "done", "\033[32m")
	case strings.Contains(inputWordLower, "true"):
		coloredWord = app.replaceWordLower(inputWord, "true", "\033[32m")
	// Синий (статусы) [36m]
	case strings.Contains(inputWordLower, "req"):
		words := []string{"requested", "requests", "request"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "reg"):
		words := []string{"registered", "registeration"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "boot"):
		words := []string{"reboot", "booting", "boot"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "out"):
		words := []string{"stdout", "timeout", "output"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "put"):
		words := []string{"input", "put"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "get"):
		words := []string{"getting", "get"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "set"):
		words := []string{"settings", "setting", "setup", "set"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "head"):
		words := []string{"headers", "header", "heades", "head"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "log"):
		words := []string{"logged", "login"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "load"):
		words := []string{"overloading", "overloaded", "overload", "uploading", "uploaded", "uploads", "upload", "downloading", "downloaded", "downloads", "download", "loading", "loaded", "load"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "read"):
		words := []string{"reading", "readed", "read"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "patch"):
		words := []string{"patching", "patched", "patch"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "up"):
		words := []string{"updates", "updated", "updating", "update", "upgrades", "upgraded", "upgrading", "upgrade", "backup", "up"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "listen"):
		words := []string{"listening", "listener", "listen"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "launch"):
		words := []string{"launched", "launching", "launch"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "chang"):
		words := []string{"changed", "changing", "change"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "clea"):
		words := []string{"cleaning", "cleaner", "clearing", "cleared", "clear"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "skip"):
		words := []string{"skipping", "skipped", "skip"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "miss"):
		words := []string{"missing", "missed"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "mount"):
		words := []string{"mountpoint", "mounted", "mounting", "mount"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "auth"):
		words := []string{"authenticating", "authentication", "authorization"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "conf"):
		words := []string{"configurations", "configuration", "configuring", "configured", "configure", "config", "conf"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "option"):
		words := []string{"options", "option"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "writ"):
		words := []string{"writing", "writed", "write"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "sav"):
		words := []string{"saved", "saving", "save"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "paus"):
		words := []string{"paused", "pausing", "pause"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "filt"):
		words := []string{"filtration", "filtr", "filtering", "filtered", "filter"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "norm"):
		words := []string{"normal", "norm"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "noti"):
		words := []string{"notifications", "notification", "notify", "noting", "notice"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "alert"):
		words := []string{"alerting", "alert"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "in"):
		words := []string{"informations", "information", "informing", "informed", "info", "installation", "installed", "installing", "install", "initialization", "initial", "using"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "down"):
		words := []string{"shutdown", "down"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "us"):
		words := []string{"status", "used", "use"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[36m")
				break
			}
		}
	case strings.Contains(inputWordLower, "debug"):
		coloredWord = app.replaceWordLower(inputWord, "debug", "\033[36m")
	case strings.Contains(inputWordLower, "verbose"):
		coloredWord = app.replaceWordLower(inputWord, "verbose", "\033[36m")
	case strings.HasPrefix(inputWordLower, "trace"):
		coloredWord = app.replaceWordLower(inputWord, "trace", "\033[36m")
	case strings.HasPrefix(inputWordLower, "protocol"):
		coloredWord = app.replaceWordLower(inputWord, "protocol", "\033[36m")
	case strings.Contains(inputWordLower, "level"):
		coloredWord = app.replaceWordLower(inputWord, "level", "\033[36m")
	// Голубой (цифры) [34m]
	// Byte (0x04)
	case app.hexByteRegex.MatchString(inputWord):
		coloredWord = app.hexByteRegex.ReplaceAllStringFunc(inputWord, func(match string) string {
			colored := ""
			for _, char := range match {
				if char == 'x' {
					colored += "\033[35m" + string(char) + "\033[0m"
				} else {
					colored += "\033[34m" + string(char) + "\033[0m"
				}
			}
			return colored
		})
	// DateTime
	case app.dateTimeRegex.MatchString(inputWord):
		coloredWord = app.dateTimeRegex.ReplaceAllStringFunc(inputWord, func(match string) string {
			colored := ""
			for _, char := range match {
				if char == '-' || char == '.' || char == ':' || char == '+' || char == 'T' || char == 'Z' {
					// Пурпурный для символов
					colored += "\033[35m" + string(char) + "\033[0m"
				} else {
					// Синий для цифр
					colored += "\033[34m" + string(char) + "\033[0m"
				}
			}
			return colored
		})
	// Integers
	case app.integersInputRegex.MatchString(inputWord):
		var colored strings.Builder
		// Флаги, для фиксации нахождения внутри числа/символа или нет
		inNumber := false
		inSymbol := false
		for _, char := range inputWord {
			switch {
			case char >= '0' && char <= '9':
				// Если это цифра и мы еще не в числе, открываем цвет
				if !inNumber {
					colored.WriteString("\033[34m")
					inNumber = true
				}
			case char == '/' || char == ':' || char == '.' || char == '-' || char == '+' || char == '%':
				// Красим символы
				colored.WriteString("\033[35m")
				inSymbol = true
				inNumber = false
			default:
				// Если это не цифра и до этого было число, закрываем цвет
				if inNumber {
					inNumber = false
				}
				// Для всех других символов
				colored.WriteString("\033[0m")
			}
			// Добавляем символ в результат
			colored.WriteRune(char)
			// Закрываем цвет для символа
			if inSymbol {
				colored.WriteString("\033[0m")
				inSymbol = false
			}
		}
		// Закрываем цвет, если строка закончилась на числе
		if inNumber {
			colored.WriteString("\033[0m")
		}
		return colored.String()
	// tcpdump
	case strings.Contains(inputWordLower, "tcp"):
		coloredWord = app.replaceWordLower(inputWord, "tcp", "\033[33m")
	case strings.Contains(inputWordLower, "udp"):
		coloredWord = app.replaceWordLower(inputWord, "udp", "\033[33m")
	case strings.Contains(inputWordLower, "icmp"):
		coloredWord = app.replaceWordLower(inputWord, "icmp", "\033[33m")
	case strings.Contains(inputWordLower, "ip"):
		words := []string{"ip4", "ipv4", "ip6", "ipv6", "ip"}
		for _, word := range words {
			if strings.Contains(inputWordLower, word) {
				coloredWord = app.replaceWordLower(inputWord, word, "\033[33m")
				break
			}
		}
	// Update delimiter
	case strings.Contains(inputWord, "⎯"):
		coloredWord = strings.ReplaceAll(inputWord, inputWord, "\033[35m"+inputWord+"\033[0m")
	// Исключения
	case strings.Contains(inputWordLower, "not"):
		coloredWord = app.replaceWordLower(inputWord, "not", "\033[31m")
	}
	return coloredWord
}

// ---------------------------------------- Log output ----------------------------------------

// Функция для обновления вывода журнала (параметр для прокрутки в самый вниз)
func (app *App) updateLogsView(lowerDown bool) {
	// Получаем доступ к выводу журнала
	v, err := app.gui.View("logs")
	if err != nil {
		return
	}
	// Очищаем окно для отображения новых строк
	v.Clear()
	// Получаем ширину и высоту окна
	viewWidth, viewHeight := v.Size()
	// Опускаем в самый низ, только если это не ручной скролл (отключается параметром)
	if lowerDown {
		// Если количество строк больше высоты окна, опускаем в самый низ
		if len(app.filteredLogLines) > viewHeight-1 {
			app.logScrollPos = len(app.filteredLogLines) - viewHeight - 1
		} else {
			app.logScrollPos = 0
		}
	}
	// Определяем количество строк для отображения, начиная с позиции logScrollPos
	startLine := app.logScrollPos
	endLine := startLine + viewHeight
	if endLine > len(app.filteredLogLines) {
		endLine = len(app.filteredLogLines)
	}
	// Учитываем auto wrap (только в конце лога)
	if app.logScrollPos == len(app.filteredLogLines)-viewHeight-1 {
		var viewLines int = 0                             // количество строк для вывода
		var viewCounter int = 0                           // обратный счетчик видимых строк для остановки
		var viewIndex int = len(app.filteredLogLines) - 1 // начальный индекс для строк с конца
		for {
			// Фиксируем текущую входную строку и счетчик
			viewLines += 1
			viewCounter += 1
			// Получаем длинну видимых символов в строке с конца
			var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
			lengthLine := len([]rune(ansiEscape.ReplaceAllString(app.filteredLogLines[viewIndex], "")))
			// Если длинна строки больше ширины окна, получаем количество дополнительных строк
			if lengthLine > viewWidth {
				// Увеличивая счетчик и пропускаем строки
				viewCounter += lengthLine / viewWidth
			}
			// Если счетчик привысил количество видимых строк, вычетаем последнюю строку из видимости
			if viewCounter > viewHeight {
				viewLines -= 1
			}
			if viewCounter >= viewHeight {
				break
			}
			// Уменьшаем индекс
			viewIndex -= 1
		}
		for i := len(app.filteredLogLines) - viewLines - 1; i < endLine; i++ {
			fmt.Fprintln(v, app.filteredLogLines[i])
		}
	} else {
		// Проходим по отфильтрованным строкам и выводим их
		for i := startLine; i < endLine; i++ {
			fmt.Fprintln(v, app.filteredLogLines[i])
		}
	}
	// Вычисляем процент прокрутки и обновляем заголовок
	var percentage int = 0
	if len(app.filteredLogLines) > 0 {
		// Стартовая позиция + размер текущего вывода логов и округляем в большую сторону (math)
		percentage = int(math.Ceil(float64((startLine+viewHeight)*100) / float64(len(app.filteredLogLines))))
		if percentage > 100 {
			v.Title = fmt.Sprintf(
				"Logs: 100%% (%d) ["+app.debugLoadTime+"/"+app.debugColorTime+"]",
				len(app.filteredLogLines),
			)
		} else {
			v.Title = fmt.Sprintf("Logs: %d%% (%d/%d) ["+app.debugLoadTime+"/"+app.debugColorTime+"]",
				percentage,
				startLine+1+viewHeight,
				len(app.filteredLogLines),
			)
		}
	} else {
		v.Title = "Logs: 0% (0) [" + app.debugLoadTime + "/" + app.debugColorTime + "]"
	}
	v.TitleColor = gocui.ColorYellow
	app.viewScrollLogs(percentage)
}

// Функция для обновления интерфейса скроллинга
func (app *App) viewScrollLogs(percentage int) {
	vScroll, _ := app.gui.View("scrollLogs")
	vScroll.Clear()
	// Определяем высоту окна
	_, viewHeight := vScroll.Size()
	// Заполняем скролл пробелами, если вывод пустой или не выходит за пределы окна
	if percentage == 0 || percentage > 100 {
		fmt.Fprintln(vScroll, "▲")
		for i := 1; i < viewHeight-1; i++ {
			fmt.Fprintln(vScroll, " ")
		}
		fmt.Fprintln(vScroll, "▼")
	} else {
		// Рассчитываем позицию курсора (корректируем процент на размер скролла и верхней стрелки)
		scrollPosition := (viewHeight*percentage)/100 - 3 - 1
		fmt.Fprintln(vScroll, "▲")
		// Выводим строки с пробелами и символом █
	for_scroll:
		for i := 1; i < viewHeight-3; i++ {
			// Проверяем текущую поизицию
			switch {
			case i == scrollPosition:
				// Выводим скролл
				fmt.Fprintln(vScroll, "███")
			case scrollPosition <= 0 || app.logScrollPos == 0:
				// Если вышли за пределы окна или текст находится в самом начале, устанавливаем курсор в начало
				fmt.Fprintln(vScroll, "███")
				// Остальное заполняем пробелами с учетом стрелки и курсора (-4) до последней стрелки (-1)
				for i := 4; i < viewHeight-1; i++ {
					fmt.Fprintln(vScroll, " ")
				}
				break for_scroll
			default:
				// Пробелы на остальных строках
				fmt.Fprintln(vScroll, " ")
			}
		}
		fmt.Fprintln(vScroll, "▼")
	}
}

// Функция для скроллинга вниз
func (app *App) scrollDownLogs(step int) error {
	v, err := app.gui.View("logs")
	if err != nil {
		return err
	}
	// Получаем высоту окна, что бы не опускать лог с пустыми строками
	_, viewHeight := v.Size()
	// Проверяем, что размер журнала больше размера окна
	if len(app.filteredLogLines) > viewHeight {
		// Увеличиваем позицию прокрутки
		app.logScrollPos += step
		// Если достигнут конец списка, останавливаем на максимальной длинне с учетом высоты окна
		if app.logScrollPos > len(app.filteredLogLines)-1-viewHeight {
			app.logScrollPos = len(app.filteredLogLines) - 1 - viewHeight
			// Включаем автоскролл (если он не отключен)
			if !app.disableAutoScroll {
				app.autoScroll = true
			} else {
				app.autoScroll = false
			}
			if !app.testMode {
				vLog, err := app.gui.View("logs")
				if err != nil {
					return err
				}
				vLog.Subtitle = fmt.Sprintf("[tail: %s lines | auto-update: %t (%d sec) | docker: %s (%s) | color: %t]", app.logViewCount, app.autoScroll, app.logUpdateSeconds, app.dockerStreamLogsStr, app.dockerStreamMode, app.colorMode)
			}
		}
		// Вызываем функцию для обновления отображения журнала
		app.updateLogsView(false)
	}
	return nil
}

// Функция для скроллинга вверх
func (app *App) scrollUpLogs(step int) error {
	app.logScrollPos -= step
	if app.logScrollPos < 0 {
		app.logScrollPos = 0
	}
	// Отключаем автоскролл
	app.autoScroll = false
	if !app.testMode {
		vLog, err := app.gui.View("logs")
		if err != nil {
			return err
		}
		vLog.Subtitle = fmt.Sprintf("[tail: %s lines | auto-update: %t (%d sec) | docker: %s (%s) | color: %t]", app.logViewCount, app.autoScroll, app.logUpdateSeconds, app.dockerStreamLogsStr, app.dockerStreamMode, app.colorMode)
	}
	app.updateLogsView(false)
	return nil
}

// Функция для переход к началу журнала
func (app *App) pageUpLogs() {
	app.logScrollPos = 0
	app.autoScroll = false
	if !app.testMode {
		vLog, _ := app.gui.View("logs")
		vLog.Subtitle = fmt.Sprintf("[tail: %s lines | auto-update: %t (%d sec) | docker: %s (%s) | color: %t]", app.logViewCount, app.autoScroll, app.logUpdateSeconds, app.dockerStreamLogsStr, app.dockerStreamMode, app.colorMode)
	}
	app.updateLogsView(false)
}

// Функция для очистки поля ввода фильтра вывода лога
func (app *App) clearFilterEditor(g *gocui.Gui) {
	v, _ := g.View("filter")
	// Очищаем содержимое View
	v.Clear()
	// Устанавливаем курсор на начальную позицию
	if err := v.SetCursor(0, 0); err != nil {
		return
	}
	// Очищаем буфер фильтра
	app.filterText = ""
	app.applyFilter(false)
}

// Функция для очистки поля ввода фильтра списков
func (app *App) clearFilterListEditor(g *gocui.Gui) {
	v, _ := g.View("filterList")
	v.Clear()
	if err := v.SetCursor(0, 0); err != nil {
		return
	}
	app.filterListText = ""
	app.applyFilterList()
}

// Функция для обновления последнего выбранного вывода лога (параметр для загрузки журнала)
func (app *App) updateLogOutput(newUpdate bool) {
	// Выполняем обновление интерфейса через метод Update для иницилизации перерисовки интерфейса
	app.gui.Update(func(g *gocui.Gui) error {
		// Сбрасываем автоскролл, что бы опустить журнал вниз, т.к. это всегда ручное обновление
		if !app.disableAutoScroll {
			app.autoScroll = true
		} else {
			app.autoScroll = false
		}
		if !app.testMode {
			vLog, err := app.gui.View("logs")
			if err != nil {
				return err
			}
			vLog.Subtitle = fmt.Sprintf("[tail: %s lines | auto-update: %t (%d sec) | docker: %s (%s) | color: %t]", app.logViewCount, app.autoScroll, app.logUpdateSeconds, app.dockerStreamLogsStr, app.dockerStreamMode, app.colorMode)
		}
		switch app.lastWindow {
		case "services":
			if app.fastMode {
				go func() {
					app.loadJournalLogs(app.lastSelected, newUpdate)
				}()
			} else {
				app.loadJournalLogs(app.lastSelected, newUpdate)
			}
		case "varLogs":
			if app.fastMode {
				go func() {
					app.loadFileLogs(app.lastSelected, newUpdate)
				}()
			} else {
				app.loadFileLogs(app.lastSelected, newUpdate)
			}
		case "docker":
			if app.fastMode {
				go func() {
					app.loadDockerLogs(app.lastSelected, newUpdate)
				}()
			} else {
				app.loadDockerLogs(app.lastSelected, newUpdate)
			}
		}
		return nil
	})
}

// Запускает фоновое обновление с изменяемым интервалом (параметры для обновления времени и загрузки журнала)
func (app *App) updateLogBackground(secondsChan chan int, newUpdate bool) {
	seconds := app.logUpdateSeconds
	// Проверяем, есть ли в канале новое значение интервала
	select {
	case s := <-secondsChan:
		seconds = s
	default:
	}
	// Таймер
	ticker := time.NewTicker(time.Duration(seconds) * time.Second)
	// Гарантируем остановку таймера при выходе из функции
	defer ticker.Stop()
	for {
		select {
		// Если в канал поступило новое значение, перезапускаем таймер с новым интервалом
		case newSeconds := <-secondsChan:
			ticker.Reset(time.Duration(newSeconds) * time.Second)
		// Когда срабатывает таймер, выполняем обновление логов
		case <-ticker.C:
			// Обновляем журнал только если включен автоскролл
			if app.autoScroll {
				app.gui.Update(func(g *gocui.Gui) error {
					switch app.lastWindow {
					case "services":
						if app.fastMode {
							go func() {
								app.loadJournalLogs(app.lastSelected, newUpdate)
							}()
						} else {
							app.loadJournalLogs(app.lastSelected, newUpdate)
						}
					case "varLogs":
						if app.fastMode {
							go func() {
								app.loadFileLogs(app.lastSelected, newUpdate)
							}()
						} else {
							app.loadFileLogs(app.lastSelected, newUpdate)
						}
					case "docker":
						if app.fastMode {
							go func() {
								app.loadDockerLogs(app.lastSelected, newUpdate)
							}()
						} else {
							app.loadDockerLogs(app.lastSelected, newUpdate)
						}
					}
					return nil
				})
			}
		}
	}
}

// Функция для обновления вывода при изменение размера окна
func (app *App) updateWindowSize(seconds int) {
	for {
		app.gui.Update(func(g *gocui.Gui) error {
			v, err := g.View("logs")
			if err != nil {
				log.Panicln(err)
			}
			windowWidth, windowHeight := v.Size()
			if windowWidth != app.windowWidth || windowHeight != app.windowHeight {
				app.windowWidth, app.windowHeight = windowWidth, windowHeight
				app.updateLogsView(true)
				if v, err := g.View("services"); err == nil {
					_, viewHeight := v.Size()
					app.maxVisibleServices = viewHeight
				}
				if v, err := g.View("varLogs"); err == nil {
					_, viewHeight := v.Size()
					app.maxVisibleFiles = viewHeight
				}
				if v, err := g.View("docker"); err == nil {
					_, viewHeight := v.Size()
					app.maxVisibleDockerContainers = viewHeight
				}
				app.applyFilterList()
			}
			// Обновляем ширину для фильтрации по дате
			maxX, _ := g.Size()
			leftPanelWidth := maxX / 4
			filterWidth := (maxX - leftPanelWidth - 1) / 2
			if _, err := g.View("sinceFilter"); err == nil {
				if _, err := g.SetView("sinceFilter", leftPanelWidth+1, 0, leftPanelWidth+1+filterWidth, 2, 0); err != nil {
					return nil
				}
			}
			if _, err := g.View("untilFilter"); err == nil {
				if _, err := g.SetView("untilFilter", leftPanelWidth+1+filterWidth+1, 0, maxX-1, 2, 0); err != nil {
					return nil
				}
			}
			return nil
		})
		time.Sleep(time.Duration(seconds) * time.Second)
	}
}

// Функция для фиксации места загрузки журнала с помощью делимитра (параметр для обновления места и времени загрузки)
func (app *App) updateDelimiter(newUpdate bool) {
	if newUpdate {
		// Фиксируем (сохраняем) предпоследнюю (-2, т.к. последняя строка всегда пустая) строку для вставки делимитра (если это ручной выбор из списка) или выходим
		if len(app.currentLogLines) > 2 {
			app.lastUpdateLine = app.currentLogLines[len(app.currentLogLines)-2]
		} else {
			return
		}
		// Сбрасываем автоскролл
		if !app.disableAutoScroll {
			app.autoScroll = true
		} else {
			app.autoScroll = false
		}
		if !app.testMode {
			vLog, _ := app.gui.View("logs")
			vLog.Subtitle = fmt.Sprintf("[tail: %s lines | auto-update: %t (%d sec) | docker: %s (%s) | color: %t]", app.logViewCount, app.autoScroll, app.logUpdateSeconds, app.dockerStreamLogsStr, app.dockerStreamMode, app.colorMode)
		}
		// Фиксируем новое время загрузки журнала
		app.updateTime = time.Now().Format("15:04:05")
	} else {
		// Ищем индекс строки в массиве с конца
		delimiterIndex := 0
		for i := len(app.currentLogLines) - 1; i >= 0; i-- {
			if app.currentLogLines[i] == app.lastUpdateLine {
				delimiterIndex = i
				break
			}
		}
		// Проверяем, что строка найдена и найденный индекс меньше длинны массива строк
		if delimiterIndex > 0 && delimiterIndex < len(app.currentLogLines)-2 {
			// Формируем длинну делимитра
			v, _ := app.gui.View("logs")
			width, _ := v.Size()
			lengthDelimiter := width/2 - 5
			delimiter1 := strings.Repeat("⎯", lengthDelimiter)
			delimiter2 := delimiter1
			if width > lengthDelimiter+lengthDelimiter+10 {
				delimiter2 = strings.Repeat("⎯", lengthDelimiter+1)
			}
			var delimiterString string = delimiter1 + " " + app.updateTime + " " + delimiter2
			// Вставляем новую строку после указанного индекса + 1 пустая строка (сдвигая остальные строки массива)
			app.currentLogLines = append(app.currentLogLines[:delimiterIndex+1],
				append([]string{delimiterString}, app.currentLogLines[delimiterIndex+1:]...)...)
		}
	}
}

// ---------------------------------------- Key Binding ----------------------------------------

// Карта для сопостовления сочетаний клавиш со значениями из конфигурации
var keyMap = map[string]gocui.Key{
	"f1":        gocui.KeyF1,
	"f2":        gocui.KeyF2,
	"f3":        gocui.KeyF3,
	"f4":        gocui.KeyF4,
	"f5":        gocui.KeyF5,
	"f6":        gocui.KeyF6,
	"f7":        gocui.KeyF7,
	"f8":        gocui.KeyF8,
	"f9":        gocui.KeyF9,
	"f10":       gocui.KeyF10,
	"f11":       gocui.KeyF11,
	"f12":       gocui.KeyF12,
	"ctrl+a":    gocui.KeyCtrlA,
	"ctrl+b":    gocui.KeyCtrlB,
	"ctrl+c":    gocui.KeyCtrlC,
	"ctrl+d":    gocui.KeyCtrlD,
	"ctrl+e":    gocui.KeyCtrlE,
	"ctrl+f":    gocui.KeyCtrlF,
	"ctrl+g":    gocui.KeyCtrlG,
	"ctrl+h":    gocui.KeyCtrlH,
	"ctrl+i":    gocui.KeyCtrlI,
	"ctrl+j":    gocui.KeyCtrlJ,
	"ctrl+k":    gocui.KeyCtrlK,
	"ctrl+l":    gocui.KeyCtrlL,
	"ctrl+m":    gocui.KeyCtrlM,
	"ctrl+n":    gocui.KeyCtrlN,
	"ctrl+o":    gocui.KeyCtrlO,
	"ctrl+p":    gocui.KeyCtrlP,
	"ctrl+q":    gocui.KeyCtrlQ,
	"ctrl+r":    gocui.KeyCtrlR,
	"ctrl+s":    gocui.KeyCtrlS,
	"ctrl+t":    gocui.KeyCtrlT,
	"ctrl+u":    gocui.KeyCtrlU,
	"ctrl+v":    gocui.KeyCtrlV,
	"ctrl+w":    gocui.KeyCtrlW,
	"ctrl+x":    gocui.KeyCtrlX,
	"ctrl+y":    gocui.KeyCtrlY,
	"ctrl+z":    gocui.KeyCtrlZ,
	"tab":       gocui.KeyTab,
	"shift+tab": gocui.KeyBacktab,
	"enter":     gocui.KeyEnter,
	"space":     gocui.KeySpace,
	"backspace": gocui.KeyBackspace,
	"delete":    gocui.KeyDelete,
	"escape":    gocui.KeyEsc,
}

// Функция для опредиления клавиш из конфигурации
func getHotkey(configKey, defaultKey string) any {
	// Опускаем регистр для всех вхождений (букв и сочетаний)
	inputKey := strings.ToLower(configKey)
	// Если это одна буква, конвертируем string в rune (используя DecodeRuneInString) и извлекаем значение
	if len(inputKey) == 1 {
		if r, _ := utf8.DecodeRuneInString(inputKey); r != utf8.RuneError {
			return r
		}
	} else {
		// Если сочетание клавиш содержит shift, извлекаем последнюю букву в верхнем регистре
		if strings.HasPrefix(inputKey, "shift+") && inputKey != "shift+tab" {
			inputKey = strings.ToTitle(configKey)
			return []rune(inputKey)[len(inputKey)-1]
		} else {
			// Ищем сочетание клавиш в карте
			key, exists := keyMap[inputKey]
			if exists {
				return key
			}
		}
	}
	// Возвращяем значение по умолчанию (которое передается во втором параметре)
	if len(defaultKey) == 1 {
		if r, _ := utf8.DecodeRuneInString(defaultKey); r != utf8.RuneError {
			return r
		}
	}
	return keyMap[defaultKey]
}

// Функция для биндинга клавиш
func (app *App) setupKeybindings() error {
	// Help (F1)
	// Открытие окна справки
	customHelp := getHotkey(config.Hotkeys.Help, "f1")
	helpHandler := func(g *gocui.Gui, v *gocui.View) error {
		app.showInterfaceHelp(g)
		// Удаляем глобальные биндинги
		g.DeleteKeybindings("")
		// Удаляем все биндинги назначенные для окон
		viewsRange := []string{"filterList", "services", "varLogs", "docker", "filter", "sinceFilter", "untilFilter", "logs"}
		for _, viewName := range viewsRange {
			g.DeleteKeybindings(viewName)
		}
		// Создаем временный биндинг на Esc для закрытия окна
		if err := app.gui.SetKeybinding("", gocui.KeyEsc, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			app.closeHelp(g)
			// Возвращяем стандартные биндиги после закрытия окна справки
			if err := app.setupKeybindings(); err != nil {
				log.Panicln("Error key bindings", err)
			}
			return nil
		}); err != nil {
			return err
		}
		if err := app.gui.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
			return err
		}
		return nil
	}
	if err := app.gui.SetKeybinding("", customHelp, gocui.ModNone, helpHandler); err != nil {
		return err
	}

	// ↑↑↑
	// Пролистывание вверх
	// Up (1)
	if err := app.gui.SetKeybinding("services", gocui.KeyArrowUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevService(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", gocui.KeyArrowUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevFileName(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.KeyArrowUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevDockerContainer(v, 1)
	}); err != nil {
		return err
	}
	// PgUp (1) #10
	if err := app.gui.SetKeybinding("services", gocui.KeyPgup, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevService(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", gocui.KeyPgup, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevFileName(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.KeyPgup, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevDockerContainer(v, 1)
	}); err != nil {
		return err
	}
	// Custom up from config
	// Default: k (1)
	customUp := getHotkey(config.Hotkeys.Up, "k")
	if err := app.gui.SetKeybinding("services", customUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevService(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", customUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevFileName(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", customUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevDockerContainer(v, 1)
	}); err != nil {
		return err
	}
	// Shift+Up (10)
	if err := app.gui.SetKeybinding("services", gocui.KeyArrowUp, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevService(v, 10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", gocui.KeyArrowUp, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevFileName(v, 10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.KeyArrowUp, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevDockerContainer(v, 10)
	}); err != nil {
		return err
	}
	// Shift+PgUp (10)
	if err := app.gui.SetKeybinding("services", gocui.KeyPgup, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevService(v, 10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", gocui.KeyPgup, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevFileName(v, 10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.KeyPgup, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevDockerContainer(v, 10)
	}); err != nil {
		return err
	}
	// Custom up from config
	// Default: shift+k (10)
	customQuickUp := getHotkey(config.Hotkeys.QuickUp, "K")
	if err := app.gui.SetKeybinding("services", customQuickUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevService(v, 10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", customQuickUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevFileName(v, 10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", customQuickUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevDockerContainer(v, 10)
	}); err != nil {
		return err
	}
	// Alt+Up (100)
	if err := app.gui.SetKeybinding("services", gocui.KeyArrowUp, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevService(v, 100)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", gocui.KeyArrowUp, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevFileName(v, 100)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.KeyArrowUp, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevDockerContainer(v, 100)
	}); err != nil {
		return err
	}
	// Alt+PgUp (100)
	if err := app.gui.SetKeybinding("services", gocui.KeyPgup, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevService(v, 100)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", gocui.KeyPgup, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevFileName(v, 100)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.KeyPgup, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevDockerContainer(v, 100)
	}); err != nil {
		return err
	}
	// Custom up from config
	// Default: ctrl+k (100)
	customVeryQuickUp := getHotkey(config.Hotkeys.VeryQuickUp, "ctrl+k")
	if err := app.gui.SetKeybinding("services", customVeryQuickUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevService(v, 100)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", customVeryQuickUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevFileName(v, 100)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", customVeryQuickUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevDockerContainer(v, 100)
	}); err != nil {
		return err
	}

	// ↓↓↓
	// Перемещение вниз к следующей службе (функция nextService), файлу (nextFileName) или контейнеру (nextDockerContainer)
	// Down (1)
	if err := app.gui.SetKeybinding("services", gocui.KeyArrowDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextService(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", gocui.KeyArrowDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextFileName(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.KeyArrowDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextDockerContainer(v, 1)
	}); err != nil {
		return err
	}
	// PgDown (1) #10
	if err := app.gui.SetKeybinding("services", gocui.KeyPgdn, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextService(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", gocui.KeyPgdn, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextFileName(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.KeyPgdn, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextDockerContainer(v, 1)
	}); err != nil {
		return err
	}
	// Custom down from config
	// Default: j (1)
	customDown := getHotkey(config.Hotkeys.Down, "j")
	if err := app.gui.SetKeybinding("services", customDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextService(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", customDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextFileName(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", customDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextDockerContainer(v, 1)
	}); err != nil {
		return err
	}
	// Быстрое пролистывание вниз через 10 записей
	// Shift+Down (10)
	if err := app.gui.SetKeybinding("services", gocui.KeyArrowDown, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextService(v, 10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", gocui.KeyArrowDown, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextFileName(v, 10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.KeyArrowDown, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextDockerContainer(v, 10)
	}); err != nil {
		return err
	}
	// Shift+PgDown (10)
	if err := app.gui.SetKeybinding("services", gocui.KeyPgdn, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextService(v, 10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", gocui.KeyPgdn, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextFileName(v, 10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.KeyPgdn, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextDockerContainer(v, 10)
	}); err != nil {
		return err
	}
	// Custom down from config
	// Default: shift+j (10)
	customQuickDown := getHotkey(config.Hotkeys.QuickDown, "J")
	if err := app.gui.SetKeybinding("services", customQuickDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextService(v, 10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", customQuickDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextFileName(v, 10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", customQuickDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextDockerContainer(v, 10)
	}); err != nil {
		return err
	}
	// Alt+Down (100)
	if err := app.gui.SetKeybinding("services", gocui.KeyArrowDown, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextService(v, 100)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", gocui.KeyArrowDown, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextFileName(v, 100)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.KeyArrowDown, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextDockerContainer(v, 100)
	}); err != nil {
		return err
	}
	// Alt+PgDown (100)
	if err := app.gui.SetKeybinding("services", gocui.KeyPgdn, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextService(v, 100)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", gocui.KeyPgdn, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextFileName(v, 100)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.KeyPgdn, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextDockerContainer(v, 100)
	}); err != nil {
		return err
	}
	// Custom down from config
	// Default: ctrl+j (100)
	customVeryQuickDown := getHotkey(config.Hotkeys.VeryQuickDown, "ctrl+j")
	if err := app.gui.SetKeybinding("services", customVeryQuickDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextService(v, 100)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", customVeryQuickDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextFileName(v, 100)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", customVeryQuickDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextDockerContainer(v, 100)
	}); err != nil {
		return err
	}

	// Filtering mode (↑/↓)
	// Переключение между режимами фильтрации через Up/Down для выбранного окна
	if err := app.gui.SetKeybinding("filter", gocui.KeyArrowUp, gocui.ModNone, app.setFilterModeRight); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("filter", gocui.KeyArrowDown, gocui.ModNone, app.setFilterModeLeft); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("sinceFilter", gocui.KeyArrowUp, gocui.ModNone, app.setFilterModeRight); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("sinceFilter", gocui.KeyArrowDown, gocui.ModNone, app.setFilterModeLeft); err != nil {
		return err
	}
	// PgUp/PgDown
	if err := app.gui.SetKeybinding("filter", gocui.KeyPgup, gocui.ModNone, app.setFilterModeRight); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("filter", gocui.KeyPgdn, gocui.ModNone, app.setFilterModeLeft); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("sinceFilter", gocui.KeyPgup, gocui.ModNone, app.setFilterModeRight); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("sinceFilter", gocui.KeyPgdn, gocui.ModNone, app.setFilterModeLeft); err != nil {
		return err
	}
	// Custom up and down for switch filter mode from config (ctrl+k b ctrl+j)
	customUpFilterMode := getHotkey(config.Hotkeys.SwitchFilterMode, "ctrl+k")
	customDownFilterMode := getHotkey(config.Hotkeys.BackSwitchFilterMode, "ctrl+j")
	if err := app.gui.SetKeybinding("filter", customUpFilterMode, gocui.ModNone, app.setFilterModeRight); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("filter", customDownFilterMode, gocui.ModNone, app.setFilterModeLeft); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("sinceFilter", customUpFilterMode, gocui.ModNone, app.setFilterModeRight); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("sinceFilter", customDownFilterMode, gocui.ModNone, app.setFilterModeLeft); err != nil {
		return err
	}

	// ←/→
	// Custom left and right from config
	customLeft := getHotkey(config.Hotkeys.Left, "h")
	customRight := getHotkey(config.Hotkeys.Right, "l")
	// Переключение выбора журналов для systemd/journald (отключено для Windows)
	if app.getOS != "windows" {
		// Left/Right
		if err := app.gui.SetKeybinding("services", gocui.KeyArrowLeft, gocui.ModNone, app.setUnitListLeft); err != nil {
			return err
		}
		if err := app.gui.SetKeybinding("services", gocui.KeyArrowRight, gocui.ModNone, app.setUnitListRight); err != nil {
			return err
		}
		// [/]
		if err := app.gui.SetKeybinding("services", '[', gocui.ModNone, app.setUnitListLeft); err != nil {
			return err
		}
		if err := app.gui.SetKeybinding("services", ']', gocui.ModNone, app.setUnitListRight); err != nil {
			return err
		}
		// Default: h/l (100)
		if err := app.gui.SetKeybinding("services", customLeft, gocui.ModNone, app.setUnitListLeft); err != nil {
			return err
		}
		if err := app.gui.SetKeybinding("services", customRight, gocui.ModNone, app.setUnitListRight); err != nil {
			return err
		}
	}
	// Переключение выбора журналов для File System
	if app.keybindingsEnabled {
		// Установка привязок
		if err := app.gui.SetKeybinding("varLogs", gocui.KeyArrowLeft, gocui.ModNone, app.setLogFilesListLeft); err != nil {
			return err
		}
		if err := app.gui.SetKeybinding("varLogs", gocui.KeyArrowRight, gocui.ModNone, app.setLogFilesListRight); err != nil {
			return err
		}
		if err := app.gui.SetKeybinding("varLogs", '[', gocui.ModNone, app.setLogFilesListLeft); err != nil {
			return err
		}
		if err := app.gui.SetKeybinding("varLogs", ']', gocui.ModNone, app.setLogFilesListRight); err != nil {
			return err
		}
		if err := app.gui.SetKeybinding("varLogs", customLeft, gocui.ModNone, app.setLogFilesListLeft); err != nil {
			return err
		}
		if err := app.gui.SetKeybinding("varLogs", customRight, gocui.ModNone, app.setLogFilesListRight); err != nil {
			return err
		}
	} else {
		// Удаление привязок
		if err := app.gui.DeleteKeybinding("varLogs", gocui.KeyArrowLeft, gocui.ModNone); err != nil {
			return err
		}
		if err := app.gui.DeleteKeybinding("varLogs", gocui.KeyArrowRight, gocui.ModNone); err != nil {
			return err
		}
		if err := app.gui.DeleteKeybinding("varLogs", '[', gocui.ModNone); err != nil {
			return err
		}
		if err := app.gui.DeleteKeybinding("varLogs", ']', gocui.ModNone); err != nil {
			return err
		}
		if err := app.gui.DeleteKeybinding("varLogs", customLeft, gocui.ModNone); err != nil {
			return err
		}
		if err := app.gui.DeleteKeybinding("varLogs", customRight, gocui.ModNone); err != nil {
			return err
		}
	}
	// Переключение выбора журналов для Containerization System
	if err := app.gui.SetKeybinding("docker", gocui.KeyArrowLeft, gocui.ModNone, app.setContainersListLeft); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.KeyArrowRight, gocui.ModNone, app.setContainersListRight); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", '[', gocui.ModNone, app.setContainersListLeft); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", ']', gocui.ModNone, app.setContainersListRight); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", customLeft, gocui.ModNone, app.setContainersListLeft); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", customRight, gocui.ModNone, app.setContainersListRight); err != nil {
		return err
	}

	// Logs ↓↓↓
	// Пролистывание вывода журнала через 1/10/500 записей вниз
	// Down/PgDown/j (1)
	if err := app.gui.SetKeybinding("logs", gocui.KeyArrowDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollDownLogs(1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", gocui.KeyPgdn, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollDownLogs(1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", customDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollDownLogs(1)
	}); err != nil {
		return err
	}
	// Shift + Down/PgDown/j (10)
	if err := app.gui.SetKeybinding("logs", gocui.KeyArrowDown, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollDownLogs(10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", gocui.KeyPgdn, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollDownLogs(10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", customQuickDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollDownLogs(10)
	}); err != nil {
		return err
	}
	// Alt + Down/PgDown and Ctrl+j (500)
	if err := app.gui.SetKeybinding("logs", gocui.KeyArrowDown, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollDownLogs(500)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", gocui.KeyPgdn, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollDownLogs(500)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", customVeryQuickDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollDownLogs(500)
	}); err != nil {
		return err
	}

	// Logs ↑↑↑
	// Пролистывание вывода журнала через 1/10/500 записей вверх
	// Up/PgUp/k (1)
	if err := app.gui.SetKeybinding("logs", gocui.KeyArrowUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollUpLogs(1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", gocui.KeyPgup, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollUpLogs(1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", customUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollUpLogs(1)
	}); err != nil {
		return err
	}
	// Shift + Up/PgUp/k (10)
	if err := app.gui.SetKeybinding("logs", gocui.KeyArrowUp, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollUpLogs(10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", gocui.KeyPgup, gocui.ModShift, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollUpLogs(10)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", customQuickUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollUpLogs(10)
	}); err != nil {
		return err
	}
	// Alt + Up/PgUp and Ctrl+k (500)
	if err := app.gui.SetKeybinding("logs", gocui.KeyArrowUp, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollUpLogs(500)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", gocui.KeyPgup, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollUpLogs(500)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", customVeryQuickUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollUpLogs(500)
	}); err != nil {
		return err
	}

	// Tab для переключения между окнами
	customTab := getHotkey(config.Hotkeys.SwitchWindow, "tab")
	if err := app.gui.SetKeybinding("", customTab, gocui.ModNone, app.nextView); err != nil {
		return err
	}
	// Shift+Tab (Back Tab) для переключения между окнами в обратном порядке
	customBackTab := getHotkey(config.Hotkeys.BackSwitchWindows, "shift+tab")
	if err := app.gui.SetKeybinding("", customBackTab, gocui.ModNone, app.backView); err != nil {
		return err
	}

	// Enter для выбора службы и загрузки журналов
	customEnter := getHotkey(config.Hotkeys.LoadJournal, "enter")
	if err := app.gui.SetKeybinding("services", customEnter, gocui.ModNone, app.selectService); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", customEnter, gocui.ModNone, app.selectFile); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", customEnter, gocui.ModNone, app.selectDocker); err != nil {
		return err
	}
	// Enter для загрузки журнала из фильтра по времени
	if err := app.gui.SetKeybinding("sinceFilter", customEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		app.updateLogOutput(true)
		return nil
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("untilFilter", customEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		app.updateLogOutput(true)
		return nil
	}); err != nil {
		return err
	}

	// filter (/) slash
	// Переключение фокуса на окно фильтрации списков журналов
	customSlash := getHotkey(config.Hotkeys.GoToFilter, "/")
	if err := app.gui.SetKeybinding("services", customSlash, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		app.lastCurrentView = "services"
		app.backCurrentView = true
		return app.setSelectView(app.gui, "filterList")
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", customSlash, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		app.lastCurrentView = "varLogs"
		app.backCurrentView = true
		return app.setSelectView(app.gui, "filterList")
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", customSlash, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		app.lastCurrentView = "docker"
		app.backCurrentView = true
		return app.setSelectView(app.gui, "filterList")
	}); err != nil {
		return err
	}
	// В окне вывода журнала переключаемся на фильтр журнала
	if err := app.gui.SetKeybinding("logs", customSlash, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		app.lastCurrentView = "logs"
		app.backCurrentView = true
		return app.setSelectView(app.gui, "filter")
	}); err != nil {
		return err
	}
	// Возврат к последнему окну до использования слэша с использование Enter из окна фильтрации
	if err := app.gui.SetKeybinding("filterList", customEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.backCurrentView {
			app.backCurrentView = false
			return app.setSelectView(app.gui, app.lastCurrentView)
		} else {
			return nil
		}
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("filter", customEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.backCurrentView {
			app.backCurrentView = false
			return app.setSelectView(app.gui, app.lastCurrentView)
		} else {
			return nil
		}
	}); err != nil {
		return err
	}

	// End/Ctrl+E
	// Перемещение к концу журнала
	if err := app.gui.SetKeybinding("", gocui.KeyEnd, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		// Сбрасываем автоскролл
		if !app.disableAutoScroll {
			app.autoScroll = true
		} else {
			app.autoScroll = false
		}
		vLog, err := app.gui.View("logs")
		if err != nil {
			return err
		}
		vLog.Subtitle = fmt.Sprintf("[tail: %s lines | auto-update: %t (%d sec) | docker: %s (%s) | color: %t]", app.logViewCount, app.autoScroll, app.logUpdateSeconds, app.dockerStreamLogsStr, app.dockerStreamMode, app.colorMode)
		app.updateLogsView(true)
		return nil
	}); err != nil {
		return err
	}
	customEnd := getHotkey(config.Hotkeys.GoToEnd, "ctrl+e")
	if err := app.gui.SetKeybinding("", customEnd, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if !app.disableAutoScroll {
			app.autoScroll = true
		} else {
			app.autoScroll = false
		}
		vLog, err := app.gui.View("logs")
		if err != nil {
			return err
		}
		vLog.Subtitle = fmt.Sprintf("[tail: %s lines | auto-update: %t (%d sec) | docker: %s (%s) | color: %t]", app.logViewCount, app.autoScroll, app.logUpdateSeconds, app.dockerStreamLogsStr, app.dockerStreamMode, app.colorMode)
		app.updateLogsView(true)
		return nil
	}); err != nil {
		return err
	}

	// Home/Ctrl+A
	// Перемещение к началу журнала
	if err := app.gui.SetKeybinding("", gocui.KeyHome, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		app.pageUpLogs()
		return nil
	}); err != nil {
		return err
	}
	customHome := getHotkey(config.Hotkeys.GoToTop, "ctrl+a")
	if err := app.gui.SetKeybinding("", customHome, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		app.pageUpLogs()
		return nil
	}); err != nil {
		return err
	}

	// tail mode (Alt+Left/Right)
	// Переключение для количества строк вывода
	customTailMore := getHotkey(config.Hotkeys.TailModeMore, "ctrl+x")
	customTailLess := getHotkey(config.Hotkeys.TailModeLess, "ctrl+z")
	if err := app.gui.SetKeybinding("", customTailMore, gocui.ModNone, app.setCountLogViewUp); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("", customTailLess, gocui.ModNone, app.setCountLogViewDown); err != nil {
		return err
	}

	// update interval Shift+Left/Right
	// Увеличение фоновго интервала обновления журнала
	customUpdateIntervalMore := getHotkey(config.Hotkeys.UpdateIntervalMore, "ctrl+p")
	customUpdateIntervalLess := getHotkey(config.Hotkeys.UpdateIntervalLess, "ctrl+o")
	if err := app.gui.SetKeybinding("", customUpdateIntervalMore, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.logUpdateSeconds >= 2 && app.logUpdateSeconds <= 9 {
			app.logUpdateSeconds++
			v, err := app.gui.View("logs")
			if err != nil {
				return err
			}
			app.secondsChan <- app.logUpdateSeconds
			v.Subtitle = fmt.Sprintf("[tail: %s lines | auto-update: %t (%d sec) | docker: %s (%s) | color: %t]", app.logViewCount, app.autoScroll, app.logUpdateSeconds, app.dockerStreamLogsStr, app.dockerStreamMode, app.colorMode)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("", customUpdateIntervalLess, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.logUpdateSeconds >= 3 && app.logUpdateSeconds <= 10 {
			app.logUpdateSeconds--
			v, err := app.gui.View("logs")
			if err != nil {
				return err
			}
			// Изменяем интервал в горутине
			app.secondsChan <- app.logUpdateSeconds
			v.Subtitle = fmt.Sprintf("[tail: %s lines | auto-update: %t (%d sec) | docker: %s (%s) | color: %t]", app.logViewCount, app.autoScroll, app.logUpdateSeconds, app.dockerStreamLogsStr, app.dockerStreamMode, app.colorMode)
		}
		return nil
	}); err != nil {
		return err
	}

	// auto update (Ctrl+U)
	// Включение или отключение автоматического скроллинга
	customAutoUpdate := getHotkey(config.Hotkeys.AutoUpdateJournal, "ctrl+u")
	if err := app.gui.SetKeybinding("", customAutoUpdate, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.disableAutoScroll {
			app.disableAutoScroll = false
			app.autoScroll = false
		} else {
			app.disableAutoScroll = true
			app.autoScroll = false
		}
		vLog, err := app.gui.View("logs")
		if err != nil {
			return err
		}
		vLog.Subtitle = fmt.Sprintf("[tail: %s lines | auto-update: %t (%d sec) | docker: %s (%s) | color: %t]", app.logViewCount, app.autoScroll, app.logUpdateSeconds, app.dockerStreamLogsStr, app.dockerStreamMode, app.colorMode)
		app.updateLogOutput(false)
		return nil
	}); err != nil {
		return err
	}

	// update journal (Ctrl+R)
	// Ручное обновление текущего вывода журнала
	// Актуально в режиме выключенного автоматического обновления
	customUpdateJournal := getHotkey(config.Hotkeys.UpdateJournal, "ctrl+r")
	if err := app.gui.SetKeybinding("", customUpdateJournal, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		app.updateLogOutput(false)
		return nil
	}); err != nil {
		return err
	}

	// update lists (Ctrl+Q)
	// Обновить все текущие списки журналов вручную
	customUpdateLists := getHotkey(config.Hotkeys.UpdateLists, "ctrl+q")
	if err := app.gui.SetKeybinding("", customUpdateLists, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.getOS != "windows" {
			app.loadServices(app.selectUnits)
			app.loadFiles(app.selectPath)
		} else {
			app.loadWinFiles(app.selectPath)
		}
		app.loadDockerContainer(app.selectContainerizationSystem)
		return nil
	}); err != nil {
		return err
	}

	// color disable/enable (Ctrl+W)
	// Выключение/включение встроенной (custom built-in) покраски или через tailspin
	customColor := getHotkey(config.Hotkeys.ColorDisable, "ctrl+w")
	if err := app.gui.SetKeybinding("", customColor, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.colorMode {
			app.colorMode = false
		} else {
			app.colorMode = true
		}
		if len(app.currentLogLines) != 0 {
			app.updateLogsView(true)
			app.applyFilter(false)
			app.updateLogOutput(false)
		}
		vLog, err := app.gui.View("logs")
		if err != nil {
			return err
		}
		vLog.Subtitle = fmt.Sprintf("[tail: %s lines | auto-update: %t (%d sec) | docker: %s (%s) | color: %t]", app.logViewCount, app.autoScroll, app.logUpdateSeconds, app.dockerStreamLogsStr, app.dockerStreamMode, app.colorMode)
		return nil
	}); err != nil {
		return err
	}

	// tailspin (Ctrl+N)
	// Включение/выключение режима покраски через tailspin
	customTailspin := getHotkey(config.Hotkeys.TailspinEnable, "ctrl+n")
	if err := app.gui.SetKeybinding("", customTailspin, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.tailSpinMode {
			app.tailSpinMode = false
		} else {
			// Проверяем, что tailspin или tspin установлен в системе
			tsCommands := []string{"tailspin", "tspin"}
			for _, ts := range tsCommands {
				cmd := exec.Command(ts, "--version")
				_, err := cmd.Output()
				// Если не установлен, выводим интерфейс ошибки на 3 секунды
				if err != nil {
					if !app.testMode {
						go func() {
							text := "tailspin/tspin not found in environment"
							app.showInterfaceInfo(g, true, text)
							time.Sleep(3 * time.Second)
							app.closeInfo(g)
						}()
					}
				} else {
					app.tailSpinMode = true
					app.tailSpinBinName = ts
				}
			}
		}
		if len(app.currentLogLines) != 0 {
			app.updateLogsView(true)
			app.applyFilter(false)
			app.updateLogOutput(false)
		}
		return nil
	}); err != nil {
		return err
	}

	// docker log load mode from stream or file system (Ctrl+D)
	// Переключение режима чтения журналов Docker из потоков или файловой системы
	customDockerMode := getHotkey(config.Hotkeys.SwitchDockerMode, "ctrl+d")
	if err := app.gui.SetKeybinding("", customDockerMode, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.dockerStreamLogs {
			app.dockerStreamLogs = false
			app.dockerStreamLogsStr = "json"
		} else {
			app.dockerStreamLogs = true
			app.dockerStreamLogsStr = "stream"
		}
		app.updateLogOutput(false)
		return nil
	}); err != nil {
		return err
	}

	// docker stream (Ctrl+S)
	// Переключение режима вывода потоков журналов (фильтрация по потоку)
	customStreamMode := getHotkey(config.Hotkeys.SwitchStreamMode, "ctrl+s")
	if err := app.gui.SetKeybinding("", customStreamMode, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		switch {
		case app.dockerStreamMode == "all":
			app.dockerStreamMode = "stdout"
		case app.dockerStreamMode == "stdout":
			app.dockerStreamMode = "stderr"
		case app.dockerStreamMode == "stderr":
			app.dockerStreamMode = "all"
		}
		app.updateLogOutput(false)
		return nil
	}); err != nil {
		return err
	}

	// docker timestamp (Ctrl+T)
	// Переключение режима вывода timestamp и название потока
	customTimestamp := getHotkey(config.Hotkeys.TimestampShow, "ctrl+t")
	if err := app.gui.SetKeybinding("", customTimestamp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		switch {
		case app.timestampDocker && app.streamTypeDocker:
			app.streamTypeDocker = false
		case app.timestampDocker && !app.streamTypeDocker:
			app.timestampDocker = false
		case !app.timestampDocker && !app.streamTypeDocker:
			app.streamTypeDocker = true
		case !app.timestampDocker && app.streamTypeDocker:
			app.timestampDocker = true
		}
		app.updateLogOutput(false)
		return nil
	}); err != nil {
		return err
	}

	// Exit (ctrl+c)
	// Очистка поля ввода для фильтрации списков или выход
	customExit := getHotkey(config.Hotkeys.Exit, "ctrl+c")
	if err := app.gui.SetKeybinding("filterList", customExit, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.filterListText == "" {
			return quit(g, v)
		} else {
			app.clearFilterListEditor(g)
			return nil
		}
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("services", customExit, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.filterListText == "" {
			return quit(g, v)
		} else {
			app.clearFilterListEditor(g)
			return nil
		}
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", customExit, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.filterListText == "" {
			return quit(g, v)
		} else {
			app.clearFilterListEditor(g)
			return nil
		}
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", customExit, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.filterListText == "" {
			return quit(g, v)
		} else {
			app.clearFilterListEditor(g)
			return nil
		}
	}); err != nil {
		return err
	}
	// Очистка поля ввода для фильтрации логов или выход
	if err := app.gui.SetKeybinding("filter", customExit, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.filterText == "" {
			return quit(g, v)
		} else {
			app.clearFilterEditor(g)
			return nil
		}
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", customExit, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.filterText == "" {
			return quit(g, v)
		} else {
			app.clearFilterEditor(g)
			return nil
		}
	}); err != nil {
		return err
	}
	// Очистка поля ввода для фильтрации по времени
	if err := app.gui.SetKeybinding("sinceFilter", customExit, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.sinceFilterText == "" {
			return quit(g, v)
		} else {
			v.Clear()
			app.sinceFilterText = strings.TrimSpace(v.Buffer())
			v.FrameColor = gocui.ColorGreen
			app.sinceTimestampFilterMode = false
			return nil
		}
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("untilFilter", customExit, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if app.untilFilterText == "" {
			return quit(g, v)
		} else {
			v.Clear()
			app.untilFilterText = strings.TrimSpace(v.Buffer())
			v.FrameColor = gocui.ColorGreen
			app.untilTimestampFilterMode = false
			return nil
		}
	}); err != nil {
		return err
	}

	// Mouse control
	// Привязка клика мыши для выбора элемента в списке журналов и изменения фокуса на окно
	if err := app.gui.SetKeybinding("filterList", gocui.MouseLeft, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.setSelectView(g, "filterList")
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("services", gocui.MouseLeft, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		err := app.selectService(g, v)
		if err != nil {
			return err
		}
		return app.setSelectView(g, "services")
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", gocui.MouseLeft, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		err := app.selectFile(g, v)
		if err != nil {
			return err
		}
		return app.setSelectView(g, "varLogs")
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.MouseLeft, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		err := app.selectDocker(g, v)
		if err != nil {
			return err
		}
		return app.setSelectView(g, "docker")
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("filter", gocui.MouseLeft, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.setSelectView(g, "filter")
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("sinceFilter", gocui.MouseLeft, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.setSelectView(g, "sinceFilter")
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("untilFilter", gocui.MouseLeft, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.setSelectView(g, "untilFilter")
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", gocui.MouseLeft, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.setSelectView(g, "logs")
	}); err != nil {
		return err
	}

	// Скроллинг колесом мыши вверх/вниз на 1 элемент
	if err := app.gui.SetKeybinding("services", gocui.MouseWheelUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevService(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("services", gocui.MouseWheelDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextService(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", gocui.MouseWheelUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevFileName(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("varLogs", gocui.MouseWheelDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextFileName(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.MouseWheelUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.prevDockerContainer(v, 1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("docker", gocui.MouseWheelDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.nextDockerContainer(v, 1)
	}); err != nil {
		return err
	}
	// Скроллинг по журналу через 1 или 100 (alt/ctrl) строк
	if err := app.gui.SetKeybinding("logs", gocui.MouseWheelUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollUpLogs(1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", gocui.MouseWheelUp, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollUpLogs(100)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", gocui.MouseWheelUp, gocui.ModMouseCtrl, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollUpLogs(100)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", gocui.MouseWheelDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollDownLogs(1)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", gocui.MouseWheelDown, gocui.ModAlt, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollDownLogs(100)
	}); err != nil {
		return err
	}
	if err := app.gui.SetKeybinding("logs", gocui.MouseWheelDown, gocui.ModMouseCtrl, func(g *gocui.Gui, v *gocui.View) error {
		return app.scrollDownLogs(100)
	}); err != nil {
		return err
	}

	return nil
}

// Интерфейс справки
func (app *App) showInterfaceHelp(g *gocui.Gui) {
	// Получаем размеры терминала
	maxX, maxY := g.Size()
	// Размеры окна help
	width, height := 108, 54
	// Вычисляем координаты для центрального расположения
	x0 := (maxX - width) / 2
	y0 := (maxY - height) / 2
	x1 := x0 + width
	y1 := y0 + height
	helpView, err := g.SetView("help", x0, y0, x1, y1, 0)
	if err != nil && !errors.Is(err, gocui.ErrUnknownView) {
		return
	}
	helpView.Title = " Help "
	helpView.Autoscroll = true
	helpView.Wrap = true
	helpView.FrameColor = gocui.ColorGreen
	helpView.TitleColor = gocui.ColorGreen
	helpView.Clear()
	fmt.Fprintln(helpView, "\n                   \033[32m_                              \033[36m_                                    _ ")
	fmt.Fprintln(helpView, "                  \033[32m| |                            \033[36m| |                                  | |")
	fmt.Fprintln(helpView, "                  \033[32m| |      __ _  ____ _   _      \033[36m| |  ___   _   _  _ __  _ __    __ _ | |")
	fmt.Fprintln(helpView, "                  \033[32m| |     / _` ||_  /| | | | \033[36m_   | | / _ \\ | | | || '__|| '_ \\  / _` || |")
	fmt.Fprintln(helpView, "                  \033[32m| |____| (_| | / / | |_| |\033[36m| |__| || (_) || |_| || |   | | | || (_| || |")
	fmt.Fprintln(helpView, "                  \033[32m|______|\\__,_|/___| \\__, | \033[36m\\____/  \\___/  \\__,_||_|   |_| |_| \\__,_||_|")
	fmt.Fprintln(helpView, "                  \033[32m					 __/ |                                             ")
	fmt.Fprintln(helpView, "                  \033[32m                    |___/\033[0m")
	fmt.Fprintln(helpView, "\n    Version: "+app.wordColor(programVersion))
	fmt.Fprintln(helpView, "\n    Hotkeys description (default values):")
	fmt.Fprintln(helpView, "\n      \033[32mUp\033[0m/\033[32mPgUp\033[0m/\033[32mk\033[0m and \033[32mDown\033[0m/\033[32mPgDown\033[0m/\033[32mj\033[0m - move up and down through all journal lists and log output,")
	fmt.Fprintln(helpView, "      as well as changing the filtering mode in the filter window.")
	fmt.Fprintln(helpView, "      \033[32mShift\033[0m/\033[32mAlt\033[0m+\033[32mUp\033[0m/\033[32mDown\033[0m - quickly move up and down through all journal lists and log output")
	fmt.Fprintln(helpView, "      every 10 or 100 lines (500 for log output).")
	fmt.Fprintln(helpView, "      \033[32mShift\033[0m/\033[32mCtrl\033[0m+\033[32mk\033[0m/\033[32mj\033[0m - quickly move up and down (like Vim and alternative for macOS from config).")
	fmt.Fprintln(helpView, "      \033[32mLeft\033[0m/\033[32m[\033[0m/\033[32mh\033[0m and \033[32mRight\033[0m/\033[32m]\033[0m/\033[32ml\033[0m - switch between journal lists in the selected window.")
	fmt.Fprintln(helpView, "      \033[32mTab\033[0m - switch to next window.")
	fmt.Fprintln(helpView, "      \033[32mShift\033[0m+\033[32mTab\033[0m - return to previous window.")
	fmt.Fprintln(helpView, "      \033[32mEnter\033[0m - load a log from the list window or return to the previous window from the filter window.")
	fmt.Fprintln(helpView, "      \033[32m/\033[0m - go to the filter window from the current list window or logs window.")
	fmt.Fprintln(helpView, "      \033[32mEnd\033[0m/\033[32mCtrl\033[0m+\033[32mE\033[0m - go to the end of the log.")
	fmt.Fprintln(helpView, "      \033[32mHome\033[0m/\033[32mCtrl\033[0m+\033[32mA\033[0m - go to the top of the log.")
	fmt.Fprintln(helpView, "      \033[32mCtrl\033[0m+\033[32mX\033[0m/\033[32mZ\033[0m - change the number of log lines to output (default: 50000, range: 200-200000).")
	fmt.Fprintln(helpView, "      \033[32mCtrl\033[0m+\033[32mP\033[0m/\033[32mO\033[0m - change the auto refresh interval of the log output (default: 5, range: 2-10).")
	fmt.Fprintln(helpView, "      \033[32mCtrl\033[0m+\033[32mU\033[0m - disable streaming of new events (log is loaded once without automatic update).")
	fmt.Fprintln(helpView, "      \033[32mCtrl\033[0m+\033[32mR\033[0m - update the current log output manually (relevant in disable streaming mode).")
	fmt.Fprintln(helpView, "      \033[32mCtrl\033[0m+\033[32mQ\033[0m - update all log lists.")
	fmt.Fprintln(helpView, "      \033[32mCtrl\033[0m+\033[32mW\033[0m - enable or disable ANSI coloring for output.")
	fmt.Fprintln(helpView, "      \033[32mCtrl\033[0m+\033[32mN\033[0m - enable or disable coloring via tailspin.")
	fmt.Fprintln(helpView, "      \033[32mCtrl\033[0m+\033[32mD\033[0m - change read mode for docker logs (stream only or json from file system).")
	fmt.Fprintln(helpView, "      \033[32mCtrl\033[0m+\033[32mS\033[0m - change stream display mode for docker logs (all, stdout or stderr only).")
	fmt.Fprintln(helpView, "      \033[32mCtrl\033[0m+\033[32mT\033[0m - enable or disable built-in timestamp and stream type for docker logs.")
	fmt.Fprintln(helpView, "      \033[32mCtrl\033[0m+\033[32mC\033[0m - clear input text in the filter window or exit.")
	fmt.Fprintln(helpView, "\n    Supported formats for filtering by timestamp:")
	fmt.Fprintln(helpView, "\n      "+app.wordColor("00:00"))
	fmt.Fprintln(helpView, "      "+app.wordColor("00:00:00"))
	fmt.Fprintln(helpView, "      "+app.wordColor("2025-04-14"))
	fmt.Fprintln(helpView, "      "+app.wordColor("2025-04-14 00:00"))
	fmt.Fprintln(helpView, "      "+app.wordColor("2025-04-14 00:00:00"))
	fmt.Fprintln(helpView, "\n    Examples of short format:")
	fmt.Fprintln(helpView, "\n      Since \033[35m-\033[34m48h\033[0m until \033[35m-\033[34m24h\033[0m for container logs from journald (logs for the previous day).")
	fmt.Fprintln(helpView, "      Since \033[35m+\033[34m1h\033[0m until \033[35m+\033[34m30m\033[0m for system journals from docker or podman.")
	fmt.Fprintln(helpView, "\n    Source code: "+app.wordColor("https://github.com/Lifailon/lazyjournal"))
}

func (app *App) closeHelp(g *gocui.Gui) {
	if err := g.DeleteView("help"); err != nil {
		return
	}
}

// Интерфейс ошибки
func (app *App) showInterfaceInfo(g *gocui.Gui, errInfo bool, text string) {
	maxX, maxY := g.Size()
	width, height := 50, 3
	x0 := (maxX - width) - 5
	y0 := (maxY - height) - 2
	x1 := x0 + width
	y1 := y0 + height
	helpView, err := g.SetView("info", x0, y0, x1, y1, 0)
	if err != nil && !errors.Is(err, gocui.ErrUnknownView) {
		return
	}
	if errInfo {
		helpView.Title = " Error "
		helpView.FrameColor = gocui.ColorRed
		helpView.TitleColor = gocui.ColorRed
	} else {
		helpView.Title = " Info "
		helpView.FrameColor = gocui.ColorGreen
		helpView.TitleColor = gocui.ColorGreen
	}
	helpView.Wrap = true
	helpView.Clear()
	fmt.Fprintln(helpView, text)
}

func (app *App) closeInfo(g *gocui.Gui) {
	if err := g.DeleteView("info"); err != nil {
		return
	}
}

// Функции для переключения количества строк для вывода логов

func (app *App) setCountLogViewUp(g *gocui.Gui, v *gocui.View) error {
	switch app.logViewCount {
	case "200":
		app.logViewCount = "500"
	case "500":
		app.logViewCount = "1000"
	case "1000":
		app.logViewCount = "5000"
	case "5000":
		app.logViewCount = "10000"
	case "10000":
		app.logViewCount = "20000"
	case "20000":
		app.logViewCount = "30000"
	case "30000":
		app.logViewCount = "50000"
	case "50000":
		app.logViewCount = "100000"
	case "100000":
		app.logViewCount = "150000"
	case "150000":
		app.logViewCount = "200000"
	case "200000":
		app.logViewCount = "200000"
	}
	// Загружаем журнал заново
	app.updateLogOutput(true)
	// Обновляем статус
	vLog, err := app.gui.View("logs")
	if err != nil {
		return err
	}
	vLog.Subtitle = fmt.Sprintf("[tail: %s lines | auto-update: %t (%d sec) | docker: %s (%s) | color: %t]", app.logViewCount, app.autoScroll, app.logUpdateSeconds, app.dockerStreamLogsStr, app.dockerStreamMode, app.colorMode)
	return nil
}

func (app *App) setCountLogViewDown(g *gocui.Gui, v *gocui.View) error {
	switch app.logViewCount {
	case "200000":
		app.logViewCount = "150000"
	case "150000":
		app.logViewCount = "100000"
	case "100000":
		app.logViewCount = "50000"
	case "50000":
		app.logViewCount = "30000"
	case "30000":
		app.logViewCount = "20000"
	case "20000":
		app.logViewCount = "10000"
	case "10000":
		app.logViewCount = "5000"
	case "5000":
		app.logViewCount = "1000"
	case "1000":
		app.logViewCount = "500"
	case "500":
		app.logViewCount = "200"
	case "200":
		app.logViewCount = "200"
	}
	app.updateLogOutput(true)
	vLog, err := app.gui.View("logs")
	if err != nil {
		return err
	}
	vLog.Subtitle = fmt.Sprintf("[tail: %s lines | auto-update: %t (%d sec) | docker: %s (%s) | color: %t]", app.logViewCount, app.autoScroll, app.logUpdateSeconds, app.dockerStreamLogsStr, app.dockerStreamMode, app.colorMode)
	return nil
}

// Функция для переключения режима фильтрации (вверх)
func (app *App) setFilterModeRight(g *gocui.Gui, v *gocui.View) error {
	selectedFilter, err := g.View("filter")
	if err != nil {
		log.Panicln(err)
	}
	switch selectedFilter.Title {
	case "Filter (Default)":
		selectedFilter.Title = "Filter (Fuzzy)"
		app.selectFilterMode = "fuzzy"
	case "Filter (Fuzzy)":
		selectedFilter.Title = "Filter (Regex)"
		app.selectFilterMode = "regex"
	case "Filter (Regex)":
		// Фиксируем название
		selectedFilter.Title = "Filter (Timestamp)"
		app.selectFilterMode = "timestamp"
		// Создаем два новых окна
		maxX, _ := g.Size()
		leftPanelWidth := maxX / 4
		filterWidth := (maxX - leftPanelWidth - 1) / 2
		if v, err := g.SetView("sinceFilter", leftPanelWidth+1, 0, leftPanelWidth+1+filterWidth, 2, 0); err != nil {
			v.Title = "Since timestamp"
			v.Editable = true
			v.Wrap = true
			// Обработка времени и даты
			v.Editor = app.timestampFilterEditor("sinceFilter")
			// Изменить цвет окна
			v.FrameColor = gocui.ColorGreen
			v.TitleColor = gocui.ColorGreen
			// Выбираем новое окно
			if _, err := g.SetCurrentView("sinceFilter"); err != nil {
				return nil
			}
			// Возобновляет текст из переменной
			fmt.Fprint(v, app.sinceFilterText)
			// Корректируем позицию курсора
			if err = v.SetCursor(len(app.sinceFilterText), 0); err != nil {
				return nil
			}
		}
		if v2, err := g.SetView("untilFilter", leftPanelWidth+1+filterWidth+1, 0, maxX-1, 2, 0); err != nil {
			v2.Title = "Until timestamp"
			v2.Editable = true
			v2.Wrap = true
			v2.Editor = app.timestampFilterEditor("untilFilter")
			fmt.Fprint(v2, app.untilFilterText)
			if err = v2.SetCursor(len(app.untilFilterText), 0); err != nil {
				return nil
			}
		}
	case "Filter (Timestamp)":
		// Удаляем временные два окна
		if err = g.DeleteView("sinceFilter"); err != nil {
			return nil
		}
		if err = g.DeleteView("untilFilter"); err != nil {
			return nil
		}
		// Возвращяем фокус и цвет назад
		if _, err := g.SetCurrentView("filter"); err != nil {
			return nil
		}
		v.FrameColor = gocui.ColorGreen
		v.TitleColor = gocui.ColorGreen
		selectedFilter.Title = "Filter (Default)"
		app.selectFilterMode = "default"
	}
	if app.selectFilterMode == "timestamp" {
	} else {
		app.applyFilter(false)
	}
	return nil
}

// Функция для переключения режима фильтрации (вниз)
func (app *App) setFilterModeLeft(g *gocui.Gui, v *gocui.View) error {
	selectedFilter, err := g.View("filter")
	if err != nil {
		log.Panicln(err)
	}
	switch selectedFilter.Title {
	case "Filter (Default)":
		selectedFilter.Title = "Filter (Timestamp)"
		app.selectFilterMode = "timestamp"
		maxX, _ := g.Size()
		leftPanelWidth := maxX / 4
		filterWidth := (maxX - leftPanelWidth - 1) / 2
		if v, err := g.SetView("sinceFilter", leftPanelWidth+1, 0, leftPanelWidth+1+filterWidth, 2, 0); err != nil {
			v.Title = "Since timestamp"
			v.Editable = true
			v.Wrap = true
			v.Editor = app.timestampFilterEditor("sinceFilter")
			v.FrameColor = gocui.ColorGreen
			v.TitleColor = gocui.ColorGreen
			if _, err := g.SetCurrentView("sinceFilter"); err != nil {
				return nil
			}
			fmt.Fprint(v, app.sinceFilterText)
			if err = v.SetCursor(len(app.sinceFilterText), 0); err != nil {
				return nil
			}
		}
		if v2, err := g.SetView("untilFilter", leftPanelWidth+1+filterWidth+1, 0, maxX-1, 2, 0); err != nil {
			v2.Title = "Until timestamp"
			v2.Editable = true
			v2.Wrap = true
			v2.Editor = app.timestampFilterEditor("untilFilter")
			fmt.Fprint(v2, app.untilFilterText)
			if err = v2.SetCursor(len(app.untilFilterText), 0); err != nil {
				return nil
			}
		}
	case "Filter (Timestamp)":
		if err = g.DeleteView("sinceFilter"); err != nil {
			return nil
		}
		if err = g.DeleteView("untilFilter"); err != nil {
			return nil
		}
		if _, err := g.SetCurrentView("filter"); err != nil {
			return nil
		}
		v.FrameColor = gocui.ColorGreen
		v.TitleColor = gocui.ColorGreen
		selectedFilter.Title = "Filter (Regex)"
		app.selectFilterMode = "regex"
	case "Filter (Regex)":
		selectedFilter.Title = "Filter (Fuzzy)"
		app.selectFilterMode = "fuzzy"
	case "Filter (Fuzzy)":
		selectedFilter.Title = "Filter (Default)"
		app.selectFilterMode = "default"
	}
	return nil
}

// Функции для переключения выбора журналов из journalctl

func (app *App) setUnitListRight(g *gocui.Gui, v *gocui.View) error {
	selectedServices, err := g.View("services")
	if err != nil {
		log.Panicln(err)
	}
	// Сбрасываем содержимое массива и положение курсора
	app.journals = app.journals[:0]
	app.startServices = 0
	app.selectedJournal = 0
	// Меняем журнал и обновляем список
	switch app.selectUnits {
	case "services":
		app.selectUnits = "UNIT"
		selectedServices.Title = " < System journals (0) > "
		app.loadServices(app.selectUnits)
	case "UNIT":
		app.selectUnits = "USER_UNIT"
		selectedServices.Title = " < User journals (0) > "
		app.loadServices(app.selectUnits)
	case "USER_UNIT":
		app.selectUnits = "kernel"
		selectedServices.Title = " < Kernel boot (0) > "
		app.loadServices(app.selectUnits)
	case "kernel":
		app.selectUnits = "auditd"
		selectedServices.Title = " < Audit rules keys (0) > "
		app.loadServices(app.selectUnits)
	case "auditd":
		app.selectUnits = "services"
		selectedServices.Title = " < Unit list (0) > "
		app.loadServices(app.selectUnits)
	}
	return nil
}

func (app *App) setUnitListLeft(g *gocui.Gui, v *gocui.View) error {
	selectedServices, err := g.View("services")
	if err != nil {
		log.Panicln(err)
	}
	app.journals = app.journals[:0]
	app.startServices = 0
	app.selectedJournal = 0
	switch app.selectUnits {
	case "services":
		app.selectUnits = "auditd"
		selectedServices.Title = " < Audit rules keys (0) > "
		app.loadServices(app.selectUnits)
	case "auditd":
		app.selectUnits = "kernel"
		selectedServices.Title = " < Kernel boot (0) > "
		app.loadServices(app.selectUnits)
	case "kernel":
		app.selectUnits = "USER_UNIT"
		selectedServices.Title = " < User journals (0) > "
		app.loadServices(app.selectUnits)
	case "USER_UNIT":
		app.selectUnits = "UNIT"
		selectedServices.Title = " < System journals (0) > "
		app.loadServices(app.selectUnits)
	case "UNIT":
		app.selectUnits = "services"
		selectedServices.Title = " < Unit list (0) > "
		app.loadServices(app.selectUnits)
	}
	return nil
}

// Функция для переключения выбора журналов файловой системы
func (app *App) setLogFilesListRight(g *gocui.Gui, v *gocui.View) error {
	selectedVarLog, err := g.View("varLogs")
	if err != nil {
		log.Panicln(err)
	}
	// Добавляем сообщение о загрузке журнала
	g.Update(func(g *gocui.Gui) error {
		selectedVarLog.Clear()
		fmt.Fprintln(selectedVarLog, "Searching log files...")
		selectedVarLog.Highlight = false
		return nil
	})
	// Отключаем переключение списков
	app.keybindingsEnabled = false
	if err := app.setupKeybindings(); err != nil {
		log.Panicln("Error key bindings", err)
	}
	// Полсекундная задержка, для корректного обновления интерфейса после выполнения функции
	time.Sleep(500 * time.Millisecond)
	app.logfiles = app.logfiles[:0]
	app.startFiles = 0
	app.selectedFile = 0
	// Запускаем функцию загрузки журнала в горутине
	if app.getOS == "windows" {
		go func() {
			switch app.selectPath {
			case "ProgramFiles":
				app.selectPath = "ProgramFiles86"
				selectedVarLog.Title = " < Program Files x86 (0) > "
				app.loadWinFiles(app.selectPath)
			case "ProgramFiles86":
				app.selectPath = "ProgramData"
				selectedVarLog.Title = " < ProgramData (0) > "
				app.loadWinFiles(app.selectPath)
			case "ProgramData":
				app.selectPath = "AppDataLocal"
				selectedVarLog.Title = " < AppData Local (0) > "
				app.loadWinFiles(app.selectPath)
			case "AppDataLocal":
				app.selectPath = "AppDataRoaming"
				selectedVarLog.Title = " < AppData Roaming (0) > "
				app.loadWinFiles(app.selectPath)
			case "AppDataRoaming":
				app.selectPath = "ProgramFiles"
				selectedVarLog.Title = " < Program Files (0) > "
				app.loadWinFiles(app.selectPath)
			}
			// Включаем переключение списков
			app.keybindingsEnabled = true
			if err := app.setupKeybindings(); err != nil {
				log.Panicln("Error key bindings", err)
			}
		}()
	} else {
		go func() {
			switch app.selectPath {
			case "/var/log/":
				app.selectPath = "/opt/"
				selectedVarLog.Title = " < Optional package logs (0) > "
				app.loadFiles(app.selectPath)
			case "/opt/":
				app.selectPath = "/home/"
				selectedVarLog.Title = " < Users home logs (0) > "
				app.loadFiles(app.selectPath)
			case "/home/":
				app.selectPath = "descriptor"
				selectedVarLog.Title = " < Process descriptor logs (0) > "
				app.loadFiles(app.selectPath)
			case "descriptor":
				app.selectPath = "/var/log/"
				selectedVarLog.Title = " < System var logs (0) > "
				app.loadFiles(app.selectPath)
			}
			// Включаем переключение списков
			app.keybindingsEnabled = true
			if err := app.setupKeybindings(); err != nil {
				log.Panicln("Error key bindings", err)
			}
		}()
	}
	return nil
}

func (app *App) setLogFilesListLeft(g *gocui.Gui, v *gocui.View) error {
	selectedVarLog, err := g.View("varLogs")
	if err != nil {
		log.Panicln(err)
	}
	g.Update(func(g *gocui.Gui) error {
		selectedVarLog.Clear()
		fmt.Fprintln(selectedVarLog, "Searching log files...")
		selectedVarLog.Highlight = false
		return nil
	})
	app.keybindingsEnabled = false
	if err := app.setupKeybindings(); err != nil {
		log.Panicln("Error key bindings", err)
	}
	time.Sleep(500 * time.Millisecond)
	app.logfiles = app.logfiles[:0]
	app.startFiles = 0
	app.selectedFile = 0
	if app.getOS == "windows" {
		go func() {
			switch app.selectPath {
			case "ProgramFiles":
				app.selectPath = "AppDataRoaming"
				selectedVarLog.Title = " < AppData Roaming (0) > "
				app.loadWinFiles(app.selectPath)
			case "AppDataRoaming":
				app.selectPath = "AppDataLocal"
				selectedVarLog.Title = " < AppData Local (0) > "
				app.loadWinFiles(app.selectPath)
			case "AppDataLocal":
				app.selectPath = "ProgramData"
				selectedVarLog.Title = " < ProgramData (0) > "
				app.loadWinFiles(app.selectPath)
			case "ProgramData":
				app.selectPath = "ProgramFiles86"
				selectedVarLog.Title = " < Program Files x86 (0) > "
				app.loadWinFiles(app.selectPath)
			case "ProgramFiles86":
				app.selectPath = "ProgramFiles"
				selectedVarLog.Title = " < Program Files (0) > "
				app.loadWinFiles(app.selectPath)
			}
			app.keybindingsEnabled = true
			if err := app.setupKeybindings(); err != nil {
				log.Panicln("Error key bindings", err)
			}
		}()
	} else {
		go func() {
			switch app.selectPath {
			case "/var/log/":
				app.selectPath = "descriptor"
				selectedVarLog.Title = " < Process descriptor logs (0) > "
				app.loadFiles(app.selectPath)
			case "descriptor":
				app.selectPath = "/home/"
				selectedVarLog.Title = " < Users home logs (0) > "
				app.loadFiles(app.selectPath)
			case "/home/":
				app.selectPath = "/opt/"
				selectedVarLog.Title = " < Optional package logs (0) > "
				app.loadFiles(app.selectPath)
			case "/opt/":
				app.selectPath = "/var/log/"
				selectedVarLog.Title = " < System var logs (0) > "
				app.loadFiles(app.selectPath)
			}
			app.keybindingsEnabled = true
			if err := app.setupKeybindings(); err != nil {
				log.Panicln("Error key bindings", err)
			}
		}()
	}
	return nil
}

// Функция для переключения списков системы контейнеризации
func (app *App) setContainersListRight(g *gocui.Gui, v *gocui.View) error {
	selectedDocker, err := g.View("docker")
	if err != nil {
		log.Panicln(err)
	}
	app.dockerContainers = app.dockerContainers[:0]
	app.startDockerContainers = 0
	app.selectedDockerContainer = 0
	switch app.selectContainerizationSystem {
	case "docker":
		app.selectContainerizationSystem = "compose"
		selectedDocker.Title = " < Compose stacks (0) > "
		app.loadDockerContainer(app.selectContainerizationSystem)
	case "compose":
		app.selectContainerizationSystem = "podman"
		selectedDocker.Title = " < Podman containers (0) > "
		app.loadDockerContainer(app.selectContainerizationSystem)
	case "podman":
		app.selectContainerizationSystem = "kubectl"
		selectedDocker.Title = " < Kubernetes pods (0) > "
		app.loadDockerContainer(app.selectContainerizationSystem)
	case "kubectl":
		app.selectContainerizationSystem = "docker"
		selectedDocker.Title = " < Docker containers (0) > "
		app.loadDockerContainer(app.selectContainerizationSystem)
	}
	return nil
}

func (app *App) setContainersListLeft(g *gocui.Gui, v *gocui.View) error {
	selectedDocker, err := g.View("docker")
	if err != nil {
		log.Panicln(err)
	}
	app.dockerContainers = app.dockerContainers[:0]
	app.startDockerContainers = 0
	app.selectedDockerContainer = 0
	switch app.selectContainerizationSystem {
	case "docker":
		app.selectContainerizationSystem = "kubectl"
		selectedDocker.Title = " < Kubernetes pods (0) > "
		app.loadDockerContainer(app.selectContainerizationSystem)
	case "kubectl":
		app.selectContainerizationSystem = "podman"
		selectedDocker.Title = " < Podman containers (0) > "
		app.loadDockerContainer(app.selectContainerizationSystem)
	case "podman":
		app.selectContainerizationSystem = "compose"
		selectedDocker.Title = " < Compose stacks (0) > "
		app.loadDockerContainer(app.selectContainerizationSystem)
	case "compose":
		app.selectContainerizationSystem = "docker"
		selectedDocker.Title = " < Docker containers (0) > "
		app.loadDockerContainer(app.selectContainerizationSystem)
	}
	return nil
}

// Функция для переключения окон через Tab
func (app *App) nextView(g *gocui.Gui, v *gocui.View) error {
	selectedFilterList, err := g.View("filterList")
	if err != nil {
		log.Panicln(err)
	}
	selectedServices, err := g.View("services")
	if err != nil {
		log.Panicln(err)
	}
	selectedVarLog, err := g.View("varLogs")
	if err != nil {
		log.Panicln(err)
	}
	selectedDocker, err := g.View("docker")
	if err != nil {
		log.Panicln(err)
	}
	selectedFilter, err := g.View("filter")
	if err != nil {
		log.Panicln(err)
	}
	sinceFilter, err := g.View("sinceFilter")
	if err != nil {
		app.timestampFilterView = false
	} else {
		app.timestampFilterView = true
	}
	untilFilter, err := g.View("untilFilter")
	if err != nil {
		app.timestampFilterView = false
	} else {
		app.timestampFilterView = true
	}
	selectedLogs, err := g.View("logs")
	if err != nil {
		log.Panicln(err)
	}
	selectedScrollLogs, err := g.View("scrollLogs")
	if err != nil {
		log.Panicln(err)
	}
	currentView := g.CurrentView()
	var nextView string
	// Начальное окно
	if currentView == nil {
		nextView = "services"
	} else {
		switch {
		case currentView.Name() == "filterList":
			nextView = "services"
			selectedFilterList.FrameColor = gocui.ColorDefault
			selectedFilterList.TitleColor = gocui.ColorDefault
			selectedServices.FrameColor = gocui.ColorGreen
			selectedServices.TitleColor = gocui.ColorGreen
			selectedVarLog.FrameColor = app.fileSystemFrameColor
			selectedVarLog.TitleColor = gocui.ColorDefault
			selectedDocker.FrameColor = app.dockerFrameColor
			selectedDocker.TitleColor = gocui.ColorDefault
			selectedFilter.FrameColor = gocui.ColorDefault
			selectedFilter.TitleColor = gocui.ColorDefault
			selectedLogs.FrameColor = gocui.ColorDefault
			selectedScrollLogs.FrameColor = gocui.ColorDefault
		case currentView.Name() == "services":
			nextView = "varLogs"
			selectedFilterList.FrameColor = gocui.ColorDefault
			selectedFilterList.TitleColor = gocui.ColorDefault
			selectedServices.FrameColor = app.journalListFrameColor
			selectedServices.TitleColor = gocui.ColorDefault
			selectedVarLog.FrameColor = gocui.ColorGreen
			selectedVarLog.TitleColor = gocui.ColorGreen
			selectedDocker.FrameColor = app.dockerFrameColor
			selectedDocker.TitleColor = gocui.ColorDefault
			selectedFilter.FrameColor = gocui.ColorDefault
			selectedFilter.TitleColor = gocui.ColorDefault
			selectedLogs.FrameColor = gocui.ColorDefault
			selectedScrollLogs.FrameColor = gocui.ColorDefault
		case currentView.Name() == "varLogs":
			nextView = "docker"
			selectedFilterList.FrameColor = gocui.ColorDefault
			selectedFilterList.TitleColor = gocui.ColorDefault
			selectedServices.FrameColor = app.journalListFrameColor
			selectedServices.TitleColor = gocui.ColorDefault
			selectedVarLog.FrameColor = app.fileSystemFrameColor
			selectedVarLog.TitleColor = gocui.ColorDefault
			selectedDocker.FrameColor = gocui.ColorGreen
			selectedDocker.TitleColor = gocui.ColorGreen
			selectedFilter.FrameColor = gocui.ColorDefault
			selectedFilter.TitleColor = gocui.ColorDefault
			selectedLogs.FrameColor = gocui.ColorDefault
			selectedScrollLogs.FrameColor = gocui.ColorDefault
		case currentView.Name() == "docker":
			if app.timestampFilterView {
				nextView = "sinceFilter"
				selectedFilterList.FrameColor = gocui.ColorDefault
				selectedFilterList.TitleColor = gocui.ColorDefault
				selectedServices.FrameColor = app.journalListFrameColor
				selectedServices.TitleColor = gocui.ColorDefault
				selectedVarLog.FrameColor = app.fileSystemFrameColor
				selectedVarLog.TitleColor = gocui.ColorDefault
				selectedDocker.FrameColor = app.dockerFrameColor
				selectedDocker.TitleColor = gocui.ColorDefault
				sinceFilter.FrameColor = gocui.ColorGreen // new
				sinceFilter.TitleColor = gocui.ColorGreen // new
				selectedLogs.FrameColor = gocui.ColorDefault
				selectedScrollLogs.FrameColor = gocui.ColorDefault
			} else {
				nextView = "filter"
				selectedFilterList.FrameColor = gocui.ColorDefault
				selectedFilterList.TitleColor = gocui.ColorDefault
				selectedServices.FrameColor = app.journalListFrameColor
				selectedServices.TitleColor = gocui.ColorDefault
				selectedVarLog.FrameColor = app.fileSystemFrameColor
				selectedVarLog.TitleColor = gocui.ColorDefault
				selectedDocker.FrameColor = app.dockerFrameColor
				selectedDocker.TitleColor = gocui.ColorDefault
				selectedFilter.FrameColor = gocui.ColorGreen
				selectedFilter.TitleColor = gocui.ColorGreen
				selectedLogs.FrameColor = gocui.ColorDefault
				selectedScrollLogs.FrameColor = gocui.ColorDefault
			}
		case currentView.Name() == "sinceFilter":
			nextView = "untilFilter"
			selectedFilterList.FrameColor = gocui.ColorDefault
			selectedFilterList.TitleColor = gocui.ColorDefault
			selectedServices.FrameColor = app.journalListFrameColor
			selectedServices.TitleColor = gocui.ColorDefault
			selectedVarLog.FrameColor = app.fileSystemFrameColor
			selectedVarLog.TitleColor = gocui.ColorDefault
			selectedDocker.FrameColor = app.dockerFrameColor
			selectedDocker.TitleColor = gocui.ColorDefault
			sinceFilter.FrameColor = gocui.ColorDefault // new
			sinceFilter.TitleColor = gocui.ColorDefault // new
			untilFilter.FrameColor = gocui.ColorGreen   // new
			untilFilter.TitleColor = gocui.ColorGreen   // new
			selectedLogs.FrameColor = gocui.ColorDefault
			selectedScrollLogs.FrameColor = gocui.ColorDefault
		case currentView.Name() == "filter" || currentView.Name() == "untilFilter":
			if app.timestampFilterView {
				untilFilter.FrameColor = gocui.ColorDefault // new
				untilFilter.TitleColor = gocui.ColorDefault // new
			}
			nextView = "logs"
			selectedFilterList.FrameColor = gocui.ColorDefault
			selectedFilterList.TitleColor = gocui.ColorDefault
			selectedServices.FrameColor = app.journalListFrameColor
			selectedServices.TitleColor = gocui.ColorDefault
			selectedVarLog.FrameColor = app.fileSystemFrameColor
			selectedVarLog.TitleColor = gocui.ColorDefault
			selectedDocker.FrameColor = app.dockerFrameColor
			selectedDocker.TitleColor = gocui.ColorDefault
			selectedFilter.FrameColor = gocui.ColorDefault
			selectedFilter.TitleColor = gocui.ColorDefault
			selectedLogs.FrameColor = gocui.ColorGreen
			selectedScrollLogs.FrameColor = gocui.ColorGreen
		case currentView.Name() == "logs":
			nextView = "filterList"
			selectedFilterList.FrameColor = gocui.ColorGreen
			selectedFilterList.TitleColor = gocui.ColorGreen
			selectedServices.FrameColor = app.journalListFrameColor
			selectedServices.TitleColor = gocui.ColorDefault
			selectedVarLog.FrameColor = app.fileSystemFrameColor
			selectedVarLog.TitleColor = gocui.ColorDefault
			selectedDocker.FrameColor = app.dockerFrameColor
			selectedDocker.TitleColor = gocui.ColorDefault
			selectedFilter.FrameColor = gocui.ColorDefault
			selectedFilter.TitleColor = gocui.ColorDefault
			selectedLogs.FrameColor = gocui.ColorDefault
			selectedScrollLogs.FrameColor = gocui.ColorDefault
		}
	}
	// Устанавливаем новое активное окно
	if _, err := g.SetCurrentView(nextView); err != nil {
		return err
	}
	return nil
}

// Функция для переключения окон в обратном порядке через Shift+Tab
func (app *App) backView(g *gocui.Gui, v *gocui.View) error {
	selectedFilterList, err := g.View("filterList")
	if err != nil {
		log.Panicln(err)
	}
	selectedServices, err := g.View("services")
	if err != nil {
		log.Panicln(err)
	}
	selectedVarLog, err := g.View("varLogs")
	if err != nil {
		log.Panicln(err)
	}
	selectedDocker, err := g.View("docker")
	if err != nil {
		log.Panicln(err)
	}
	selectedFilter, err := g.View("filter")
	if err != nil {
		log.Panicln(err)
	}
	sinceFilter, err := g.View("sinceFilter")
	if err != nil {
		app.timestampFilterView = false
	} else {
		app.timestampFilterView = true
	}
	untilFilter, err := g.View("untilFilter")
	if err != nil {
		app.timestampFilterView = false
	} else {
		app.timestampFilterView = true
	}
	selectedLogs, err := g.View("logs")
	if err != nil {
		log.Panicln(err)
	}
	selectedScrollLogs, err := g.View("scrollLogs")
	if err != nil {
		log.Panicln(err)
	}
	currentView := g.CurrentView()
	var nextView string
	if currentView == nil {
		nextView = "services"
	} else {
		switch {
		case currentView.Name() == "filterList":
			nextView = "logs"
			selectedFilterList.FrameColor = gocui.ColorDefault
			selectedFilterList.TitleColor = gocui.ColorDefault
			selectedServices.FrameColor = app.journalListFrameColor
			selectedServices.TitleColor = gocui.ColorDefault
			selectedVarLog.FrameColor = app.fileSystemFrameColor
			selectedVarLog.TitleColor = gocui.ColorDefault
			selectedDocker.FrameColor = app.dockerFrameColor
			selectedDocker.TitleColor = gocui.ColorDefault
			selectedFilter.FrameColor = gocui.ColorDefault
			selectedFilter.TitleColor = gocui.ColorDefault
			selectedLogs.FrameColor = gocui.ColorGreen
			selectedScrollLogs.FrameColor = gocui.ColorGreen
		case currentView.Name() == "services":
			nextView = "filterList"
			selectedFilterList.FrameColor = gocui.ColorGreen
			selectedFilterList.TitleColor = gocui.ColorGreen
			selectedServices.FrameColor = app.journalListFrameColor
			selectedServices.TitleColor = gocui.ColorDefault
			selectedVarLog.FrameColor = app.fileSystemFrameColor
			selectedVarLog.TitleColor = gocui.ColorDefault
			selectedDocker.FrameColor = app.dockerFrameColor
			selectedDocker.TitleColor = gocui.ColorDefault
			selectedFilter.FrameColor = gocui.ColorDefault
			selectedFilter.TitleColor = gocui.ColorDefault
			selectedLogs.FrameColor = gocui.ColorDefault
			selectedScrollLogs.FrameColor = gocui.ColorDefault
		case currentView.Name() == "logs":
			if app.timestampFilterView {
				nextView = "untilFilter"
				selectedFilterList.FrameColor = gocui.ColorDefault
				selectedFilterList.TitleColor = gocui.ColorDefault
				selectedServices.FrameColor = app.journalListFrameColor
				selectedServices.TitleColor = gocui.ColorDefault
				selectedVarLog.FrameColor = app.fileSystemFrameColor
				selectedVarLog.TitleColor = gocui.ColorDefault
				selectedDocker.FrameColor = app.dockerFrameColor
				selectedDocker.TitleColor = gocui.ColorDefault
				untilFilter.FrameColor = gocui.ColorGreen // new
				untilFilter.TitleColor = gocui.ColorGreen // new
				selectedLogs.FrameColor = gocui.ColorDefault
				selectedScrollLogs.FrameColor = gocui.ColorDefault
			} else {
				nextView = "filter"
				selectedFilterList.FrameColor = gocui.ColorDefault
				selectedFilterList.TitleColor = gocui.ColorDefault
				selectedServices.FrameColor = app.journalListFrameColor
				selectedServices.TitleColor = gocui.ColorDefault
				selectedVarLog.FrameColor = app.fileSystemFrameColor
				selectedVarLog.TitleColor = gocui.ColorDefault
				selectedDocker.FrameColor = app.dockerFrameColor
				selectedDocker.TitleColor = gocui.ColorDefault
				selectedFilter.FrameColor = gocui.ColorGreen
				selectedFilter.TitleColor = gocui.ColorGreen
				selectedLogs.FrameColor = gocui.ColorDefault
				selectedScrollLogs.FrameColor = gocui.ColorDefault
			}
		case currentView.Name() == "untilFilter":
			nextView = "sinceFilter"
			selectedFilterList.FrameColor = gocui.ColorDefault
			selectedFilterList.TitleColor = gocui.ColorDefault
			selectedServices.FrameColor = app.journalListFrameColor
			selectedServices.TitleColor = gocui.ColorDefault
			selectedVarLog.FrameColor = app.fileSystemFrameColor
			selectedVarLog.TitleColor = gocui.ColorDefault
			selectedDocker.FrameColor = app.dockerFrameColor
			selectedDocker.TitleColor = gocui.ColorDefault
			sinceFilter.FrameColor = gocui.ColorGreen   // new
			sinceFilter.TitleColor = gocui.ColorGreen   // new
			untilFilter.FrameColor = gocui.ColorDefault // new
			untilFilter.TitleColor = gocui.ColorDefault // new
			selectedLogs.FrameColor = gocui.ColorDefault
			selectedScrollLogs.FrameColor = gocui.ColorDefault
		case currentView.Name() == "filter" || currentView.Name() == "sinceFilter":
			if app.timestampFilterView {
				sinceFilter.FrameColor = gocui.ColorDefault // new
				sinceFilter.TitleColor = gocui.ColorDefault // new
			}
			nextView = "docker"
			selectedFilterList.FrameColor = gocui.ColorDefault
			selectedFilterList.TitleColor = gocui.ColorDefault
			selectedServices.FrameColor = app.journalListFrameColor
			selectedServices.TitleColor = gocui.ColorDefault
			selectedVarLog.FrameColor = app.fileSystemFrameColor
			selectedVarLog.TitleColor = gocui.ColorDefault
			selectedDocker.FrameColor = gocui.ColorGreen
			selectedDocker.TitleColor = gocui.ColorGreen
			selectedFilter.FrameColor = gocui.ColorDefault
			selectedFilter.TitleColor = gocui.ColorDefault
			selectedLogs.FrameColor = gocui.ColorDefault
			selectedScrollLogs.FrameColor = gocui.ColorDefault
		case currentView.Name() == "docker":
			nextView = "varLogs"
			selectedFilterList.FrameColor = gocui.ColorDefault
			selectedFilterList.TitleColor = gocui.ColorDefault
			selectedServices.FrameColor = app.journalListFrameColor
			selectedServices.TitleColor = gocui.ColorDefault
			selectedVarLog.FrameColor = gocui.ColorGreen
			selectedVarLog.TitleColor = gocui.ColorGreen
			selectedDocker.FrameColor = app.dockerFrameColor
			selectedDocker.TitleColor = gocui.ColorDefault
			selectedFilter.FrameColor = gocui.ColorDefault
			selectedFilter.TitleColor = gocui.ColorDefault
			selectedLogs.FrameColor = gocui.ColorDefault
			selectedScrollLogs.FrameColor = gocui.ColorDefault
		case currentView.Name() == "varLogs":
			nextView = "services"
			selectedFilterList.FrameColor = gocui.ColorDefault
			selectedFilterList.TitleColor = gocui.ColorDefault
			selectedServices.FrameColor = gocui.ColorGreen
			selectedServices.TitleColor = gocui.ColorGreen
			selectedVarLog.FrameColor = app.fileSystemFrameColor
			selectedVarLog.TitleColor = gocui.ColorDefault
			selectedDocker.FrameColor = app.dockerFrameColor
			selectedDocker.TitleColor = gocui.ColorDefault
			selectedFilter.FrameColor = gocui.ColorDefault
			selectedFilter.TitleColor = gocui.ColorDefault
			selectedLogs.FrameColor = gocui.ColorDefault
			selectedScrollLogs.FrameColor = gocui.ColorDefault
		}
	}
	if _, err := g.SetCurrentView(nextView); err != nil {
		return err
	}
	return nil
}

func (app *App) setSelectView(g *gocui.Gui, viewName string) error {
	// Сбрасываем цвет всех окон
	views := []string{"filterList", "services", "varLogs", "docker", "filter", "sinceFilter", "untilFilter", "logs"}
	for _, name := range views {
		if v, err := g.View(name); err == nil {
			v.FrameColor = gocui.ColorDefault
			// Исключение для tail
			if name != "logs" {
				v.TitleColor = gocui.ColorDefault
			}
		}
	}
	// Устанавливаем цвет для активного окна
	if v, err := g.View(viewName); err == nil {
		v.FrameColor = gocui.ColorGreen
		if viewName != "logs" {
			v.TitleColor = gocui.ColorGreen
		}
	}
	// Устанавливаем фокус на активное окно
	_, err := g.SetCurrentView(viewName)
	return err
}

// Функция для выхода
func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
