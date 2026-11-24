package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/awesome-gocui/gocui"
)

func TestCreatReport(t *testing.T) {
	file, _ := os.Create("test-report.md")
	defer func() { _ = file.Close() }()
}

func TestWinFiles(t *testing.T) {
	// Пропускаем тест целиком для Linux/macOS/bsd
	if runtime.GOOS != "windows" {
		t.Skip("Skip Windows test")
	}

	// Создаем файл отчета
	file, _ := os.OpenFile("test-report.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer func() { _ = file.Close() }()
	_, _ = file.WriteString("## Windows file logs\n")
	_, _ = file.WriteString("| Lines | Read | Color | Path |\n")
	_, _ = file.WriteString("|-------|------|-------|------|\n")

	// Тестируемые параметры для функции
	testCases := []struct {
		name       string
		selectPath string
	}{
		{"Program Files", "ProgramFiles"},
		{"Program Data", "ProgramData"},
		{"Windows Logs", "WindowsLogs"},
		{"Custom Path", "WinCustomPath"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Заполняем базовые параметры структуры
			app := &App{
				selectPath:       tc.selectPath,
				testMode:         true,
				logViewCount:     "10000",
				logUpdateSeconds: 5,
				getOS:            "windows",
				// Режим и текст для фильтрации
				selectFilterMode: "fuzzy",
				filterText:       "",
				// Инициализируем переменные с регулярными выражениями
				trimHttpRegex:      trimHttpRegex,
				trimHttpsRegex:     trimHttpsRegex,
				hexByteRegex:       hexByteRegex,
				dateTimeRegex:      dateTimeRegex,
				integersInputRegex: integersInputRegex,
				syslogUnitRegex:    syslogUnitRegex,
			}

			currentUser, _ := user.Current()
			app.userName = currentUser.Username
			if strings.Contains(app.userName, "\\") {
				app.userName = strings.Split(app.userName, "\\")[1]
			}
			app.systemDisk = os.Getenv("SystemDrive")
			if len(app.systemDisk) >= 1 {
				app.systemDisk = string(app.systemDisk[0])
			} else {
				app.systemDisk = "C"
			}
			app.customPath = homeDir()

			// Пропускаем тесты для CI
			if app.userName == "runneradmin" {
				t.Skip("Skip test for", app.userName, "in CI")
			}

			// (1) Заполняем массив из названий файлов и путей к ним
			app.loadWinFiles(app.selectPath)
			// Если список файлов пустой, тест будет провален
			if len(app.logfiles) == 0 {
				t.Errorf("File list is null")
			} else {
				t.Log("Log files count:", len(app.logfiles))
			}

			var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
			// Проходимся по всем путям в массиве
			for _, logfile := range app.logfiles {
				// Удаляем покраску из названия файла в массиве (в интерфейсе строка читается без покраски при выборе)
				logFileName := ansiEscape.ReplaceAllString(logfile.name, "")
				// Фиксируем время запуска функции
				startTime := time.Now()
				// (2) Читаем журнал
				app.loadFileLogs(strings.TrimSpace(logFileName), true)
				endTime := time.Since(startTime)
				// (3) Фильтруем и красим
				startTime2 := time.Now()
				app.applyFilter(true)
				endTime2 := time.Since(startTime2)
				// Записываем в отчет путь, количество строк в массива вread из файла, время чтения и фильтрации + покраски
				_, _ = fmt.Fprintf(file, "| %d | %s | %s | %s |\n", len(app.currentLogLines), endTime, endTime2, app.lastLogPath)
			}
		})
	}
}

func TestWinEvents(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skip Windows test")
	}

	file, _ := os.OpenFile("test-report.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer func() { _ = file.Close() }()
	_, _ = file.WriteString("## Windows Event Logs\n")
	_, _ = file.WriteString("| Lines | Read | Color | Event Name |\n")
	_, _ = file.WriteString("|-------|------|-------|------------|\n")

	app := &App{
		testMode:           true,
		logViewCount:       "10000",
		logUpdateSeconds:   5,
		getOS:              "windows",
		systemDisk:         "C",
		userName:           "lifailon",
		selectFilterMode:   "fuzzy",
		filterText:         "",
		trimHttpRegex:      trimHttpRegex,
		trimHttpsRegex:     trimHttpsRegex,
		hexByteRegex:       hexByteRegex,
		dateTimeRegex:      dateTimeRegex,
		integersInputRegex: integersInputRegex,
		syslogUnitRegex:    syslogUnitRegex,
	}

	app.loadWinEvents()
	if len(app.journals) == 0 {
		t.Errorf("Event list is null")
	} else {
		t.Log("Windows Event Logs count:", len(app.journals))
	}

	var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	for _, journal := range app.journals {
		app.updateFile = true
		serviceName := ansiEscape.ReplaceAllString(journal.name, "")
		startTime := time.Now()
		app.loadJournalLogs(strings.TrimSpace(serviceName), true)
		endTime := time.Since(startTime)

		startTime2 := time.Now()
		app.applyFilter(true)
		endTime2 := time.Since(startTime2)

		_, _ = fmt.Fprintf(file, "| %d | %s | %s | %s |\n", len(app.currentLogLines), endTime, endTime2, serviceName)
	}
}

func TestUnixFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skip Linux test")
	}

	file, _ := os.OpenFile("test-report.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer func() { _ = file.Close() }()
	_, _ = file.WriteString("## Unix file logs\n")
	_, _ = file.WriteString("| Lines | Read | Color | Path |\n")
	_, _ = file.WriteString("|-------|------|-------|------|\n")

	testCases := []struct {
		name       string
		selectPath string
	}{
		{"System var logs", "varlog"},
		{"Optional package logs", "customPath"},
		{"Process descriptor logs", "descriptor"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := &App{
				selectPath:         tc.selectPath,
				testMode:           true,
				logViewCount:       "10000",
				logUpdateSeconds:   5,
				getOS:              "linux",
				userName:           "lifailon",
				selectFilterMode:   "fuzzy",
				filterText:         "",
				trimHttpRegex:      trimHttpRegex,
				trimHttpsRegex:     trimHttpsRegex,
				hexByteRegex:       hexByteRegex,
				dateTimeRegex:      dateTimeRegex,
				integersInputRegex: integersInputRegex,
				syslogUnitRegex:    syslogUnitRegex,
				customPath:         homeDir(),
			}

			// Пропускаем тесты в macOS/BSD
			if runtime.GOOS != "linux" && tc.selectPath != "varlog" {
				t.Skip("Skip test for macOS in CI")
			}

			app.loadFiles(app.selectPath)
			if len(app.logfiles) == 0 {
				t.Errorf("File list is null")
			} else {
				t.Log("Log files count:", len(app.logfiles))
			}

			var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
			for _, logfile := range app.logfiles {
				logFileName := ansiEscape.ReplaceAllString(logfile.name, "")
				startTime := time.Now()
				app.loadFileLogs(strings.TrimSpace(logFileName), true)
				endTime := time.Since(startTime)

				startTime2 := time.Now()
				app.applyFilter(true)
				endTime2 := time.Since(startTime2)

				_, _ = fmt.Fprintf(file, "| %d | %s | %s | %s |\n", len(app.currentLogLines), endTime, endTime2, app.lastLogPath)
			}
		})
	}
}

func TestLinuxJournal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skip Linux test")
	}

	file, _ := os.OpenFile("test-report.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer func() { _ = file.Close() }()
	_, _ = file.WriteString("## System logs\n")
	_, _ = file.WriteString("| Lines | Read | Color | Journal Name |\n")
	_, _ = file.WriteString("|-------|------|-------|--------------|\n")

	testCases := []struct {
		name        string
		journalName string
	}{
		{"System units", "systemUnits"},
		{"User units", "userUnits"},
		{"System journals", "systemJournals"},
		{"Kernel boot", "kernelBoot"},
		{"Audit", "auditd"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := &App{
				selectUnits:        tc.journalName,
				testMode:           true,
				logViewCount:       "10000",
				logUpdateSeconds:   5,
				getOS:              "linux",
				selectFilterMode:   "fuzzy",
				filterText:         "",
				trimHttpRegex:      trimHttpRegex,
				trimHttpsRegex:     trimHttpsRegex,
				hexByteRegex:       hexByteRegex,
				dateTimeRegex:      dateTimeRegex,
				integersInputRegex: integersInputRegex,
				syslogUnitRegex:    syslogUnitRegex,
				unitType:           "service",
				journalField:       "SYSLOG_IDENTIFIER",
				journalPriority:    "debug",
				journalBootPeriod:  "all",
			}

			app.loadServices(app.selectUnits)
			if len(app.journals) == 0 {
				if tc.journalName == "auditd" && os.Geteuid() != 0 {
					t.Log("Skip auditd from non-root")
				} else {
					t.Errorf("Journal list is null")
				}
			} else {
				t.Log("Journal count:", len(app.journals))
			}

			if tc.journalName == "auditd" {
				_, _ = file.WriteString("## Audit rules\n")
				_, _ = file.WriteString("| Lines | Read | Color | Rules keys   |\n")
				_, _ = file.WriteString("|-------|------|-------|--------------|\n")
			}

			var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
			for _, journal := range app.journals {
				serviceName := ansiEscape.ReplaceAllString(journal.name, "")
				startTime := time.Now()
				app.loadJournalLogs(strings.TrimSpace(serviceName), true)
				endTime := time.Since(startTime)

				startTime2 := time.Now()
				app.applyFilter(true)
				endTime2 := time.Since(startTime2)

				_, _ = fmt.Fprintf(file, "| %d | %s | %s | %s |\n", len(app.currentLogLines), endTime, endTime2, serviceName)
			}
		})
	}
}

func TestDockerContainer(t *testing.T) {
	file, _ := os.OpenFile("test-report.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer func() { _ = file.Close() }()
	_, _ = file.WriteString("## Container logs\n")
	_, _ = file.WriteString("| Lines | Read | Color | Container Name |\n")
	_, _ = file.WriteString("|-------|------|-------|----------------|\n")

	testCases := []struct {
		name                         string
		selectContainerizationSystem string
	}{
		{"Docker", "docker"},
		{"Compose", "compose"},
		// {"Podman", "podman"},
		// {"Kubernetes", "kubernetes"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Пропускаем не установленые системы
			_, err := exec.LookPath(tc.selectContainerizationSystem)
			if err != nil {
				t.Skip("Skip: ", tc.selectContainerizationSystem, " not installed (environment not found)")
			}
			app := &App{
				selectContainerizationSystem: tc.selectContainerizationSystem,
				testMode:                     true,
				logViewCount:                 "10000",
				logUpdateSeconds:             5,
				selectFilterMode:             "fuzzy",
				filterText:                   "",
				trimHttpRegex:                trimHttpRegex,
				trimHttpsRegex:               trimHttpsRegex,
				hexByteRegex:                 hexByteRegex,
				dateTimeRegex:                dateTimeRegex,
				integersInputRegex:           integersInputRegex,
				syslogUnitRegex:              syslogUnitRegex,
				uniquePrefixColorMap:         make(map[string]string),
			}

			app.loadDockerContainer(app.selectContainerizationSystem)
			if len(app.dockerContainers) == 0 {
				t.Errorf("Container list is null")
			} else {
				t.Log("Container count:", len(app.dockerContainers))
			}

			var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
			for _, dockerContainer := range app.dockerContainers {
				containerName := ansiEscape.ReplaceAllString(dockerContainer.name, "")
				startTime := time.Now()
				app.loadDockerLogs(strings.TrimSpace(containerName), true)
				endTime := time.Since(startTime)

				startTime2 := time.Now()
				app.applyFilter(true)
				endTime2 := time.Since(startTime2)

				_, _ = fmt.Fprintf(file, "| %d | %s | %s | %s |\n", len(app.currentLogLines), endTime, endTime2, containerName)
			}
		})
	}
}

func setColorLogHome(t *testing.T) {
	t.Helper()
	testHome := t.TempDir()
	logData, err := os.ReadFile("color.log")
	if err != nil {
		t.Fatalf("read color.log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testHome, "color.log"), logData, 0600); err != nil {
		t.Fatalf("write color.log fixture: %v", err)
	}
	t.Setenv("HOME", testHome)
}

func TestColor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skip unix test")
	}

	setColorLogHome(t)

	app := &App{
		testMode:           true,
		logViewCount:       "10000",
		logUpdateSeconds:   5,
		selectPath:         "customPath",
		customPath:         homeDir(),
		filterListText:     "color",
		trimHttpRegex:      trimHttpRegex,
		trimHttpsRegex:     trimHttpsRegex,
		hexByteRegex:       hexByteRegex,
		dateTimeRegex:      dateTimeRegex,
		integersInputRegex: integersInputRegex,
		syslogUnitRegex:    syslogUnitRegex,
	}

	// Определяем переменные для покраски
	app.hostName, _ = os.Hostname()
	if strings.Contains(app.hostName, ".") {
		app.hostName = strings.Split(app.hostName, ".")[0]
	}
	currentUser, _ := user.Current()
	app.userName = currentUser.Username
	if strings.Contains(app.userName, "\\") {
		app.userName = strings.Split(app.userName, "\\")[1]
	}
	passwd, _ := os.Open("/etc/passwd")
	scanner := bufio.NewScanner(passwd)
	for scanner.Scan() {
		line := scanner.Text()
		userName := strings.Split(line, ":")
		if len(userName) > 0 {
			app.userNameArray = append(app.userNameArray, userName[0])
		}
	}

	// Загружаем файловые журналы и фильтруем вывод (находим тестовый лог-файл)
	app.loadFiles(app.selectPath)
	app.logfilesNotFilter = app.logfiles
	app.applyFilterList()

	if len(app.logfiles) == 0 {
		t.Fatalf("File list is null")
	} else {
		t.Log("Log files count:", len(app.logfiles))
	}

	// Загружаем журнал
	var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	logFileName := ansiEscape.ReplaceAllString(app.logfiles[0].name, "")
	app.loadFileLogs(strings.TrimSpace(logFileName), true)

	// Выводим содержимое с покраской
	app.applyFilter(true)
	t.Log("Lines: ", len(app.filteredLogLines))
	for _, line := range app.filteredLogLines {
		t.Log(line)
	}
}

func TestFilter(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skip unix test")
	}

	testCases := []struct {
		name             string
		selectFilterMode string
	}{
		{"Exact filter mode", "exact"},
		{"Fuzzy filter mode", "fuzzy"},
		{"Regex filter mode", "regex"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setColorLogHome(t)

			app := &App{
				testMode:           true,
				logViewCount:       "10000",
				logUpdateSeconds:   5,
				selectPath:         "customPath",
				customPath:         homeDir(),
				filterListText:     "color",
				selectFilterMode:   tc.selectFilterMode,
				filterText:         "true",
				trimHttpRegex:      trimHttpRegex,
				trimHttpsRegex:     trimHttpsRegex,
				hexByteRegex:       hexByteRegex,
				dateTimeRegex:      dateTimeRegex,
				integersInputRegex: integersInputRegex,
				syslogUnitRegex:    syslogUnitRegex,
			}

			if tc.selectFilterMode == "regex" {
				app.filterText = "^true$"
			}

			app.hostName, _ = os.Hostname()
			if strings.Contains(app.hostName, ".") {
				app.hostName = strings.Split(app.hostName, ".")[0]
			}
			currentUser, _ := user.Current()
			app.userName = currentUser.Username
			if strings.Contains(app.userName, "\\") {
				app.userName = strings.Split(app.userName, "\\")[1]
			}
			passwd, _ := os.Open("/etc/passwd")
			scanner := bufio.NewScanner(passwd)
			for scanner.Scan() {
				line := scanner.Text()
				userName := strings.Split(line, ":")
				if len(userName) > 0 {
					app.userNameArray = append(app.userNameArray, userName[0])
				}
			}

			app.loadFiles(app.selectPath)
			app.logfilesNotFilter = app.logfiles
			app.applyFilterList()

			if len(app.logfiles) == 0 {
				t.Fatalf("File list is null")
			} else {
				t.Log("Log files count:", len(app.logfiles))
			}

			var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
			logFileName := ansiEscape.ReplaceAllString(app.logfiles[0].name, "")
			app.loadFileLogs(strings.TrimSpace(logFileName), true)

			app.applyFilter(true)
			t.Log("Lines: ", len(app.filteredLogLines))
			for _, line := range app.filteredLogLines {
				t.Log(line)
			}
		})
	}
}

func TestFlags(t *testing.T) {
	app := &App{}
	app.uniquePrefixColorMap = make(map[string]string)
	app.logging = true
	app.loggingPath = "lazyjournal.log"
	app.loggingType = "text"
	app.setupLogging()
	defer func() { _ = app.loggingFile.Close() }()
	showHelp()
	showConfig()
	app.showAudit()
}

func TestCommandColor(t *testing.T) {
	app := &App{
		testMode:                     false,
		startServices:                0,
		selectedJournal:              0,
		startFiles:                   0,
		selectedFile:                 0,
		startDockerContainers:        0,
		selectedDockerContainer:      0,
		selectUnits:                  "services",
		selectPath:                   "varlog",
		selectContainerizationSystem: "docker",
		selectFilterMode:             "exact",
		logViewCount:                 "10000",
		logUpdateSeconds:             5,
		journalListFrameColor:        gocui.ColorDefault,
		fileSystemFrameColor:         gocui.ColorDefault,
		dockerFrameColor:             gocui.ColorDefault,
		autoScroll:                   true,
		trimHttpRegex:                trimHttpRegex,
		trimHttpsRegex:               trimHttpsRegex,
		hexByteRegex:                 hexByteRegex,
		dateTimeRegex:                dateTimeRegex,
		integersInputRegex:           integersInputRegex,
		syslogUnitRegex:              syslogUnitRegex,
		keybindingsEnabled:           true,
		uniquePrefixColorMap:         make(map[string]string),
	}

	// Читаем содержимое тестируемого файла
	data, err := os.ReadFile("color.log")
	if err != nil {
		t.Fatalf("Error read test file: %v", err)
	}
	// Подменяем stdin содержимым из файла
	bytes.NewReader(data)
	// Захватываем stdin
	originalStdin := os.Stdin
	// Создаем pipe, чтобы перенаправить stdin
	pr, pw, _ := os.Pipe()
	os.Stdin = pr
	// Записываем данные в "pipe" (это имитирует передачу данных в stdin)
	go func() {
		_, _ = pw.Write(data)
		_ = pw.Close()
	}()

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

	app.commandLineColor(false)
	// Восстанавливаем оригинальный stdin
	os.Stdin = originalStdin
}

func TestCommandExactFilter(t *testing.T) {
	app := &App{
		testMode:                     false,
		startServices:                0,
		selectedJournal:              0,
		startFiles:                   0,
		selectedFile:                 0,
		startDockerContainers:        0,
		selectedDockerContainer:      0,
		selectUnits:                  "services",
		selectPath:                   "varlog",
		selectContainerizationSystem: "docker",
		selectFilterMode:             "exact",
		logViewCount:                 "10000",
		logUpdateSeconds:             5,
		journalListFrameColor:        gocui.ColorDefault,
		fileSystemFrameColor:         gocui.ColorDefault,
		dockerFrameColor:             gocui.ColorDefault,
		autoScroll:                   true,
		trimHttpRegex:                trimHttpRegex,
		trimHttpsRegex:               trimHttpsRegex,
		hexByteRegex:                 hexByteRegex,
		dateTimeRegex:                dateTimeRegex,
		integersInputRegex:           integersInputRegex,
		syslogUnitRegex:              syslogUnitRegex,
		keybindingsEnabled:           true,
		uniquePrefixColorMap:         make(map[string]string),
	}

	data, err := os.ReadFile("color.log")
	if err != nil {
		t.Fatalf("Error read test file: %v", err)
	}
	bytes.NewReader(data)
	pr, pw, _ := os.Pipe()
	os.Stdin = pr
	go func() {
		_, _ = pw.Write(data)
		_ = pw.Close()
	}()

	var filter = "success"
	app.commandLineExact(filter, false)
	app.commandLineExact(filter, true)
}

func TestCommandFuzzyFilter(t *testing.T) {
	app := &App{
		testMode:                     false,
		startServices:                0,
		selectedJournal:              0,
		startFiles:                   0,
		selectedFile:                 0,
		startDockerContainers:        0,
		selectedDockerContainer:      0,
		selectUnits:                  "services",
		selectPath:                   "varlog",
		selectContainerizationSystem: "docker",
		selectFilterMode:             "exact",
		logViewCount:                 "10000",
		logUpdateSeconds:             5,
		journalListFrameColor:        gocui.ColorDefault,
		fileSystemFrameColor:         gocui.ColorDefault,
		dockerFrameColor:             gocui.ColorDefault,
		autoScroll:                   true,
		trimHttpRegex:                trimHttpRegex,
		trimHttpsRegex:               trimHttpsRegex,
		hexByteRegex:                 hexByteRegex,
		dateTimeRegex:                dateTimeRegex,
		integersInputRegex:           integersInputRegex,
		syslogUnitRegex:              syslogUnitRegex,
		keybindingsEnabled:           true,
		uniquePrefixColorMap:         make(map[string]string),
	}

	data, err := os.ReadFile("color.log")
	if err != nil {
		t.Fatalf("Error read test file: %v", err)
	}
	bytes.NewReader(data)
	pr, pw, _ := os.Pipe()
	os.Stdin = pr
	go func() {
		_, _ = pw.Write(data)
		_ = pw.Close()
	}()

	var filter = "success"
	app.commandLineFuzzy(filter, false)
	app.commandLineFuzzy(filter, true)
}

func TestCommandRegexFilter(t *testing.T) {
	app := &App{
		testMode:                     false,
		startServices:                0,
		selectedJournal:              0,
		startFiles:                   0,
		selectedFile:                 0,
		startDockerContainers:        0,
		selectedDockerContainer:      0,
		selectUnits:                  "services",
		selectPath:                   "varlog",
		selectContainerizationSystem: "docker",
		selectFilterMode:             "exact",
		logViewCount:                 "10000",
		logUpdateSeconds:             5,
		journalListFrameColor:        gocui.ColorDefault,
		fileSystemFrameColor:         gocui.ColorDefault,
		dockerFrameColor:             gocui.ColorDefault,
		autoScroll:                   true,
		trimHttpRegex:                trimHttpRegex,
		trimHttpsRegex:               trimHttpsRegex,
		hexByteRegex:                 hexByteRegex,
		dateTimeRegex:                dateTimeRegex,
		integersInputRegex:           integersInputRegex,
		syslogUnitRegex:              syslogUnitRegex,
		keybindingsEnabled:           true,
		uniquePrefixColorMap:         make(map[string]string),
	}

	data, err := os.ReadFile("color.log")
	if err != nil {
		t.Fatalf("Error read test file: %v", err)
	}
	bytes.NewReader(data)
	pr, pw, _ := os.Pipe()
	os.Stdin = pr
	go func() {
		_, _ = pw.Write(data)
		_ = pw.Close()
	}()

	var filter = "http|127"
	filter = "(?i)" + filter
	regex, err := regexp.Compile(filter)
	if err != nil {
		fmt.Println("Regular expression syntax error")
	}
	app.commandLineRegex(regex, false)
	app.commandLineRegex(regex, true)
}

func TestMainInterface(t *testing.T) {
	go runGoCui(true)
	time.Sleep(3 * time.Second)
}

func TestMockInterface(t *testing.T) {
	app := &App{
		testMode:                     false,
		startServices:                0,
		selectedJournal:              0,
		startFiles:                   0,
		selectedFile:                 0,
		startDockerContainers:        0,
		selectedDockerContainer:      0,
		selectUnits:                  "services",
		selectPath:                   "varlog",
		selectContainerizationSystem: "docker",
		selectFilterMode:             "exact",
		logViewCount:                 "10000",
		logUpdateSeconds:             5,
		journalListFrameColor:        gocui.ColorDefault,
		fileSystemFrameColor:         gocui.ColorDefault,
		dockerFrameColor:             gocui.ColorDefault,
		autoScroll:                   true,
		trimHttpRegex:                trimHttpRegex,
		trimHttpsRegex:               trimHttpsRegex,
		hexByteRegex:                 hexByteRegex,
		dateTimeRegex:                dateTimeRegex,
		integersInputRegex:           integersInputRegex,
		syslogUnitRegex:              syslogUnitRegex,
		keybindingsEnabled:           true,
		uniquePrefixColorMap:         make(map[string]string),
	}

	// Включаем логирование выполняемых команд в файл
	app.logging = true
	app.loggingPath = "lazyjournal.log"
	app.loggingType = "text"
	app.setupLogging()
	defer func() { _ = app.loggingFile.Close() }()
	passLog := "\033[32mPASS\033[0m: "
	debugLog := "\033[33mDEBUG\033[0m: "

	app.getOS = runtime.GOOS
	app.getArch = runtime.GOARCH

	var err error

	// Отключение tcell для CI
	g, err = gocui.NewGui(gocui.OutputSimulator, true)
	var debug = true

	// Debug mode (включить отображение интерфейса и отключить логирование)
	// g, err = gocui.NewGui(gocui.OutputNormal, true)
	// debug = false

	if err != nil {
		log.Panicln(err)
	}

	app.gui = g
	g.SetManagerFunc(app.layout)
	g.Mouse = false

	g.FgColor = gocui.ColorDefault
	g.BgColor = gocui.ColorDefault

	if err := app.setupKeybindings(); err != nil {
		log.Panicln("Error key bindings", err)
	}

	if err := app.layout(g); err != nil {
		log.Panicln(err)
	}

	app.hostName, _ = os.Hostname()
	if strings.Contains(app.hostName, ".") {
		app.hostName = strings.Split(app.hostName, ".")[0]
	}
	currentUser, _ := user.Current()
	app.userName = currentUser.Username
	if strings.Contains(app.userName, "\\") {
		app.userName = strings.Split(app.userName, "\\")[1]
	}
	app.systemDisk = os.Getenv("SystemDrive")
	if len(app.systemDisk) >= 1 {
		app.systemDisk = string(app.systemDisk[0])
	} else {
		app.systemDisk = "C"
	}
	passwd, _ := os.Open("/etc/passwd")
	scanner := bufio.NewScanner(passwd)
	for scanner.Scan() {
		line := scanner.Text()
		userName := strings.Split(line, ":")
		if len(userName) > 0 {
			app.userNameArray = append(app.userNameArray, userName[0])
		}
	}

	if v, err := g.View("services"); err == nil {
		_, viewHeight := v.Size()
		app.maxVisibleServices = viewHeight
	}
	if app.getOS == "windows" {
		v, err := g.View("services")
		if err != nil {
			log.Panicln(err)
		}
		v.Title = " < Windows event logs (0) > "
		go func() {
			app.loadWinEvents()
		}()
	} else {
		app.loadServices(app.selectUnits)
	}

	if v, err := g.View("varLogs"); err == nil {
		_, viewHeight := v.Size()
		app.maxVisibleFiles = viewHeight
	}

	if app.getOS == "windows" {
		selectedVarLog, err := g.View("varLogs")
		if err != nil {
			log.Panicln(err)
		}
		g.Update(func(g *gocui.Gui) error {
			selectedVarLog.Clear()
			_, _ = fmt.Fprintln(selectedVarLog, "Searching log files...")
			selectedVarLog.Highlight = false
			return nil
		})
		selectedVarLog.Title = " < Program Files (0) > "
		app.selectPath = "ProgramFiles"
		go func() {
			app.loadWinFiles(app.selectPath)
		}()
	} else {
		app.loadFiles(app.selectPath)
	}

	if v, err := g.View("docker"); err == nil {
		_, viewHeight := v.Size()
		app.maxVisibleDockerContainers = viewHeight
	}
	app.loadDockerContainer(app.selectContainerizationSystem)

	if _, err := g.SetCurrentView("filterList"); err != nil {
		return
	}

	app.secondsChan = make(chan int, app.logUpdateSeconds)
	go func() {
		app.updateLogBackground(app.secondsChan, false)
	}()

	go func() {
		app.updateWindowSize(1)
	}()

	// Отображение GUI в режиме OutputNormal
	go func() { _ = g.MainLoop() }()

	time.Sleep(3 * time.Second)

	// Check help (F1)
	app.showInterfaceHelp(g)
	app.closeHelp(g)
	if debug {
		textLog := "test help interface (F1)"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// Check ssh and context manager (F2)
	app.showInterfaceManager(g)
	app.closeManager(g)
	if debug {
		textLog := "test ssh and context manager interface (F2)"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// Check highlighting (coloring)
	app.currentLogLines = []string{
		"http://127.0.0.1:8443",
		"https://github.com/Lifailon/lazyjournal",
		"/dev/null",
		"root",
		"warning",
		"stderr",
		"success",
		"restart",
		"0x04",
		"2025-02-26T21:38:35.956968+03:00",
		"127.0.0.1, 127.0.0.1:8443",
	}
	app.updateDelimiter(true)
	app.applyFilter(true)
	time.Sleep(3 * time.Second)
	if debug {
		textLog := "test highlighting (coloring)"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// Обновить вывод лога
	app.updateLogOutput(false)
	if debug {
		textLog := "update log (Ctrl+R)"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// Проверяем фильтрацию текста для списков
	app.filterListText = "a"
	_ = app.createFilterEditor("lists")
	time.Sleep(1 * time.Second)
	// app.filterListText = ""
	app.applyFilterList()
	time.Sleep(1 * time.Second)
	if debug {
		textLog := "test filter lists"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// Очистка фильтров
	app.clearFilterListEditor(g)
	app.clearFilterEditor(g)
	if debug {
		textLog := "clear filters before exit (Ctrl+C)"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// Проверяем фильтрацию по timestamp
	_ = app.timestampFilterEditor("sinceFilter")
	_ = app.timestampFilterEditor("untilFilter")
	time.Sleep(1 * time.Second)
	if debug {
		textLog := "test filter timestamp"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// TAB journals
	if debug {
		textLog := "tab to journals"
		t.Log(debugLog + textLog)
		slog.Debug(textLog)
	}
	_ = app.nextView(g, nil)
	time.Sleep(1 * time.Second)

	// Journals (services)
	if v, err := g.View("services"); err == nil {
		// Перемещаемся по списку вниз
		_ = app.nextService(v, 100)
		time.Sleep(1 * time.Second)
		// Загружаем журнал
		_ = app.selectService(g, v)
		time.Sleep(3 * time.Second)
		// Перемещаемся по списку вверх
		_ = app.prevService(v, 100)
		time.Sleep(1 * time.Second)
		// Переключаем списки (только для Linux)
		if runtime.GOOS != "windows" {
			// Вправо
			if debug {
				textLog := "List next (right)"
				t.Log(debugLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setUnitListRight(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "User units (UNIT)"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setUnitListRight(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "System journals (USER_UNIT)"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setUnitListRight(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "Kernel boot"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setUnitListRight(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "Audit rules keys (auditd)"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setUnitListRight(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "System units (services)"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			// Влево
			if debug {
				textLog := "List back (left)"
				t.Log(debugLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setUnitListLeft(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "Audit rules keys (auditd)"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setUnitListLeft(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "Kernel boot"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setUnitListLeft(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "System journals (USER_UNIT)"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setUnitListLeft(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "User units (UNIT)"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setUnitListLeft(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "System units (services)"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
		}
	}
	if debug {
		textLog := "test journals"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// TAB filesystem
	if debug {
		textLog := "tab to filesystem"
		t.Log(debugLog + textLog)
		slog.Debug(textLog)
	}
	_ = app.nextView(g, nil)
	time.Sleep(1 * time.Second)

	// File System (varLogs)
	if v, err := g.View("varLogs"); err == nil {
		// Перемещаемся по списку вниз
		_ = app.nextFileName(v, 100)
		time.Sleep(1 * time.Second)
		// Загружаем журнал
		_ = app.selectFile(g, v)
		time.Sleep(3 * time.Second)
		// Перемещаемся по списку вверх
		_ = app.prevFileName(v, 100)
		time.Sleep(1 * time.Second)
		if runtime.GOOS != "windows" {
			// Вправо
			if debug {
				textLog := "List next (right)"
				t.Log(debugLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setLogFilesListRight(g, v)
			time.Sleep(10 * time.Second)
			if debug {
				textLog := "Custom path"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setLogFilesListRight(g, v)
			time.Sleep(10 * time.Second)
			if debug {
				textLog := "Process descriptor logs"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setLogFilesListRight(g, v)
			time.Sleep(10 * time.Second)
			if debug {
				textLog := "System var logs"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			// Влево
			if debug {
				textLog := "List back (left)"
				t.Log(debugLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setLogFilesListLeft(g, v)
			time.Sleep(10 * time.Second)
			if debug {
				textLog := "Process descriptor logs"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setLogFilesListLeft(g, v)
			time.Sleep(10 * time.Second)
			if debug {
				textLog := "Custom path"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setLogFilesListLeft(g, v)
			time.Sleep(10 * time.Second)
			if debug {
				textLog := "System var logs"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
		}
	}
	if debug {
		textLog := "test filesystem"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// TAB containerization system
	if debug {
		textLog := "tab to containerization system"
		t.Log(debugLog + textLog)
		slog.Debug(textLog)
	}
	_ = app.nextView(g, nil)
	time.Sleep(1 * time.Second)

	// Containerization System (docker)
	if v, err := g.View("docker"); err == nil {
		// Перемещаемся по списку вниз
		_ = app.nextDockerContainer(v, 100)
		time.Sleep(1 * time.Second)
		// Загружаем журнал (ВРЕМЕННО ОТКЛЮЧЕНО)
		_ = app.selectDocker(g, v)
		time.Sleep(3 * time.Second)
		// Перемещаемся по списку вверх
		_ = app.prevDockerContainer(v, 100)
		time.Sleep(1 * time.Second)
		if runtime.GOOS != "windows" {
			// Вправо
			if debug {
				textLog := "List next (right)"
				t.Log(debugLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setContainersListRight(g, v)
			time.Sleep(2 * time.Second)
			if debug {
				textLog := "Compose"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setContainersListRight(g, v)
			time.Sleep(2 * time.Second)
			if debug {
				textLog := "Podman"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setContainersListRight(g, v)
			time.Sleep(2 * time.Second)
			if debug {
				textLog := "Kubernetes"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setContainersListRight(g, v)
			time.Sleep(2 * time.Second)
			if debug {
				textLog := "Docker"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			// Влево
			if debug {
				textLog := "List back (left)"
				t.Log(debugLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setContainersListLeft(g, v)
			time.Sleep(2 * time.Second)
			if debug {
				textLog := "Kubernetes"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setContainersListLeft(g, v)
			time.Sleep(2 * time.Second)
			if debug {
				textLog := "Podman"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setContainersListLeft(g, v)
			time.Sleep(2 * time.Second)
			if debug {
				textLog := "Compose"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setContainersListLeft(g, v)
			time.Sleep(2 * time.Second)
			if debug {
				textLog := "Docker"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
		}
	}
	if debug {
		textLog := "test containers"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// TAB filter logs
	if debug {
		textLog := "tab to filter logs"
		t.Log(debugLog + textLog)
		slog.Debug(textLog)
	}
	_ = app.nextView(g, nil)

	// Проверяем фильтрацию текста для вывода журнала
	app.filterText = "a"
	app.applyFilter(true)
	time.Sleep(3 * time.Second)
	// Ctrl+W
	app.clearFilterEditor(g)
	app.applyFilter(true)
	time.Sleep(3 * time.Second)
	if debug {
		textLog := "test filter logs output"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// Проверяем режимы фильтрации
	if v, err := g.View("filter"); err == nil {
		// Вверх
		if debug {
			textLog := "Filter mode next (up)"
			t.Log(debugLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Filter fuzzy"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Filter regex"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Filter timestamp"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Filter exact (default)"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		// Вниз
		if debug {
			textLog := "Filter mode back (down)"
			t.Log(debugLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setFilterModeLeft(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Filter timestamp"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setFilterModeLeft(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Filter regex"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setFilterModeLeft(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Filter fuzzy"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setFilterModeLeft(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Filter exact"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
	}
	if debug {
		textLog := "test filter modes"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// TAB logs output
	if debug {
		textLog := "Tab to logs output"
		t.Log(debugLog + textLog)
		slog.Debug(textLog)
	}
	_ = app.nextView(g, nil)
	time.Sleep(1 * time.Second)
	if v, err := g.View("logs"); err == nil {
		// Up tail
		if debug {
			textLog := "Up tail"
			t.Log(debugLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 20K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 30K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 40K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 50K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 100K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 150K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		_ = app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 200K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		// Down tail
		if debug {
			textLog := "Down tail"
			t.Log(debugLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 150K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 100K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 50K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 40K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 30K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 20K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 10K (default)"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 5K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 1K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 500"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		_ = app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 200"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		// Up tail (return)
		if debug {
			textLog := "Up tail (return)"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 500"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 1K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 5K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 10K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		// Up logs output on 1
		if debug {
			textLog := "Up logs output on 1"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.scrollUpLogs(1)
		time.Sleep(1 * time.Second)
		// Up logs output on 10
		if debug {
			textLog := "Up logs output on 10 (Shift+Up)"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.scrollUpLogs(10)
		time.Sleep(1 * time.Second)
		// Up logs output on 500
		if debug {
			textLog := "Up logs output on 500 (Alt+Up)"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.scrollUpLogs(500)
		time.Sleep(1 * time.Second)
		_ = app.scrollUpLogs(500)
		time.Sleep(1 * time.Second)
		// Down logs output on 1
		if debug {
			textLog := "Down logs output on 1"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.scrollDownLogs(1)
		time.Sleep(1 * time.Second)
		// Down logs output on 10
		if debug {
			textLog := "Down logs output on 10 (Shift+Down)"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.scrollDownLogs(10)
		time.Sleep(1 * time.Second)
		// Down logs output on 500
		if debug {
			textLog := "Down logs output on 500 (Alt+Down)"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		_ = app.scrollDownLogs(500)
		time.Sleep(1 * time.Second)
		// Move log output to top
		if debug {
			textLog := "Move log output to top (Ctrl+A/Home)"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.pageUpLogs()
		time.Sleep(1 * time.Second)
		// Move log output to down
		if debug {
			textLog := "Move log output to down (Ctrl+E/End)"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.updateLogsView(true)
		time.Sleep(1 * time.Second)
	}
	if debug {
		textLog := "test log output"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// TAB filter lists
	_ = app.nextView(g, nil)
	time.Sleep(1 * time.Second)

	// Back Tab
	_ = app.backView(g, nil)
	time.Sleep(1 * time.Second)
	_ = app.backView(g, nil)
	time.Sleep(1 * time.Second)
	_ = app.backView(g, nil)
	time.Sleep(1 * time.Second)
	_ = app.backView(g, nil)
	time.Sleep(1 * time.Second)
	_ = app.backView(g, nil)
	time.Sleep(1 * time.Second)
	_ = app.backView(g, nil)
	time.Sleep(1 * time.Second)
	_ = app.backView(g, nil)
	time.Sleep(1 * time.Second)
	if debug {
		textLog := "test back tab (Shift+Tab)"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// Проверяем переключение окон с помощью мыши
	_ = app.setSelectView(g, "filterList")
	time.Sleep(1 * time.Second)
	_ = app.setSelectView(g, "services")
	time.Sleep(1 * time.Second)
	_ = app.setSelectView(g, "varLogs")
	time.Sleep(1 * time.Second)
	_ = app.setSelectView(g, "docker")
	time.Sleep(1 * time.Second)
	_ = app.setSelectView(g, "filter")
	time.Sleep(1 * time.Second)
	_ = app.setSelectView(g, "logs")
	time.Sleep(1 * time.Second)
	if debug {
		textLog := "test mouse"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// Переключаем режим фильтрации на timestamp
	_, _ = g.SetCurrentView("filter")
	if v, err := g.View("filter"); err == nil {
		_ = app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
		_ = app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
		_ = app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
	}

	// Проверяем переключение окон в режиме timestamp
	_ = app.nextView(g, nil)
	time.Sleep(1 * time.Second)
	_ = app.nextView(g, nil)
	time.Sleep(1 * time.Second)
	_ = app.setSelectView(g, "logs")
	time.Sleep(1 * time.Second)
	_ = app.setSelectView(g, "logs")
	time.Sleep(1 * time.Second)
	_ = app.setSelectView(g, "logs")
	time.Sleep(1 * time.Second)
	_ = app.nextView(g, nil)
	time.Sleep(1 * time.Second)

	// Проверяем копирование в буфер обмена
	_, _ = g.SetCurrentView("logs")
	if v, err := g.View("logs"); err == nil {
		_ = app.copyToClipboard(g, v)
		time.Sleep(1 * time.Second)
	}
	if debug {
		textLog := "test clipboard"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// Exit
	_ = quit(g, nil)
}

// Проверяет фокус после смены SSH-хоста (#80)
func TestSSHManagerNavigation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SSH fixture requires a Unix shell")
	}
	origConfig, origGui := config, g
	t.Cleanup(func() {
		config, g = origConfig, origGui
	})
	config = Config{}
	config.Ssh.Hosts = []string{"user@host-a"}

	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "ssh"), []byte("#!/bin/sh\ncase \"$*\" in *uname*) printf Linux ;; esac\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	app := &App{
		testMode:                     true,
		getOS:                        runtime.GOOS,
		getArch:                      runtime.GOARCH,
		selectUnits:                  "systemUnits",
		selectPath:                   "varlog",
		selectContainerizationSystem: "docker",
		selectFilterMode:             "exact",
		logViewCount:                 "10000",
		unitType:                     "service",
		colorModeDisable:             true,
		dockerContext:                "default",
		kubernetesContext:            "default",
		kubernetesNamespace:          "--all-namespaces",
		sshStatus:                    "false",
		globalCurrentView:            "filterList",
		keybindingsEnabled:           true,
		autoScroll:                   true,
		timestampDocker:              true,
		frameColor:                   gocui.ColorWhite,
		titleColor:                   gocui.ColorWhite,
		selectedFrameColor:           gocui.ColorGreen,
		selectedTitleColor:           gocui.ColorYellow,
		filterByDateStatus:           "false",
		journalPriority:              "debug",
		journalBootPeriod:            "all",
	}
	var err error
	g, err = gocui.NewGui(gocui.OutputSimulator, true)
	if err != nil {
		t.Fatal(err)
	}
	app.gui = g
	g.Mouse = false
	if err := app.layout(g); err != nil {
		t.Fatal(err)
	}
	app.showInterfaceManager(g)
	view, err := g.SetCurrentView("sshManager")
	if err != nil {
		t.Fatal(err)
	}
	view.Clear()
	_, _ = fmt.Fprintln(view, "user@host-a")
	if err := app.getSelectedLine(g, view); err != nil {
		t.Fatal(err)
	}
	fresh, err := g.View("sshManager")
	if err != nil {
		t.Fatal(err)
	}
	if fresh == view || g.CurrentView() != fresh {
		t.Fatal("SSH selection did not focus the rebuilt manager")
	}
	if fresh.FrameColor != app.selectedFrameColor || fresh.TitleColor != app.selectedTitleColor {
		t.Fatal("rebuilt SSH manager does not have selected colors")
	}
}

// ---------------------------------------- gocui ----------------------------------------

func TestShowInterfaceInfo(t *testing.T) {
	app := newTestApp()
	g := newTestGui(t)

	app.showInterfaceInfo(g, false, "hello info")
	v, err := g.View("info")
	if err != nil {
		t.Fatalf("info view: %v", err)
	}
	if v.Title != " Info " {
		t.Fatalf("info title = %q, want %q", v.Title, " Info ")
	}
	if !strings.Contains(v.ViewBuffer(), "hello info") {
		t.Fatalf("info buffer = %q", v.ViewBuffer())
	}

	app.showInterfaceInfo(g, true, "boom")
	if v.Title != " Error " {
		t.Fatalf("error title = %q", v.Title)
	}
	if !strings.Contains(v.ViewBuffer(), "boom") {
		t.Fatalf("error buffer = %q", v.ViewBuffer())
	}
}

func TestCloseInfo(t *testing.T) {
	app := newTestApp()
	g := newTestGui(t)
	app.showInterfaceInfo(g, false, "hello")
	if _, err := g.View("info"); err != nil {
		t.Fatalf("info view must exist: %v", err)
	}
	app.closeInfo(g)
	if _, err := g.View("info"); err == nil {
		t.Fatal("info view must be deleted")
	}
}

func TestMoveCursorUp(t *testing.T) {
	app := newTestApp()
	g := newTestGui(t, "sshManager")
	v, _ := g.View("sshManager")
	fmt.Fprintln(v, "line0")
	fmt.Fprintln(v, "line1")
	_ = v.SetCursor(0, 1)
	if err := app.moveCursorUp(g, v); err != nil {
		t.Fatal(err)
	}
	if _, cy := v.Cursor(); cy != 0 {
		t.Fatalf("moveCursorUp cursor y = %d, want 0", cy)
	}
	if err := app.moveCursorUp(g, v); err != nil {
		t.Fatal(err)
	}
}

func TestMoveCursorDown(t *testing.T) {
	app := newTestApp()
	g := newTestGui(t, "sshManager")
	v, _ := g.View("sshManager")
	fmt.Fprintln(v, "line0")
	fmt.Fprintln(v, "line1")
	_ = v.SetCursor(0, 0)

	if err := app.moveCursorDown(g, v); err != nil {
		t.Fatal(err)
	}
	if _, cy := v.Cursor(); cy != 1 {
		t.Fatalf("moveCursorDown cursor y = %d, want 1", cy)
	}
}

func TestNextViewManager(t *testing.T) {
	app := newTestApp()
	g := newTestGui(t, "a", "b", "c")
	views := []string{"a", "b", "c"}

	va, _ := g.View("a")
	_, _ = g.SetCurrentView("a")
	if err := app.nextViewManager(g, va, views); err != nil {
		t.Fatal(err)
	}
	if got := g.CurrentView().Name(); got != "b" {
		t.Fatalf("nextViewManager = %q, want b", got)
	}

	vb, _ := g.View("b")
	_, _ = g.SetCurrentView("b")
	if err := app.nextViewManager(g, vb, views); err != nil {
		t.Fatal(err)
	}
	if got := g.CurrentView().Name(); got != "c" {
		t.Fatalf("nextViewManager = %q, want c", got)
	}

	vc, _ := g.View("c")
	_, _ = g.SetCurrentView("c")
	if err := app.nextViewManager(g, vc, views); err != nil {
		t.Fatal(err)
	}
	if got := g.CurrentView().Name(); got != "a" {
		t.Fatalf("nextViewManager wrap = %q, want a", got)
	}
}

func TestCloseManager(t *testing.T) {
	app := newTestApp()
	g := newTestGui(t, "manager", "sshManager", "dockerContextManager", "kubernetesContextManager", "kubernetesNamespaceManager")

	app.closeManager(g)
	for _, name := range []string{"manager", "sshManager", "dockerContextManager", "kubernetesContextManager", "kubernetesNamespaceManager"} {
		if _, err := g.View(name); err == nil {
			t.Fatalf("view %q must be deleted", name)
		}
	}
}

func TestRestoreMainFocusAfterManager(t *testing.T) {
	app := newTestApp()
	g := newTestGui(t, "filterList", "services", "varLogs", "docker", "logs", "sshManager")
	app.globalCurrentView = "varLogs"
	if _, err := g.SetCurrentView("sshManager"); err != nil {
		t.Fatal(err)
	}

	app.closeManager(g)
	app.restoreMainFocus(g)

	if g.CurrentView() == nil {
		t.Fatal("focus was not restored (nil)")
	}
	if got := g.CurrentView().Name(); got != "varLogs" {
		t.Fatalf("focus = %q, want varLogs", got)
	}
	if app.globalCurrentView != "varLogs" {
		t.Fatalf("globalCurrentView = %q, want varLogs", app.globalCurrentView)
	}
}

func TestCloseHelp(t *testing.T) {
	app := newTestApp()
	g := newTestGui(t, "help")
	if _, err := g.View("help"); err != nil {
		t.Fatalf("help view must exist: %v", err)
	}
	app.closeHelp(g)
	if _, err := g.View("help"); err == nil {
		t.Fatal("help view must be deleted")
	}
}

// ---------------------------------------- ssh ----------------------------------------

// Проверка POSIX-экранирования аргументов удаленной команды
func TestShellQuote(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "''"},
		{"simple", "uname", "'uname'"},
		{"spaces", "docker compose", "'docker compose'"},
		{"single quote", "a'b", "'a'\\''b'"},
		{"glob", "*.log", "'*.log'"},
		{"dollar", "$HOME", "'$HOME'"},
		{"semicolon", "a;b", "'a;b'"},
		{"double quotes", `a"b`, `'a"b'`},
		{"stat format", "%n|%s|%Y", "'%n|%s|%Y'"},
		{"docker format", "{{.ID}} {{.Names}} {{.State}}", "'{{.ID}} {{.Names}} {{.State}}'"},
		{"jsonpath", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}", "'jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}'"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shellQuote(c.in); got != c.want {
				t.Fatalf("shellQuote(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// Проверка построения команды - локальный запуск и SSH с экранированием
func TestSSHExec(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localCases := []struct {
		name string
		ctx  context.Context
	}{
		{"local nil ctx", nil},
		{"local with ctx", ctx},
	}
	for _, c := range localCases {
		t.Run(c.name, func(t *testing.T) {
			cmd := sshExec(c.ctx, false, []string{"ignored"}, "journalctl", "--no-pager", "-n", "100")
			want := []string{"journalctl", "--no-pager", "-n", "100"}
			if strings.Join(cmd.Args, " ") != strings.Join(want, " ") {
				t.Fatalf("Args = %v, want %v", cmd.Args, want)
			}
		})
	}

	t.Run("ssh nil ctx", func(t *testing.T) {
		cmd := sshExec(nil, true, []string{"-p", "22", "user@host"}, "journalctl", "--no-pager", "-n", "100")
		want := []string{"ssh", "-p", "22", "user@host", "'journalctl' '--no-pager' '-n' '100'"}
		if strings.Join(cmd.Args, " ") != strings.Join(want, " ") {
			t.Fatalf("Args = %v, want %v", cmd.Args, want)
		}
	})

	t.Run("ssh with ctx", func(t *testing.T) {
		cmd := sshExec(ctx, true, []string{"user@host"}, "uname", "-s")
		want := []string{"ssh", "user@host", "'uname' '-s'"}
		if strings.Join(cmd.Args, " ") != strings.Join(want, " ") {
			t.Fatalf("Args = %v, want %v", cmd.Args, want)
		}
	})

	t.Run("ssh quoting spaces and glob", func(t *testing.T) {
		cmd := sshExec(nil, true, []string{"sudo", "user@host"}, "find", "/var/log", "-name", "*.log")
		want := []string{"ssh", "sudo", "user@host", "'find' '/var/log' '-name' '*.log'"}
		if strings.Join(cmd.Args, " ") != strings.Join(want, " ") {
			t.Fatalf("Args = %v, want %v", cmd.Args, want)
		}
	})

	t.Run("ssh single quote escaping", func(t *testing.T) {
		cmd := sshExec(nil, true, nil, "sh", "-c", "echo 'hi'")
		want := []string{"ssh", "'sh' '-c' 'echo '\\''hi'\\'''"}
		if strings.Join(cmd.Args, " ") != strings.Join(want, " ") {
			t.Fatalf("Args = %v, want %v", cmd.Args, want)
		}
	})

	t.Run("ssh does not mutate options", func(t *testing.T) {
		opts := make([]string, 1, 4)
		opts[0] = "user@host"
		_ = sshExec(nil, true, opts, "uname", "-s")
		if len(opts) != 1 || opts[0] != "user@host" {
			t.Fatalf("sshOptions mutated: %v", opts)
		}
	})
}

// Проверка обертки App.sshExec (использование app.sshMode/app.sshOptions)
func TestAppSSHExec(t *testing.T) {
	t.Run("local", func(t *testing.T) {
		app := &App{sshMode: false}
		cmd := app.sshExec(nil, "uname", "-s")
		want := []string{"uname", "-s"}
		if strings.Join(cmd.Args, " ") != strings.Join(want, " ") {
			t.Fatalf("Args = %v, want %v", cmd.Args, want)
		}
	})

	t.Run("ssh", func(t *testing.T) {
		app := &App{sshMode: true, sshOptions: []string{"user@host"}}
		cmd := app.sshExec(nil, "uname", "-s")
		want := []string{"ssh", "user@host", "'uname' '-s'"}
		if strings.Join(cmd.Args, " ") != strings.Join(want, " ") {
			t.Fatalf("Args = %v, want %v", cmd.Args, want)
		}
	})
}

// Создает исполняемую заглушку с заданным именем в binDir (кросс-платформенно)
func writeFakeBin(t *testing.T, binDir, name string, lines ...string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := filepath.Join(binDir, name+".cmd")
		content := "@echo off\r\n" + strings.Join(lines, "\r\n") + "\r\n"
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	} else {
		path := filepath.Join(binDir, name)
		content := "#!/bin/sh\n" + strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// Добавляет binDir в начало PATH, чтобы exec.Command находил заглушку
func setFakeBinPath(t *testing.T, binDir string) {
	t.Helper()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// Возвращает команду для вывода value буквально (экранирует | для sh и cmd)
func fakeEcho(value string) string {
	if runtime.GOOS == "windows" {
		return "echo " + strings.ReplaceAll(value, "|", "^|")
	}
	return "echo '" + value + "'"
}

// Проверка remoteGetOS через заглушку ssh (без реального подключения)
func TestRemoteGetOS(t *testing.T) {
	t.Run("linux", func(t *testing.T) {
		binDir := t.TempDir()
		writeFakeBin(t, binDir, "ssh", "echo Linux")
		setFakeBinPath(t, binDir)
		got, err := remoteGetOS([]string{"user@host"})
		if err != nil {
			t.Fatalf("remoteGetOS returned error: %v", err)
		}
		if got != "linux" {
			t.Fatalf("remoteGetOS = %q, want %q", got, "linux")
		}
	})

	t.Run("error", func(t *testing.T) {
		binDir := t.TempDir()
		writeFakeBin(t, binDir, "ssh", "echo connection refused >&2", "exit 1")
		setFakeBinPath(t, binDir)
		if _, err := remoteGetOS([]string{"user@host"}); err == nil {
			t.Fatal("remoteGetOS expected an error, got nil")
		}
	})
}

// Проверка statFile в локальном и ssh-режимах
func TestStatFile(t *testing.T) {
	t.Run("local", func(t *testing.T) {
		app := &App{}
		path := filepath.Join(t.TempDir(), "test.log")
		if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
		fi, err := app.statFile(path)
		if err != nil {
			t.Fatalf("statFile local: %v", err)
		}
		if fi.Name() != "test.log" || fi.Size() != 5 {
			t.Fatalf("statFile local = %q/%d, want test.log/5", fi.Name(), fi.Size())
		}
	})

	t.Run("ssh valid", func(t *testing.T) {
		binDir := t.TempDir()
		writeFakeBin(t, binDir, "ssh", fakeEcho("/var/log/syslog|8744995|1756116219"))
		setFakeBinPath(t, binDir)

		app := &App{sshMode: true, sshOptions: []string{"user@host"}}
		fi, err := app.statFile("/var/log/syslog")
		if err != nil {
			t.Fatalf("statFile ssh: %v", err)
		}
		if fi.Name() != "/var/log/syslog" || fi.Size() != 8744995 {
			t.Fatalf("statFile ssh = %q/%d", fi.Name(), fi.Size())
		}
	})

	t.Run("ssh invalid fields", func(t *testing.T) {
		binDir := t.TempDir()
		writeFakeBin(t, binDir, "ssh", "echo only-two")
		setFakeBinPath(t, binDir)

		app := &App{sshMode: true, sshOptions: []string{"user@host"}}
		if _, err := app.statFile("/var/log/syslog"); !errors.Is(err, ErrInvalidStat) {
			t.Fatalf("statFile ssh expected ErrInvalidStat, got %v", err)
		}
	})

	t.Run("ssh bad size", func(t *testing.T) {
		binDir := t.TempDir()
		writeFakeBin(t, binDir, "ssh", fakeEcho("/var/log/syslog|notanumber|1756116219"))
		setFakeBinPath(t, binDir)

		app := &App{sshMode: true, sshOptions: []string{"user@host"}}
		if _, err := app.statFile("/var/log/syslog"); err == nil {
			t.Fatal("statFile ssh expected ParseInt error, got nil")
		}
	})
}

// Проверка statFiles (пустой список и парсинг ssh-вывода)
func TestStatFiles(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		app := &App{}
		got, err := app.statFiles(nil)
		if err != nil {
			t.Fatalf("statFiles empty: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("statFiles empty = %v, want empty map", got)
		}
	})

	t.Run("ssh valid", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("cmd echo adds \\r to multiline stat output")
		}
		binDir := t.TempDir()
		writeFakeBin(t, binDir, "ssh",
			fakeEcho("/a|1|1"),
			fakeEcho("/b|2|2"),
		)
		setFakeBinPath(t, binDir)

		app := &App{sshMode: true, sshOptions: []string{"user@host"}}
		got, err := app.statFiles([]string{"/a", "/b"})
		if err != nil {
			t.Fatalf("statFiles ssh: %v", err)
		}
		if len(got) != 2 || got["/a"].Size() != 1 || got["/b"].Size() != 2 {
			t.Fatalf("statFiles ssh = %v", got)
		}
	})

	t.Run("ssh invalid", func(t *testing.T) {
		binDir := t.TempDir()
		writeFakeBin(t, binDir, "ssh", "echo broken-line")
		setFakeBinPath(t, binDir)

		app := &App{sshMode: true, sshOptions: []string{"user@host"}}
		if _, err := app.statFiles([]string{"/a"}); !errors.Is(err, ErrInvalidStat) {
			t.Fatalf("statFiles ssh expected ErrInvalidStat, got %v", err)
		}
	})
}

// Проверка getContainersFromCompose через заглушку docker/docker-compose
func TestGetContainersFromCompose(t *testing.T) {
	t.Run("docker success", func(t *testing.T) {
		binDir := t.TempDir()
		writeFakeBin(t, binDir, "docker", "echo web", "echo db")
		setFakeBinPath(t, binDir)

		app := &App{dockerCompose: "docker", dockerContext: "default"}
		got := app.getContainersFromCompose("proj")
		if len(got) != 2 || strings.TrimSpace(got[0]) != "web" || strings.TrimSpace(got[1]) != "db" {
			t.Fatalf("getContainersFromCompose docker = %v", got)
		}
	})

	t.Run("docker-compose success", func(t *testing.T) {
		binDir := t.TempDir()
		writeFakeBin(t, binDir, "docker-compose", "echo web")
		setFakeBinPath(t, binDir)

		app := &App{dockerCompose: "docker-compose", dockerContext: "default"}
		got := app.getContainersFromCompose("proj")
		if len(got) != 1 || strings.TrimSpace(got[0]) != "web" {
			t.Fatalf("getContainersFromCompose docker-compose = %v", got)
		}
	})

	t.Run("error", func(t *testing.T) {
		binDir := t.TempDir()
		writeFakeBin(t, binDir, "docker", "exit 1")
		setFakeBinPath(t, binDir)

		app := &App{dockerCompose: "docker", dockerContext: "default"}
		if got := app.getContainersFromCompose("proj"); len(got) != 0 {
			t.Fatalf("getContainersFromCompose error = %v, want empty", got)
		}
	})
}

// ---------------------------------------- Status bar ----------------------------------------

// Проверка форматирования логических значений в статусе
func TestStatusBool(t *testing.T) {
	tests := []struct {
		name  string
		value bool
		want  string
	}{
		{"on", true, "\033[32mon\033[0m"},
		{"off", false, "\033[33moff\033[0m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusBool(tt.value); got != tt.want {
				t.Fatalf("statusBool(%t) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// Проверка сокращения количества строк в статусе
func TestStatusInt(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"200", "200"},
		{"500", "500"},
		{"1000", "1K"},
		{"5000", "5K"},
		{"10000", "10K"},
		{"150000", "150K"},
		{"200000", "200K"},
		{"0", "0"},
		{"000", "K"},
		{"1234", "1234"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := statusInt(tt.input); got != tt.want {
				t.Fatalf("statusInt(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// Проверка форматирования строковых значений в статусе
func TestStatusString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "\033[2m—\033[0m"},
		{"true", "\033[32mon\033[0m"},
		{"false", "\033[33moff\033[0m"},
		{"stream", "\033[32mstream\033[0m"},
		{"json", "\033[33mjson\033[0m"},
		{"all", "\033[2mall\033[0m"},
		{"default", "\033[2mdefault\033[0m"},
		{"debug", "\033[2mdebug\033[0m"},
		{"warning", "\033[33mwarning\033[0m"},
		{"info", "\033[34minfo\033[0m"},
		{"notice", "\033[34mnotice\033[0m"},
		{"err", "\033[31merr\033[0m"},
		{"crit", "\033[31mcrit\033[0m"},
		{"alert", "\033[31malert\033[0m"},
		{"emerg", "\033[31memerg\033[0m"},
		{"since", "\033[32msince\033[0m"},
		{"until", "\033[32muntil\033[0m"},
		{"since & until", "\033[32msince & until\033[0m"},
		{"current", "\033[32mcurrent\033[0m"},
		{"previous", "\033[33mprevious\033[0m"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := statusString(tt.input); got != tt.want {
				t.Fatalf("statusString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// Проверка определения статуса фильтрации по дате
func TestSetStatusDateFilter(t *testing.T) {
	tests := []struct {
		name  string
		since bool
		until bool
		want  string
	}{
		{"since", true, false, "since"},
		{"until", false, true, "until"},
		{"since & until", true, true, "since & until"},
		{"disabled", false, false, "false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{sinceDateFilterMode: tt.since, untilDateFilterMode: tt.until}
			app.setStatusDateFilter()
			if app.filterByDateStatus != tt.want {
				t.Fatalf("filterByDateStatus = %q, want %q", app.filterByDateStatus, tt.want)
			}
		})
	}
}

// Проверка определения статуса фильтрации по номеру загрузки системы
func TestSetStatusBootFilter(t *testing.T) {
	tests := []struct {
		name string
		boot string
		want string
	}{
		{"all", "all", "all"},
		{"current", "0", "current"},
		{"previous", "-1", "previous"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{journalBootPeriod: tt.boot}
			app.setStatusBootFilter()
			if app.filterByBootStatus != tt.want {
				t.Fatalf("filterByBootStatus = %q, want %q", app.filterByBootStatus, tt.want)
			}
		})
	}
}

// Проверка приоритета флага и конфигурации при выборе периода загрузки системы
func TestResolveJournalBoot(t *testing.T) {
	tests := []struct {
		name   string
		passed bool
		flag   string
		config string
		want   string
	}{
		{"flag over config", true, "-1", "-2", "-1"},
		{"config fallback", false, "all", "-2", "-2"},
		{"default", false, "all", "", "all"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveJournalBoot(tt.passed, tt.flag, tt.config); got != tt.want {
				t.Fatalf("resolveJournalBoot(%t, %q, %q) = %q, want %q", tt.passed, tt.flag, tt.config, got, tt.want)
			}
		})
	}
}

// Проверка цикла переключения периода загрузки системы
func TestNextJournalBoot(t *testing.T) {
	tests := []struct {
		name       string
		current    string
		initial    string
		initialSet bool
		want       string
	}{
		{"all", "all", "", false, "0"},
		{"current", "0", "", false, "-1"},
		{"previous", "-1", "", false, "all"},
		{"previous to flag value", "-1", "-2", true, "-2"},
		{"flag value to all", "-2", "-2", true, "all"},
		{"unknown value", "5", "", false, "all"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextJournalBoot(tt.current, tt.initial, tt.initialSet); got != tt.want {
				t.Fatalf("nextJournalBoot(%q, %q, %t) = %q, want %q", tt.current, tt.initial, tt.initialSet, got, tt.want)
			}
		})
	}
}

// Проверка формирования аргументов journalctl для journald-списков
func TestBuildJournaldArgs(t *testing.T) {
	app := &App{
		journalField:      "SYSLOG_IDENTIFIER",
		journalPriority:   "err",
		journalBootPeriod: "0",
		logViewCount:      "10000",
	}

	// Системные юниты: фильтр по unit, boot и приоритету
	args := app.buildJournaldArgs("systemUnits", "sshd.service")
	for _, want := range []string{"--unit=sshd.service", "--boot=0", "--priority=err", "--no-pager", "--lines=10000"} {
		if !slices.Contains(args, want) {
			t.Fatalf("systemUnits args missing %q: %v", want, args)
		}
	}
	if slices.Contains(args, "--user") {
		t.Fatalf("systemUnits args must not contain --user: %v", args)
	}

	// Пользовательские юниты: добавляется --user
	args = app.buildJournaldArgs("userUnits", "_all")
	if !slices.Contains(args, "--user") {
		t.Fatalf("userUnits args missing --user: %v", args)
	}
	if slices.Contains(args, "--unit=_all") {
		t.Fatalf("userUnits args must not contain --unit=_all: %v", args)
	}

	// Системные журналы: фильтр по полю вместо unit
	args = app.buildJournaldArgs("systemJournals", "kernel")
	if !slices.Contains(args, "SYSLOG_IDENTIFIER=kernel") {
		t.Fatalf("systemJournals args missing field filter: %v", args)
	}
	if slices.Contains(args, "--unit=kernel") {
		t.Fatalf("systemJournals args must not contain --unit=kernel: %v", args)
	}

	// Фильтры по времени добавляются парами флаг/значение
	app.sinceDateFilterMode = true
	app.sinceFilterText = "2026-01-01"
	app.untilDateFilterMode = true
	app.untilFilterText = "2026-01-02"
	args = app.buildJournaldArgs("systemUnits", "_all")
	if !slices.Contains(args, "--since") || !slices.Contains(args, "2026-01-01") {
		t.Fatalf("args missing --since filter: %v", args)
	}
	if !slices.Contains(args, "--until") || !slices.Contains(args, "2026-01-02") {
		t.Fatalf("args missing --until filter: %v", args)
	}
}

// Проверка отрисовки двухстрочного статуса и выравнивания подсказки
func TestRenderStatus(t *testing.T) {
	oldHelp := config.Hotkeys.ShowHelp
	config.Hotkeys.ShowHelp = "f1"
	defer func() { config.Hotkeys.ShowHelp = oldHelp }()

	gui, err := gocui.NewGui(gocui.OutputSimulator, true)
	if err != nil {
		t.Fatalf("new gui: %v", err)
	}
	defer gui.Close()

	app := &App{
		autoScroll:                true,
		logViewCount:              "10000",
		logUpdateSeconds:          5,
		ignoreCaseFilter:          false,
		filterByDateStatus:        "since",
		journalPriority:           "debug",
		filterByBootStatus:        "all",
		colorModeDisable:          false,
		sshStatus:                 "off",
		dockerContext:             "default",
		dockerStreamLogsStatus:    "stream",
		kubernetesContext:         "default",
		kubernetesNamespaceStatus: "all",
		kubernetesPreviousPods:    "current",
		timestampDocker:           false,
	}

	var buffer string
	var width int
	rendered := make(chan struct{})
	gui.SetManagerFunc(func(g *gocui.Gui) error {
		_, maxY := g.Size()
		v, err := g.SetView("status", -1, maxY-3, 300, maxY, 8)
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		app.renderStatus(v)
		buffer = v.ViewBuffer()
		width, _ = v.Size()
		select {
		case <-rendered:
		default:
			close(rendered)
		}
		return nil
	})
	go func() {
		<-rendered
		time.Sleep(20 * time.Millisecond)
		gui.Update(func(g *gocui.Gui) error { return gocui.ErrQuit })
	}()
	if err := gui.MainLoop(); err != nil && err != gocui.ErrQuit {
		t.Fatalf("main loop: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buffer, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected 2 status lines, got %d: %q", len(lines), buffer)
	}
	first := removeANSI(lines[0])
	for _, want := range []string{
		"STREAM",
		"Update on",
		"Lines 10K",
		"Interval 5s",
		"FILTER",
		"Date since",
		"Priority debug",
		"Boot all",
		"Color on",
	} {
		if !strings.Contains(first, want) {
			t.Fatalf("first status line missing %q: %q", want, first)
		}
	}
	second := removeANSI(lines[1])
	for _, want := range []string{
		"SSH off",
		"DOCKER Context default",
		"Source stream",
		"Timestamp off",
		"K8S Context default",
		"Namespace all",
		"Pod current",
	} {
		if !strings.Contains(second, want) {
			t.Fatalf("second status line missing %q: %q", want, second)
		}
	}
	if strings.Index(second, "Timestamp") > strings.Index(second, "K8S") {
		t.Fatalf("Timestamp must be before K8S: %q", second)
	}
	if !strings.HasSuffix(second, "Help F1") {
		t.Fatalf("help is not right-aligned at the end: %q", second)
	}
	if got := len([]rune(second)); got != width {
		t.Fatalf("second status line width = %d, want view width %d: %q", got, width, second)
	}
}

func TestUpdateStatus(t *testing.T) {
	app := &App{
		gui:          newTestGui(t),
		autoScroll:   true,
		logViewCount: "10000",
	}
	app.updateStatus()

	app.gui = newTestGui(t, "status")
	app.updateStatus()
	view, err := app.gui.View("status")
	if err != nil {
		t.Fatalf("view status: %v", err)
	}
	if !strings.Contains(removeANSI(view.ViewBuffer()), "STREAM") {
		t.Fatalf("status view was not rendered: %q", view.ViewBuffer())
	}
}

func TestRenderStatusNoHelp(t *testing.T) {
	oldHelp := config.Hotkeys.ShowHelp
	config.Hotkeys.ShowHelp = "   "
	defer func() { config.Hotkeys.ShowHelp = oldHelp }()

	app := &App{
		gui:                newTestGui(t, "status"),
		autoScroll:         true,
		logViewCount:       "10000",
		logUpdateSeconds:   5,
		sshStatus:          "off",
		dockerContext:      "default",
		journalPriority:    "all",
		filterByBootStatus: "all",
	}
	view, _ := app.gui.View("status")
	app.renderStatus(view)
	buffer := view.ViewBuffer()
	if strings.Contains(removeANSI(buffer), "Help") {
		t.Fatalf("help must be hidden: %q", buffer)
	}
	if lines := strings.Split(strings.TrimRight(buffer, "\n"), "\n"); len(lines) != 2 {
		t.Fatalf("expected 2 status lines, got %d: %q", len(lines), buffer)
	}
}

func TestRenderStatusNarrowAndFallback(t *testing.T) {
	oldHelp := config.Hotkeys.ShowHelp
	config.Hotkeys.ShowHelp = "?"
	defer func() { config.Hotkeys.ShowHelp = oldHelp }()

	app := &App{
		gui:                newTestGuiWidth(t, 40, "status"),
		autoScroll:         false,
		logViewCount:       "",
		logUpdateSeconds:   1,
		sshStatus:          "off",
		dockerContext:      "default",
		journalPriority:    "all",
		filterByBootStatus: "all",
	}
	view, _ := app.gui.View("status")
	app.renderStatus(view)
	lines := strings.Split(strings.TrimRight(view.ViewBuffer(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 status lines, got %d", len(lines))
	}
	first := removeANSI(lines[0])
	if !strings.Contains(first, "Lines ") {
		t.Fatalf("status fallback line missing: %q", first)
	}
	second := removeANSI(lines[1])
	if !strings.HasSuffix(second, " Help ?") {
		t.Fatalf("narrow status must use one space before help: %q", second)
	}
}

func TestStatusStringUnknown(t *testing.T) {
	if got, want := statusString("custom"), "\033[33mcustom\033[0m"; got != want {
		t.Fatalf("statusString unknown = %q, want %q", got, want)
	}
}

func TestUpdateStatusFilterByDate(t *testing.T) {
	app := &App{
		gui:              newTestGui(t, "status", "logs"),
		colorModeDisable: true,
	}
	app.updateStatusFilterByDate()
	if app.filterByDateStatus != "false" {
		t.Fatalf("filterByDateStatus = %q, want false", app.filterByDateStatus)
	}

	app.sinceDateFilterMode = true
	app.updateStatusFilterByDate()
	if app.filterByDateStatus != "since" {
		t.Fatalf("filterByDateStatus = %q, want since", app.filterByDateStatus)
	}

	app.untilDateFilterMode = true
	app.updateStatusFilterByDate()
	if app.filterByDateStatus != "since & until" {
		t.Fatalf("filterByDateStatus = %q, want since & until", app.filterByDateStatus)
	}
	view, _ := app.gui.View("status")
	if !strings.Contains(removeANSI(view.ViewBuffer()), "Date since & until") {
		t.Fatalf("status view missing date filter: %q", view.ViewBuffer())
	}

	app.sinceDateFilterMode = false
	app.updateStatusFilterByDate()
	if app.filterByDateStatus != "until" {
		t.Fatalf("filterByDateStatus = %q, want until", app.filterByDateStatus)
	}
}

// ---------------------------------------- Filter ----------------------------------------

func newTestApp() *App {
	return &App{
		testMode:             true,
		logViewCount:         "10000",
		logUpdateSeconds:     5,
		minSymbolFilter:      1,
		ignoreCaseFilter:     true,
		selectFilterMode:     "exact",
		hostName:             "testhost",
		userName:             "testuser",
		frameColor:           gocui.ColorDefault,
		selectedFrameColor:   gocui.ColorGreen,
		errorColor:           gocui.ColorRed,
		trimHttpRegex:        trimHttpRegex,
		trimHttpsRegex:       trimHttpsRegex,
		hexByteRegex:         hexByteRegex,
		dateTimeRegex:        dateTimeRegex,
		integersInputRegex:   integersInputRegex,
		syslogUnitRegex:      syslogUnitRegex,
		uniquePrefixColorMap: make(map[string]string),
	}
}

func newTestGui(t *testing.T, names ...string) *gocui.Gui {
	return newTestGuiWidth(t, 100, names...)
}

func newTestGuiWidth(t *testing.T, width int, names ...string) *gocui.Gui {
	t.Helper()
	gui, err := gocui.NewGui(gocui.OutputSimulator, true)
	if err != nil {
		t.Fatalf("new gui: %v", err)
	}
	names = append(names, "scrollLogs")
	for _, name := range names {
		if _, err := gui.SetView(name, 0, 0, width, 20, 0); err != nil && err != gocui.ErrUnknownView {
			gui.Close()
			t.Fatalf("set view %q: %v", name, err)
		}
	}
	t.Cleanup(func() { gui.Close() })
	return gui
}

func feedStdin(t *testing.T, data string) {
	t.Helper()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	old := os.Stdin
	os.Stdin = pr
	t.Cleanup(func() {
		os.Stdin = old
		_ = pr.Close()
	})
	go func() {
		_, _ = pw.WriteString(data)
		_ = pw.Close()
	}()
}

func captureStdout(t *testing.T, fn func()) string {
	return captureOutput(t, &os.Stdout, "stdout", fn)
}

func captureStderr(t *testing.T, fn func()) string {
	return captureOutput(t, &os.Stderr, "stderr", fn)
}

func captureOutput(t *testing.T, target **os.File, name string, fn func()) string {
	t.Helper()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("%s pipe: %v", name, err)
	}
	old := *target
	*target = pw
	done := make(chan string, 1)
	go func() {
		var builder strings.Builder
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			builder.WriteString(scanner.Text())
			builder.WriteByte('\n')
		}
		done <- builder.String()
	}()
	fn()
	_ = pw.Close()
	*target = old
	output := <-done
	_ = pr.Close()
	return output
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("%q does not contain %q", got, want)
	}
}

func TestCompileFilterRegex(t *testing.T) {
	regex, err := compileFilterRegex(`\S+`, true)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !regex.MatchString("ABC") {
		t.Fatal(`\S+ with ignore case must match upper case text`)
	}
	if regex.MatchString("   ") {
		t.Fatal(`\S+ with ignore case must not match spaces`)
	}
	if _, err := compileFilterRegex("[", false); err == nil {
		t.Fatal("invalid regex must return an error")
	}
}

func TestExactFilter(t *testing.T) {
	app := newTestApp()

	got := app.exactFilter("Alpha alpha", "Alpha")
	if strings.Count(got, "\x1b[0;44m") != 2 {
		t.Fatalf("exact ignore case must highlight all matches: %q", got)
	}

	app.ignoreCaseFilter = false
	if got := app.exactFilter("alpha", "Alpha"); got != "" {
		t.Fatalf("case-sensitive exact match must fail, got %q", got)
	}
	if got := app.exactFilter("Alpha Alpha", "Alpha"); strings.Count(got, "\x1b[0;44m") != 2 {
		t.Fatalf("case-sensitive exact match must highlight all matches: %q", got)
	}

	app.ignoreCaseFilter = true
	got = app.exactFilter("İstanbul", "İ")
	if !strings.Contains(got, "\x1b[0;44m") {
		t.Fatalf("unicode exact fallback must highlight: %q", got)
	}
}

func TestFuzzyFilter(t *testing.T) {
	app := newTestApp()

	if got := app.fuzzyFilter("Hello World", "hello world"); !strings.Contains(got, "\x1b[0;44mHello\x1b[0m") {
		t.Fatalf("fuzzy ignore case did not highlight: %q", got)
	}
	if got := app.fuzzyFilter("hello", "hello world"); got != "" {
		t.Fatalf("fuzzy must require all words, got %q", got)
	}
	if got := app.fuzzyFilter("world hello", "hello world"); got == "" {
		t.Fatal("fuzzy must match regardless of word order")
	}
	if got := app.fuzzyFilter("Hello", ""); got != "Hello" {
		t.Fatalf("empty fuzzy filter changed the line: %q", got)
	}
	if got := app.fuzzyFilter("hello world", "HELLO"); !strings.Contains(got, "\x1b[0;44mhello\x1b[0m") {
		t.Fatalf("fuzzy ignore case must accept upper case filter: %q", got)
	}

	app.ignoreCaseFilter = false
	if got := app.fuzzyFilter("Hello", "hello"); got != "" {
		t.Fatalf("case-sensitive fuzzy match must fail, got %q", got)
	}
}

func TestRegexFilter(t *testing.T) {
	app := newTestApp()

	got := app.regexFilter("abc 123 xyz", regexp.MustCompile(`\d+`))
	if !strings.Contains(got, "\x1b[0;44m123\x1b[0m") {
		t.Fatalf("regex did not highlight match: %q", got)
	}
	if got := app.regexFilter("abc", regexp.MustCompile(`\d+`)); got != "" {
		t.Fatalf("regex without match must return empty string, got %q", got)
	}
}

func TestFilterLine(t *testing.T) {
	app := newTestApp()

	got := app.filterLine("Alpha alpha", "exact", "", "Alpha", nil)
	if strings.Count(got, "\x1b[0;44m") != 2 {
		t.Fatalf("exact ignore case must highlight all matches: %q", got)
	}
	if got := app.filterLine("alpha", "exact", "", "Alpha", nil); got == "" {
		t.Fatal("exact ignore case must match")
	}

	app.ignoreCaseFilter = false
	if got := app.filterLine("alpha", "exact", "", "Alpha", nil); got != "" {
		t.Fatalf("case-sensitive exact match must fail, got %q", got)
	}
	app.ignoreCaseFilter = true

	if got := app.filterLine("hello world", "fuzzy", "hello world", "", nil); got == "" {
		t.Fatal("fuzzy filter must match")
	}
	regex := regexp.MustCompile(`\d+`)
	if got := app.filterLine("value 42", "regex", "", "", regex); !strings.Contains(got, "42") {
		t.Fatalf("regex filter must match: %q", got)
	}
	if got := app.filterLine("coffee", "timestamp", "", "coffee", nil); got == "" {
		t.Fatal("timestamp mode must use exact filtering")
	}
	if got := app.filterLine("coffee", "timestamp", "", "zzz", nil); got != "" {
		t.Fatalf("timestamp mode must not match, got %q", got)
	}

	got = app.filterLine("İstanbul", "exact", "", "İ", nil)
	if !strings.Contains(got, "\x1b[0;44m") {
		t.Fatalf("unicode exact fallback must highlight: %q", got)
	}
}

func TestFilterLines(t *testing.T) {
	app := newTestApp()
	app.ignoreCaseFilter = false

	input := []string{"skip", "match one", "skip", "match two"}
	got := app.filterLines(input, "exact", "match", "match", nil)
	if len(got) != 2 {
		t.Fatalf("filterLines returned %d lines, want 2: %v", len(got), got)
	}

	oldProcs := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(oldProcs)
	large := make([]string, 4096)
	for i := range large {
		if i%100 == 0 {
			large[i] = fmt.Sprintf("match %d", i)
		} else {
			large[i] = "skip"
		}
	}
	got = app.filterLines(large, "exact", "match", "match", nil)
	if len(got) != 41 {
		t.Fatalf("parallel filterLines returned %d lines, want 41", len(got))
	}
	assertContains(t, removeANSI(got[0]), "match 0")
	assertContains(t, removeANSI(got[len(got)-1]), "match 4000")
}

func TestTailBytes(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		maxLines int
		want     string
	}{
		{"zero", "a\nb", 0, ""},
		{"negative", "a\nb", -1, ""},
		{"empty", "", 2, ""},
		{"less than limit", "a\nb", 5, "a\nb"},
		{"limit", "a\nb\nc\n", 2, "b\nc\n"},
		{"no trailing newline", "a\nb\nc", 2, "b\nc"},
		{"exact limit", "a\nb", 1, "b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(tailBytes([]byte(c.input), c.maxLines)); got != c.want {
				t.Fatalf("tailBytes(%q, %d) = %q, want %q", c.input, c.maxLines, got, c.want)
			}
		})
	}
}

func TestReadLastLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("a\nb\nc"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readLastLines(path, 2)
	if err != nil {
		t.Fatalf("readLastLines: %v", err)
	}
	if string(got) != "b\nc" {
		t.Fatalf("readLastLines = %q, want %q", got, "b\nc")
	}
	if got, err := readLastLines(path, 0); err != nil || got != nil {
		t.Fatalf("readLastLines maxLines=0 = %q, %v", got, err)
	}
	if _, err := readLastLines(filepath.Join(dir, "missing.log"), 1); err == nil {
		t.Fatal("readLastLines must fail for a missing file")
	}

	empty := filepath.Join(dir, "empty.log")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := readLastLines(empty, 1); err != nil || got != nil {
		t.Fatalf("readLastLines empty = %q, %v", got, err)
	}

	var builder strings.Builder
	for i := 0; i < 120000; i++ {
		_, _ = fmt.Fprintf(&builder, "line-%05d\n", i)
	}
	large := filepath.Join(dir, "large.log")
	if err := os.WriteFile(large, []byte(builder.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = readLastLines(large, 2)
	if err != nil {
		t.Fatalf("readLastLines large: %v", err)
	}
	assertContains(t, string(got), "line-119999")
	assertContains(t, string(got), "line-119998")
}

func TestReadFromOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "offset.log")
	if err := os.WriteFile(path, []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readFromOffset(path, 3)
	if err != nil || string(got) != "3456789" {
		t.Fatalf("readFromOffset(3) = %q, %v", got, err)
	}
	got, err = readFromOffset(path, -1)
	if err != nil || string(got) != "0123456789" {
		t.Fatalf("readFromOffset(-1) = %q, %v", got, err)
	}
	if got, err := readFromOffset(path, 10); err != nil || got != nil {
		t.Fatalf("readFromOffset(size) = %q, %v", got, err)
	}
	if got, err := readFromOffset(path, 100); err != nil || got != nil {
		t.Fatalf("readFromOffset(beyond) = %q, %v", got, err)
	}
	if _, err := readFromOffset(filepath.Join(dir, "missing.log"), 0); err == nil {
		t.Fatal("readFromOffset must fail for a missing file")
	}
}

func TestCommandLineWorkers(t *testing.T) {
	if workers := commandLineWorkers(); workers < 1 || workers > 32 {
		t.Fatalf("commandLineWorkers = %d, want 1..32", workers)
	}
}

func TestCreateFilterEditor(t *testing.T) {
	app := newTestApp()
	app.gui = newTestGui(t, "filter", "logs", "lists")
	app.filterDebounce = time.Hour
	defer func() {
		if app.filterDebounceTimer != nil {
			app.filterDebounceTimer.Stop()
		}
	}()

	view, _ := app.gui.View("filter")
	editor := app.createFilterEditor("logs")
	editor.Edit(view, 0, 'a', 0)
	editor.Edit(view, 0, 'b', 0)
	editor.Edit(view, gocui.KeySpace, 0, 0)
	if got := strings.TrimSuffix(view.Buffer(), "\n"); got != "ab " {
		t.Fatalf("editor buffer = %q, want %q", got, "ab ")
	}
	if app.filterText != "ab" {
		t.Fatalf("filterText = %q, want ab", app.filterText)
	}
	editor.Edit(view, gocui.KeyBackspace, 0, 0)
	if got := strings.TrimSuffix(view.Buffer(), "\n"); got != "ab" {
		t.Fatalf("backspace buffer = %q, want ab", got)
	}

	view.Clear()
	_, _ = fmt.Fprint(view, "ab cd")
	_ = view.SetCursor(5, 0)
	editor.Edit(view, gocui.KeyArrowLeft, 0, gocui.ModAlt)
	if x, _ := view.Cursor(); x != 2 {
		t.Fatalf("alt-left cursor = %d, want 2", x)
	}
	_ = view.SetCursor(0, 0)
	editor.Edit(view, gocui.KeyArrowRight, 0, gocui.ModAlt)
	if x, _ := view.Cursor(); x != 3 {
		t.Fatalf("alt-right cursor = %d, want 3", x)
	}

	view.Clear()
	_, _ = fmt.Fprint(view, "abc")
	_ = view.SetCursor(3, 0)
	editor.Edit(view, gocui.KeyArrowLeft, 0, gocui.ModAlt)
	if x, _ := view.Cursor(); x != 0 {
		t.Fatalf("alt-left without spaces cursor = %d, want 0", x)
	}
	editor.Edit(view, gocui.KeyArrowRight, 0, gocui.ModAlt)
	if x, _ := view.Cursor(); x != 3 {
		t.Fatalf("alt-right without spaces cursor = %d, want 3", x)
	}
	editor.Edit(view, gocui.KeyArrowLeft, 0, 0)
	if x, _ := view.Cursor(); x != 2 {
		t.Fatalf("left cursor = %d, want 2", x)
	}
	editor.Edit(view, gocui.KeyArrowRight, 0, 0)
	if x, _ := view.Cursor(); x != 3 {
		t.Fatalf("right cursor = %d, want 3", x)
	}

	view.Clear()
	_, _ = fmt.Fprint(view, "abc")
	_ = view.SetCursor(1, 0)
	editor.Edit(view, gocui.KeyDelete, 0, 0)
	if got := strings.TrimSuffix(view.Buffer(), "\n"); got != "ac" {
		t.Fatalf("delete buffer = %q, want ac", got)
	}

	app.journalsNotFilter = []Journal{{name: "xray.service"}}
	listEditor := app.createFilterEditor("lists")
	view.Clear()
	listEditor.Edit(view, 0, 'x', 0)
	if app.filterListText != "x" {
		t.Fatalf("filterListText = %q, want x", app.filterListText)
	}
	if len(app.journals) != 1 {
		t.Fatalf("list filter applied %d journals, want 1", len(app.journals))
	}
}

func TestScheduleFilterDebounce(t *testing.T) {
	app := newTestApp()
	app.testMode = false
	app.gui = newTestGui(t, "filter", "logs", "status")
	app.colorModeDisable = true
	app.filterDebounce = 5 * time.Millisecond
	app.filterText = "alpha"
	app.currentLogLines = []string{"alpha", "beta"}
	app.autoScroll = true

	errCh := make(chan error, 1)
	go func() { errCh <- app.gui.MainLoop() }()
	select {
	case err := <-errCh:
		t.Fatalf("main loop exited early: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	done := make(chan struct{})
	go func() {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			app.logStateMutex.Lock()
			applied := len(app.filteredLogLines) > 0
			app.logStateMutex.Unlock()
			if applied {
				close(done)
				app.gui.Update(func(g *gocui.Gui) error { return gocui.ErrQuit })
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	app.scheduleFilterDebounce()
	app.scheduleFilterDebounce()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("debounce did not apply the filter")
	}
	if err := <-errCh; err != nil && err != gocui.ErrQuit {
		t.Fatalf("main loop: %v", err)
	}
}

func TestTimestampFilterEditor(t *testing.T) {
	app := newTestApp()
	app.gui = newTestGui(t, "status", "logs", "sinceFilter", "untilFilter")
	app.colorModeDisable = true
	app.limitFilterDate = time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	app.sinceFilterDate = time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	app.sinceFilterText = "2026-01-05"
	app.untilFilterDate = time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)
	app.untilFilterText = "2026-01-20"

	sinceView, _ := app.gui.View("sinceFilter")
	sinceEditor := app.timestampFilterEditor("sinceFilter")
	sinceEditor.Edit(sinceView, gocui.KeyArrowRight, 0, 0)
	if !app.sinceDateFilterMode || app.sinceFilterText != "2026-01-06" {
		t.Fatalf("since after right = %q, mode %t", app.sinceFilterText, app.sinceDateFilterMode)
	}
	if sinceView.FrameColor != app.selectedFrameColor {
		t.Fatal("since frame must be selected after a valid change")
	}
	sinceEditor.Edit(sinceView, gocui.KeyArrowLeft, 0, 0)
	if app.sinceFilterText != "2026-01-05" {
		t.Fatalf("since after left = %q", app.sinceFilterText)
	}
	sinceEditor.Edit(sinceView, gocui.KeyDelete, 0, 0)
	if app.sinceDateFilterMode || sinceView.FrameColor != app.errorColor {
		t.Fatal("delete must disable the since filter")
	}
	sinceEditor.Edit(sinceView, gocui.KeyBackspace2, 0, 0)
	if app.sinceDateFilterMode {
		t.Fatal("backspace must disable the since filter")
	}
	sinceEditor.Edit(sinceView, 0, 'x', 0)

	app.sinceFilterDate = app.limitFilterDate
	app.sinceFilterText = "2026-01-10"
	app.sinceDateFilterMode = false
	sinceEditor.Edit(sinceView, gocui.KeyArrowRight, 0, 0)
	if app.sinceFilterText != "2026-01-10" || app.sinceDateFilterMode {
		t.Fatal("since filter must ignore dates after the limit")
	}

	oldRight := config.Hotkeys.Right
	config.Hotkeys.Right = "l"
	app.sinceFilterDate = time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	app.sinceFilterText = "2026-01-05"
	sinceEditor.Edit(sinceView, 0, 'l', 0)
	config.Hotkeys.Right = oldRight
	if app.sinceFilterText != "2026-01-06" {
		t.Fatalf("custom since hotkey did not switch date: %q", app.sinceFilterText)
	}

	untilView, _ := app.gui.View("untilFilter")
	untilEditor := app.timestampFilterEditor("untilFilter")
	app.untilFilterDate = time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)
	app.untilFilterText = "2026-01-08"
	app.untilDateFilterMode = false
	untilEditor.Edit(untilView, gocui.KeyArrowRight, 0, 0)
	if !app.untilDateFilterMode || app.untilFilterText != "2026-01-09" {
		t.Fatalf("until after right = %q, mode %t", app.untilFilterText, app.untilDateFilterMode)
	}
	untilEditor.Edit(untilView, gocui.KeyDelete, 0, 0)
	if app.untilDateFilterMode || untilView.FrameColor != app.errorColor {
		t.Fatal("delete must disable the until filter")
	}

	app.sinceFilterDate = time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	app.untilFilterDate = time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)
	app.untilFilterText = "2026-01-06"
	app.untilDateFilterMode = false
	untilEditor.Edit(untilView, gocui.KeyArrowLeft, 0, 0)
	if app.untilFilterText != "2026-01-06" || app.untilDateFilterMode {
		t.Fatal("until filter must ignore dates before since")
	}
	untilEditor.Edit(untilView, 0, 'x', 0)
}

func TestSwitchDate(t *testing.T) {
	app := newTestApp()
	input := time.Date(2026, 1, 5, 12, 30, 0, 0, time.UTC)
	got, text := app.switchDate(input, true)
	if !got.Equal(input.AddDate(0, 0, 1)) || text != "2026-01-06" {
		t.Fatalf("switchDate up = %v, %q", got, text)
	}
	got, text = app.switchDate(input, false)
	if !got.Equal(input.AddDate(0, 0, -1)) || text != "2026-01-04" {
		t.Fatalf("switchDate down = %v, %q", got, text)
	}
}

func TestDateFilterFrameColor(t *testing.T) {
	app := newTestApp()
	app.frameColor = gocui.ColorBlue
	if got := app.dateFilterFrameColor(false, true); got != app.errorColor {
		t.Fatalf("disabled frame color = %v, want error color", got)
	}
	if got := app.dateFilterFrameColor(true, true); got != app.selectedFrameColor {
		t.Fatalf("selected frame color = %v, want selected color", got)
	}
	if got := app.dateFilterFrameColor(true, false); got != app.frameColor {
		t.Fatalf("idle frame color = %v, want frame color", got)
	}
}

func TestApplyFilterList(t *testing.T) {
	app := newTestApp()
	app.journalsNotFilter = []Journal{{name: "sshd.service"}, {name: "nginx.service"}}
	app.logfilesNotFilter = []Logfile{{name: "auth.log"}, {name: "App.Log"}}
	app.dockerContainersNotFilter = []DockerContainers{{name: "web"}, {name: "db"}}

	app.filterListText = "SERVICE"
	app.applyFilterList()
	if len(app.journals) != 2 || len(app.logfiles) != 0 || len(app.dockerContainers) != 0 {
		t.Fatalf("filter result = %d/%d/%d", len(app.journals), len(app.logfiles), len(app.dockerContainers))
	}

	app.filterListText = "APP"
	app.applyFilterList()
	if len(app.logfiles) != 1 {
		t.Fatalf("case-insensitive list filter returned %d files, want 1", len(app.logfiles))
	}

	app.filterListText = "WEB"
	app.applyFilterList()
	if len(app.dockerContainers) != 1 || app.dockerContainers[0].name != "web" {
		t.Fatalf("docker list filter = %v", app.dockerContainers)
	}

	app.filterListText = "zzz"
	app.applyFilterList()
	if len(app.journals) != 0 || len(app.logfiles) != 0 || len(app.dockerContainers) != 0 {
		t.Fatal("list filter must return empty slices")
	}
	if app.selectedJournal != 0 || app.selectedFile != 0 || app.selectedDockerContainer != 0 {
		t.Fatal("list filter must reset selected indices")
	}

	app.testMode = false
	app.gui = newTestGui(t, "services", "varLogs", "docker")
	app.maxVisibleServices = 10
	app.maxVisibleFiles = 10
	app.maxVisibleDockerContainers = 10
	servicesView, _ := app.gui.View("services")
	servicesView.Title = "SERVICES (9) >"
	app.filterListText = "service"
	app.applyFilterList()
	assertContains(t, servicesView.Title, "(1/2) >")
	app.filterListText = "zzz"
	app.applyFilterList()
	assertContains(t, servicesView.Title, "(0) >")

	dockerView, _ := app.gui.View("docker")
	dockerView.Title = "DOCKER (9) >"
	app.filterListText = "web"
	app.applyFilterList()
	assertContains(t, dockerView.Title, "(1/1) >")

	varLogsView, _ := app.gui.View("varLogs")
	varLogsView.Title = "LOGS (9) >"
	app.filterListText = "app"
	app.applyFilterList()
	assertContains(t, varLogsView.Title, "(1/1) >")
}

func TestApplyFilter(t *testing.T) {
	app := newTestApp()
	app.colorModeDisable = true
	app.currentLogLines = []string{"alpha", "beta"}

	app.filterText = ""
	app.applyFilter(true)
	if len(app.filteredLogLines) != 4 {
		t.Fatalf("empty filter returned %d lines, want 4", len(app.filteredLogLines))
	}

	app.filterText = "."
	app.selectFilterMode = "regex"
	app.applyFilter(true)
	if len(app.filteredLogLines) != 4 {
		t.Fatal("regex dot filter must not filter the output")
	}

	app.minSymbolFilter = 3
	app.filterText = "ab"
	app.selectFilterMode = "exact"
	app.applyFilter(true)
	if len(app.filteredLogLines) != 4 {
		t.Fatal("short filter must be ignored")
	}

	app.minSymbolFilter = 1
	app.filterText = "alpha"
	app.applyFilter(true)
	if len(app.filteredLogLines) != 3 {
		t.Fatalf("exact filter returned %d lines, want 3", len(app.filteredLogLines))
	}

	app.filterText = "zzz"
	app.applyFilter(true)
	if len(app.filteredLogLines) != 1 || app.filteredLogLines[0] != "" {
		t.Fatalf("no-match filter must add one empty line: %v", app.filteredLogLines)
	}

	app.selectFilterMode = "fuzzy"
	app.filterText = "ALPHA"
	app.currentLogLines = []string{"alpha"}
	app.applyFilter(true)
	if len(app.filteredLogLines) != 3 {
		t.Fatalf("fuzzy ignore case filter returned %d lines, want 3", len(app.filteredLogLines))
	}

	app.selectFilterMode = "regex"
	app.filterText = `\D+`
	app.currentLogLines = []string{"ABC"}
	app.applyFilter(true)
	if len(app.filteredLogLines) != 3 {
		t.Fatalf("regex ignore case filter returned %d lines, want 3", len(app.filteredLogLines))
	}

	app.filterText = "["
	app.filteredLogLines = []string{"sentinel"}
	oldLogOutput := log.Writer()
	log.SetOutput(io.Discard)
	app.applyFilter(true)
	log.SetOutput(oldLogOutput)
	if len(app.filteredLogLines) != 1 || app.filteredLogLines[0] != "sentinel" {
		t.Fatalf("invalid regex must keep the previous output: %v", app.filteredLogLines)
	}

	app.colorModeDisable = false
	app.currentLogLines = []string{"error"}
	app.filterText = ""
	app.selectFilterMode = "exact"
	app.applyFilter(true)
	if !strings.Contains(app.coloredLogLines[0], "\x1b[31merror") {
		t.Fatalf("coloring was not applied: %q", app.coloredLogLines[0])
	}
}

func TestApplyFilterLockedGUI(t *testing.T) {
	app := newTestApp()
	app.testMode = false
	app.gui = newTestGui(t, "filter", "logs", "status")
	app.colorModeDisable = true
	app.currentLogLines = []string{"alpha", "beta"}
	app.filterText = "alpha"
	app.selectFilterMode = "exact"
	app.ignoreCaseFilter = false

	app.applyFilterLocked(true)
	filterView, _ := app.gui.View("filter")
	if filterView.FrameColor != app.selectedFrameColor {
		t.Fatal("valid filter must set the selected frame color")
	}
	if len(app.filteredLogLines) != 3 {
		t.Fatalf("filter returned %d lines, want 3", len(app.filteredLogLines))
	}
	logsView, _ := app.gui.View("logs")
	assertContains(t, removeANSI(logsView.ViewBuffer()), "alpha")

	app.filteredLogLines = make([]string, 100)
	app.filteredLogLines[0] = "sentinel"
	app.lastFilterText = "alpha"
	app.lastFilterMode = "exact"
	app.logScrollPos = 0
	app.applyFilterLocked(false)
	if app.filteredLogLines[0] != "sentinel" {
		t.Fatal("unchanged filter and scroll position must skip recalculation")
	}

	app.filterText = "["
	app.selectFilterMode = "regex"
	app.ignoreCaseFilter = true
	app.applyFilterLocked(true)
	if filterView.FrameColor != app.errorColor {
		t.Fatal("invalid regex must set the error frame color")
	}

	app.logScrollPos = 50
	app.filterText = ""
	app.selectFilterMode = "exact"
	app.currentLogLines = []string{"alpha"}
	app.filteredLogLines = nil
	app.applyFilterLocked(true)
	if app.logScrollPos != 0 {
		t.Fatalf("scroll position = %d, want 0", app.logScrollPos)
	}

	app.filterText = "alpha"
	app.colorModeDisable = false
	app.currentLogLines = []string{"alpha", "beta"}
	app.filteredLogLines = nil
	app.lastFilterText = ""
	app.logScrollPos = 0
	app.applyFilterLocked(true)
	if !app.lazyColor || app.logColored == nil {
		t.Fatal("lazy coloring must be enabled outside test mode")
	}

	app.disableAutoScroll = true
	app.autoScroll = true
	app.filterText = ""
	app.lastFilterText = ""
	app.currentLogLines = []string{"alpha"}
	app.filteredLogLines = nil
	app.applyFilterLocked(true)
	if app.autoScroll {
		t.Fatal("disabled auto scroll must stay disabled")
	}

	noView := newTestApp()
	noView.testMode = false
	noView.gui = newTestGui(t)
	noView.applyFilterLocked(true)
}

func TestUpdateDebugLoadTime(t *testing.T) {
	app := newTestApp()
	app.debugStartTime = time.Now().Add(-time.Second)
	app.updateDebugLoadTime()
	if app.debugLoadTime == "" {
		t.Fatal("debug load time was not updated")
	}
}

func TestReadCommandLineBatches(t *testing.T) {
	batchData := func(t *testing.T, data string, ctx context.Context) ([]string, []uint64, commandLineReadResult) {
		t.Helper()
		feedStdin(t, data)
		jobs := make(chan commandLineBatch, 64)
		resultCh := make(chan commandLineReadResult, 1)
		go func() { resultCh <- readCommandLineBatches(ctx, jobs) }()
		var lines []string
		var sequences []uint64
		for job := range jobs {
			sequences = append(sequences, job.sequence)
			lines = append(lines, job.lines...)
		}
		return lines, sequences, <-resultCh
	}

	cases := []struct {
		name string
		data string
		want []string
	}{
		{"lf", "a\nb\n", []string{"a", "b"}},
		{"crlf", "a\r\nb\r\n", []string{"a", "b"}},
		{"no final newline", "a\nb", []string{"a", "b"}},
		{"trailing cr", "a\r", []string{"a"}},
		{"empty", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lines, _, result := batchData(t, c.data, context.Background())
			if result.err != nil {
				t.Fatalf("read result: %v", result.err)
			}
			if !slices.Equal(lines, c.want) {
				t.Fatalf("lines = %v, want %v", lines, c.want)
			}
			if result.lines != len(c.want) {
				t.Fatalf("line count = %d, want %d", result.lines, len(c.want))
			}
		})
	}

	var builder strings.Builder
	for i := 0; i < 600; i++ {
		_, _ = fmt.Fprintf(&builder, "line-%d\n", i)
	}
	lines, sequences, result := batchData(t, builder.String(), context.Background())
	if result.err != nil || len(lines) != 600 {
		t.Fatalf("large batch = %d lines, %v", len(lines), result.err)
	}
	if len(sequences) < 2 {
		t.Fatalf("expected multiple batches, got %d", len(sequences))
	}
	for i, sequence := range sequences {
		if sequence != uint64(i) {
			t.Fatalf("sequence[%d] = %d", i, sequence)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	feedStdin(t, "a\n")
	jobs := make(chan commandLineBatch)
	result = readCommandLineBatches(ctx, jobs)
	if result.err == nil {
		t.Fatal("canceled context must return an error")
	}

	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	feedStdin(t, builder.String())
	jobs = make(chan commandLineBatch)
	result = readCommandLineBatches(ctx, jobs)
	if result.err == nil {
		t.Fatal("canceled context must stop a full batch")
	}

	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	feedStdin(t, "a")
	jobs = make(chan commandLineBatch)
	result = readCommandLineBatches(ctx, jobs)
	if result.err == nil {
		t.Fatal("canceled context must stop a pending line")
	}

	closedStdin, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if err := closedStdin.Close(); err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = closedStdin
	result = readCommandLineBatches(context.Background(), make(chan commandLineBatch, 1))
	os.Stdin = oldStdin
	if result.err == nil || errors.Is(result.err, io.EOF) {
		t.Fatalf("closed stdin must return a read error, got %v", result.err)
	}
}

func TestProcessCommandLine(t *testing.T) {
	app := newTestApp()

	feedStdin(t, "alpha\nbeta\n")
	out := captureStdout(t, func() { app.processCommandLine(nil, false) })
	if out != "alpha\nbeta\n" {
		t.Fatalf("passthrough output = %q", out)
	}

	feedStdin(t, "alpha\nbeta\n")
	out = captureStdout(t, func() {
		app.processCommandLine(func(line string) string {
			if strings.Contains(line, "alpha") {
				return line
			}
			return ""
		}, false)
	})
	if out != "alpha\n" {
		t.Fatalf("filtered output = %q", out)
	}

	feedStdin(t, "ERROR\n")
	out = captureStdout(t, func() { app.processCommandLine(nil, true) })
	assertContains(t, out, "\x1b[41m")

	feedStdin(t, "")
	out = captureStdout(t, func() { app.processCommandLine(nil, false) })
	if out != "" {
		t.Fatalf("empty output = %q", out)
	}
}

func TestProcessCommandLineInputErrors(t *testing.T) {
	app := newTestApp()
	oldStdin := os.Stdin

	closedStdin, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if err := closedStdin.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdin = closedStdin
	errOut := captureStderr(t, func() { app.processCommandLine(nil, false) })
	os.Stdin = oldStdin
	if errOut == "" {
		t.Fatal("closed stdin must report an error")
	}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = devNull
	errOut = captureStderr(t, func() { app.processCommandLine(nil, false) })
	os.Stdin = oldStdin
	_ = devNull.Close()
	assertContains(t, errOut, "No data")
}

func TestProcessCommandLineOutputError(t *testing.T) {
	app := newTestApp()
	closedStdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	if err := closedStdout.Close(); err != nil {
		t.Fatal(err)
	}

	oldStdout := os.Stdout
	run := func(data string) string {
		feedStdin(t, data)
		os.Stdout = closedStdout
		errOut := captureStderr(t, func() { app.processCommandLine(nil, false) })
		os.Stdout = oldStdout
		return errOut
	}

	var input strings.Builder
	for i := 0; i < 600; i++ {
		_, _ = fmt.Fprintf(&input, "line-%d\n", i)
	}
	if errOut := run(input.String()); errOut == "" {
		t.Fatal("flush error must be reported")
	}
	if errOut := run(strings.Repeat("x", 2*1024*1024) + "\n"); errOut == "" {
		t.Fatal("write error must be reported")
	}
}

func TestCommandLineFilters(t *testing.T) {
	app := newTestApp()
	app.ignoreCaseFilter = true

	feedStdin(t, "alpha\nbeta\n")
	out := captureStdout(t, func() { app.commandLineExact("ALPHA", false) })
	if removeANSI(out) != "alpha\n" {
		t.Fatalf("exact output = %q", out)
	}

	feedStdin(t, "hello world\nhello\nworld\n")
	out = captureStdout(t, func() { app.commandLineFuzzy("hello WORLD", false) })
	if removeANSI(out) != "hello world\n" {
		t.Fatalf("fuzzy output = %q", out)
	}

	feedStdin(t, "ABC\n")
	regex, err := compileFilterRegex(`\S+`, true)
	if err != nil {
		t.Fatal(err)
	}
	out = captureStdout(t, func() { app.commandLineRegex(regex, false) })
	if removeANSI(out) != "ABC\n" {
		t.Fatalf("regex output = %q", out)
	}

	app.coloredLogLines = []string{"ERROR"}
	out = captureStdout(t, func() { app.commandLineColor(true) })
	assertContains(t, out, "\x1b[41m")

	feedStdin(t, "ERROR\n")
	out = captureStdout(t, func() { app.commandLineColor(false) })
	assertContains(t, out, "\x1b[41m")
}

// ---------------------------------------- Color ----------------------------------------

func TestWordColor(t *testing.T) {
	cases := []struct {
		name string
		word string
		code string
	}{
		{"GET len3", "GET", "\x1b[42m"},
		{"PUT len3", "PUT", "\x1b[45m"},
		{"INF len3", "INF", "\x1b[46m"},
		{"WRN len3", "WRN", "\x1b[43m"},
		{"ERR len3", "ERR", "\x1b[41m"},
		{"status 200", "200", "\x1b[42m"},
		{"status 304", "304", "\x1b[43m"},
		{"status 404", "404", "\x1b[41m"},
		{"status 500", "500", "\x1b[41m"},
		{"status other", "600", "\x1b[34m"},
		{"http", "http://example.com/a1", "\x1b[31mhttp"},
		{"https", "https://example.com/a1", "\x1b[32mhttps"},
		{"path", "/var/log/app.log", "\x1b[35m/"},
		{"quoted path", "\"/var/log\"", "\x1b[35m/"},
		{"home path", "~/logs/app.log", "\x1b[33m~"},
		{"relative path", "./logs/app.log", "\x1b[33m."},
		{"syslog unit", "sshd[123]:", "\x1b[36msshd"},
		{"kernel", "kernel:", "\x1b[36mkernel"},
		{"rsyslogd", "rsyslogd:", "\x1b[36mrsyslogd"},
		{"sudo", "sudo:", "\x1b[36msudo"},
		{"GET method", "GET,", "\x1b[42m"},
		{"POST method", "POST,", "\x1b[43m"},
		{"PUT method", "PUT,", "\x1b[45m"},
		{"PATCH method", "PATCH,", "\x1b[45m"},
		{"TRACE method", "TRACE,", "\x1b[45m"},
		{"OPTIONS method", "OPTIONS,", "\x1b[45m"},
		{"CONNECT method", "CONNECT,", "\x1b[42m"},
		{"DELETE method", "DELETE,", "\x1b[41m"},
		{"DONE status", "DONE!", "\x1b[42m"},
		{"WARNING status", "WARNING!", "\x1b[43m"},
		{"WARN status", "WARN!", "\x1b[43m"},
		{"WRN status", "WRN!", "\x1b[43m"},
		{"DEBUG status", "DEBUG!", "\x1b[46m"},
		{"INFO status", "INFO!", "\x1b[46m"},
		{"INF status", "INF!", "\x1b[46m"},
		{"NOTICE status", "NOTICE!", "\x1b[46m"},
		{"ERROR status", "ERROR!", "\x1b[41m"},
		{"ERR status", "ERR!", "\x1b[41m"},
		{"CRITICAL status", "CRITICAL!", "\x1b[41m"},
		{"CRIT status", "CRIT!", "\x1b[41m"},
		{"ALERT status", "ALERT!", "\x1b[41m"},
		{"EMERGENCY status", "EMERGENCY!", "\x1b[41m"},
		{"EMERG status", "EMERG!", "\x1b[41m"},
		{"FAILURE status", "FAILURE!", "\x1b[41m"},
		{"FAIL status", "FAIL!", "\x1b[41m"},
		{"FATAL status", "FATAL!", "\x1b[41m"},
		{"user", "testuser!", "\x1b[33m"},
		{"warning", "warning!", "\x1b[33m"},
		{"restart", "restarting!", "\x1b[33m"},
		{"reconnect", "reconnecting!", "\x1b[33m"},
		{"retry", "retrying!", "\x1b[33m"},
		{"delimiter", "a⎯b", "\x1b[35m"},
		{"unblock", "unblocking!", "\x1b[32m"},
		{"stderr", "stderr!", "\x1b[31m"},
		{"disconnected", "disconnected!", "\x1b[31m"},
		{"crashed", "crashed!", "\x1b[31m"},
		{"deleted", "deleted!", "\x1b[31m"},
		{"removed", "removed!", "\x1b[31m"},
		{"stopped", "stopped!", "\x1b[31m"},
		{"invalid", "invalid!", "\x1b[31m"},
		{"aborted", "aborted!", "\x1b[31m"},
		{"blocked", "blocked!", "\x1b[31m"},
		{"inactive", "inactive!", "\x1b[31m"},
		{"exited", "exited!", "\x1b[31m"},
		{"critical", "critical!", "\x1b[31m"},
		{"failed", "failed!", "\x1b[31m"},
		{"fatal", "fatal!", "\x1b[31m"},
		{"closed", "closed!", "\x1b[31m"},
		{"dropped", "dropped!", "\x1b[31m"},
		{"panicked", "panicked!", "\x1b[31m"},
		{"emergency", "emergency!", "\x1b[31m"},
		{"rejected", "rejected!", "\x1b[31m"},
		{"refused", "refused!", "\x1b[31m"},
		{"unhealthy", "unhealth!", "\x1b[31m"},
		{"unstable", "unstable!", "\x1b[31m"},
		{"unavailable", "unavailable!", "\x1b[31m"},
		{"unknown", "unknown!", "\x1b[31m"},
		{"unsuccessful", "unsuccessful!", "\x1b[31m"},
		{"unauthorized", "unauthorized!", "\x1b[31m"},
		{"forbidden", "forbidden!", "\x1b[31m"},
		{"conflict", "conflict!", "\x1b[31m"},
		{"false", "false!", "\x1b[31m"},
		{"null", "null!", "\x1b[31m"},
		{"nil", "nil!", "\x1b[31m"},
		{"none", "none!", "\x1b[31m"},
		{"success", "successfully!", "\x1b[32m"},
		{"completed", "completed!", "\x1b[32m"},
		{"accepted", "accepted!", "\x1b[32m"},
		{"connected", "connected!", "\x1b[32m"},
		{"finished", "finished!", "\x1b[32m"},
		{"started", "started!", "\x1b[32m"},
		{"enabled", "enabled!", "\x1b[32m"},
		{"allowed", "allowed!", "\x1b[32m"},
		{"passed", "passed!", "\x1b[32m"},
		{"ready", "ready!", "\x1b[32m"},
		{"available", "available!", "\x1b[32m"},
		{"running", "running!", "\x1b[32m"},
		{"installed", "installed!", "\x1b[32m"},
		{"true", "true!", "\x1b[32m"},
		{"done", "done!", "\x1b[32m"},
		{"hex", "0x1A", "\x1b[34m"},
		{"datetime", "2026-01-01T12:00:00Z", "\x1b[34m"},
		{"integers", "-12.5%", "\x1b[34m"},
		{"session", "session!", "\x1b[36m"},
		{"logged", "logged!", "\x1b[36m"},
		{"registered", "registered!", "\x1b[36m"},
		{"authenticating", "authenticating!", "\x1b[36m"},
		{"returned", "returned!", "\x1b[36m"},
		{"listening", "listening!", "\x1b[36m"},
		{"opened", "opened!", "\x1b[36m"},
		{"created", "created!", "\x1b[36m"},
		{"booting", "booting!", "\x1b[36m"},
		{"loading", "loading!", "\x1b[36m"},
		{"updated", "updated!", "\x1b[36m"},
		{"synchronization", "synchronization!", "\x1b[36m"},
		{"launched", "launched!", "\x1b[36m"},
		{"changed", "changed!", "\x1b[36m"},
		{"cleared", "cleared!", "\x1b[36m"},
		{"skipped", "skipped!", "\x1b[36m"},
		{"missing", "missing!", "\x1b[36m"},
		{"mounted", "mounted!", "\x1b[36m"},
		{"configured", "configured!", "\x1b[36m"},
		{"reading", "reading!", "\x1b[36m"},
		{"writing", "writing!", "\x1b[36m"},
		{"saved", "saved!", "\x1b[36m"},
		{"paused", "paused!", "\x1b[36m"},
		{"normal", "normal!", "\x1b[36m"},
		{"alerting", "alerting!", "\x1b[36m"},
		{"notifications", "notifications!", "\x1b[36m"},
		{"timeout", "timeout!", "\x1b[36m"},
		{"stdout", "stdout!", "\x1b[36m"},
		{"debug", "debug!", "\x1b[36m"},
		{"verbose", "verbose!", "\x1b[36m"},
		{"level", "level!", "\x1b[36m"},
		{"status", "status!", "\x1b[36m"},
		{"shutdown", "shutdown!", "\x1b[36m"},
		{"information", "information!", "\x1b[36m"},
		{"initial", "initial!", "\x1b[36m"},
		{"installing", "installing!", "\x1b[36m"},
		{"installation", "installation!", "\x1b[36m"},
	}
	app := newTestApp()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := app.wordColor(c.word)
			if got == c.word {
				t.Fatalf("wordColor(%q) was not colored", c.word)
			}
			assertContains(t, got, c.code)
		})
	}
}

func TestWordColorHostAndUser(t *testing.T) {
	app := newTestApp()

	got := app.wordColor("testhost!")
	if got == "testhost!" || !strings.Contains(got, "\033[0m") {
		t.Fatalf("host was not colored: %q", got)
	}
	got = app.wordColor("testuser!")
	if got == "testuser!" {
		t.Fatalf("user was not colored: %q", got)
	}
	assertContains(t, got, "\x1b[33m")

	app.userNameSet = map[string]struct{}{"alice": {}}
	if !app.containsUser("alice") {
		t.Fatal("containsUser must find a user in the set")
	}
	if app.containsUser("bob") {
		t.Fatal("containsUser must not find a missing user")
	}
	assertContains(t, app.wordColor("alice"), "\x1b[33m")

	app.userNameSet = nil
	app.userNameArray = []string{"carol"}
	if !app.containsUser("carol") {
		t.Fatal("containsUser must fall back to the array")
	}
	if app.containsUser("dave") {
		t.Fatal("containsUser must return false for an unknown array user")
	}
}

func TestWordColorActionsDisable(t *testing.T) {
	app := newTestApp()
	app.colorActionsDisable = true

	for _, word := range []string{"restarting!", "retrying!", "shutdown!"} {
		if got := app.wordColor(word); got != word {
			t.Fatalf("disabled action %q was colored: %q", word, got)
		}
	}
	if got := app.wordColor("warning!"); got == "warning!" {
		t.Fatal("warning coloring must stay enabled")
	}
}

func TestLineColor(t *testing.T) {
	app := newTestApp()

	if got := app.lineColor(""); got != "" {
		t.Fatalf("empty line = %q", got)
	}
	if got := app.lineColor("hello"); got != "hello" {
		t.Fatalf("plain line = %q", got)
	}

	jsonLine := app.lineColor(`{"key":"value"}`)
	assertContains(t, jsonLine, "\x1b[33m{")
	assertContains(t, jsonLine, "\x1b[33m\"")

	filtered := app.lineColor("a \x1b[0;44mERROR\x1b[0m b")
	if strings.Contains(filtered, "\x1b[31mERROR") {
		t.Fatalf("filter background must suppress word coloring: %q", filtered)
	}

	for _, phrase := range []string{"Not found", "not found", "Bad request", "bad request"} {
		got := app.lineColor(phrase)
		if !strings.Contains(got, "\x1b[31m"+phrase+"\x1b[0m") {
			t.Fatalf("phrase %q was not colored: %q", phrase, got)
		}
	}

	app.lastContainerizationSystem = "compose"
	app.uniquePrefixColorMap["web"] = "\x1b[35m"
	if got := app.lineColor("[web] hello"); got != "[\x1b[35mweb\x1b[0m] hello" {
		t.Fatalf("compose container = %q", got)
	}
	if got := app.lineColor("[db] hello"); got != "[db] hello" {
		t.Fatalf("compose container without color = %q", got)
	}
	if got := app.lineColor("⎯⎯⎯"); !strings.Contains(got, "\x1b[35m") {
		t.Fatalf("compose divider was not colored: %q", got)
	}
	if got := app.lineColor("[web hello"); got != "[web hello" {
		t.Fatalf("compose line without separator = %q", got)
	}
}

func TestMainColor(t *testing.T) {
	app := newTestApp()

	if got := app.mainColor(nil); len(got) != 0 {
		t.Fatalf("empty mainColor = %v", got)
	}
	if got := app.mainColor([]string{"hello"}); len(got) != 1 || got[0] != "hello" {
		t.Fatalf("single mainColor = %v", got)
	}

	oldProcs := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(oldProcs)
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%d", i)
	}
	got := app.mainColor(lines)
	if len(got) != len(lines) {
		t.Fatalf("mainColor returned %d lines, want %d", len(got), len(lines))
	}
	for i, line := range got {
		if line != lines[i] {
			t.Fatalf("mainColor order at %d = %q, want %q", i, line, lines[i])
		}
	}
}

func TestReplaceWordLower(t *testing.T) {
	app := newTestApp()

	got := app.replaceWordLower("hello hello", "hello", "\x1b[33m")
	if strings.Count(got, "\x1b[33mhello\x1b[0m") != 2 {
		t.Fatalf("replaceWordLower = %q", got)
	}
	got = app.replaceWordLower("hello", "hello", "\x1b[42m\x1b[30m")
	if got != "\x1b[42m\x1b[30m hello \x1b[0m" {
		t.Fatalf("background replaceWordLower = %q", got)
	}
	if got := app.replaceWordLower("shelloworld", "hello", "\x1b[33m"); got != "shelloworld" {
		t.Fatalf("word boundary was ignored: %q", got)
	}
}

func TestURLPathColor(t *testing.T) {
	app := newTestApp()
	got := app.urlPathColor("/a?b=1")
	assertContains(t, got, "\x1b[35m/")
	assertContains(t, got, "\x1b[35m?")
	assertContains(t, got, "\x1b[34m1")
}

func TestIntColor(t *testing.T) {
	app := newTestApp()

	assertContains(t, app.intColor("abc123"), "\x1b[34m")
	assertContains(t, app.intColor("1.2%"), "\x1b[35m")
	assertContains(t, app.intColor("abc"), "\x1b[0m")
	got := app.intColor("12")
	if !strings.HasSuffix(got, "\x1b[0m") {
		t.Fatalf("number color was not reset: %q", got)
	}
}

func TestParseStringToInt(t *testing.T) {
	cases := []struct {
		input string
		want  int
		ok    bool
	}{
		{"", 0, false},
		{"12a", 0, false},
		{"-1", 0, false},
		{"42", 42, true},
		{"007", 7, true},
		{"999999999999999999999999", 0, false},
	}
	for _, c := range cases {
		got, ok := parseStringToInt(c.input)
		if got != c.want || ok != c.ok {
			t.Fatalf("parseStringToInt(%q) = %d, %t", c.input, got, ok)
		}
	}
}

func TestCommandLineColor(t *testing.T) {
	app := newTestApp()
	app.coloredLogLines = []string{"alpha", "beta"}
	out := captureStdout(t, func() { app.commandLineColor(true) })
	assertContains(t, out, "alpha")
	assertContains(t, out, "beta")
}

// ---------------------------------------- Helpers ----------------------------------------

// Методы-обертки os.Stat
func TestFileInfo(t *testing.T) {
	mod := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	fi := &fileInfo{name: "a.log", size: 123, modTime: mod}
	if fi.Name() != "a.log" {
		t.Fatalf("Name = %q", fi.Name())
	}
	if fi.Size() != 123 {
		t.Fatalf("Size = %d", fi.Size())
	}
	if !fi.ModTime().Equal(mod) {
		t.Fatalf("ModTime = %v", fi.ModTime())
	}
	if fi.Mode() != 0o644 {
		t.Fatalf("Mode = %v", fi.Mode())
	}
	if fi.IsDir() {
		t.Fatal("IsDir must be false for files")
	}
	if fi.Sys() != nil {
		t.Fatal("Sys must be nil")
	}
}

// Удаление timestamp для разных систем контейнеризации
func TestRemoveTimestamp(t *testing.T) {
	tests := []struct {
		name string
		line string
		sys  string
		want string
	}{
		{"docker with space", "2024-01-01T00:00:00Z message", "docker", "message"},
		{"podman with space", "2024-01-01T00:00:00Z message", "podman", "message"},
		{"docker no space", "nodatestamp", "docker", "nodatestamp"},
		{"compose three fields", "web 2024-01-01T00:00:00Z message", "compose", "web message"},
		{"compose short", "only two", "compose", "only two"},
		{"kubectl three fields", "pod namespace message", "kubectl", "pod message"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeTimestamp(tt.line, tt.sys); got != tt.want {
				t.Fatalf("removeTimestamp(%q, %q) = %q, want %q", tt.line, tt.sys, got, tt.want)
			}
		})
	}
}

// Поиск пользователей по map или слайсу
func TestContainsUser(t *testing.T) {
	app := newTestApp()

	app.userNameArray = []string{"alice", "bob"}
	if !app.containsUser("alice") || !app.containsUser("bob") {
		t.Fatal("containsUser must find slice users")
	}
	if app.containsUser("carol") {
		t.Fatal("containsUser must not find missing user")
	}

	app.userNameSet = map[string]struct{}{"root": {}}
	if !app.containsUser("root") {
		t.Fatal("containsUser must find set user")
	}
	if app.containsUser("alice") {
		t.Fatal("containsUser must prefer userNameSet over slice")
	}
}

// Рразбор сочетаний клавиш из конфигурации
func TestGetHotkey(t *testing.T) {
	tests := []struct {
		name       string
		configKey  string
		defaultKey string
		want       any
		wantMod    gocui.Modifier
	}{
		{"single letter", "a", "", 'a', gocui.ModNone},
		{"f1", "f1", "", gocui.KeyF1, gocui.ModNone},
		{"ctrl+a", "ctrl+a", "", gocui.KeyCtrlA, gocui.ModNone},
		{"alt+f1", "alt+f1", "", gocui.KeyF1, gocui.ModAlt},
		{"shift+a", "shift+a", "", 'A', gocui.ModNone},
		{"tab", "tab", "", gocui.KeyTab, gocui.ModNone},
		{"unknown with default key", "zzz", "f2", gocui.KeyF2, gocui.ModNone},
		{"unknown with default letter", "zzz", "b", 'b', gocui.ModNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, mod := getHotkey(tt.configKey, tt.defaultKey)
			if got != tt.want || mod != tt.wantMod {
				t.Fatalf("getHotkey(%q, %q) = %v/%v, want %v/%v", tt.configKey, tt.defaultKey, got, mod, tt.want, tt.wantMod)
			}
		})
	}
}

// Проверяем, что homeDir возвращает непустую строку
func TestHomeDir(t *testing.T) {
	dir := homeDir()
	if dir == "" {
		t.Fatal("homeDir returned empty string")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Logf("homeDir path: %s, stat error: %v", dir, err)
	}
}

// ---------------------------------------- Сonfig ----------------------------------------

func TestParseConfig(t *testing.T) {
	c := &Config{}
	path, err := c.parseConfig([]byte("settings:\n  loggingType: \"json\"\n"), "test.yml")
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if path != "test.yml" {
		t.Fatalf("parseConfig path = %q", path)
	}
	if c.Settings.LoggingType != "json" {
		t.Fatalf("parseConfig loggingType = %q", c.Settings.LoggingType)
	}

	if _, err := c.parseConfig([]byte("settings: ["), "bad.yml"); !errors.Is(err, ErrYamlSyntax) {
		t.Fatalf("parseConfig expected ErrYamlSyntax, got %v", err)
	}
}

func TestGetConfig(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()

	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte("settings:\n  loggingType: \"json\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Config{}
	path, err := c.getConfig()
	if err != nil {
		t.Fatalf("getConfig: %v", err)
	}
	if path != filepath.Join(dir, "config.yml") {
		t.Fatalf("getConfig path = %q", path)
	}
	if c.Settings.LoggingType != "json" {
		t.Fatalf("getConfig loggingType = %q", c.Settings.LoggingType)
	}

	// Битый YAML в cwd должен вернуть ErrYamlSyntax
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte("settings: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (&Config{}).getConfig(); !errors.Is(err, ErrYamlSyntax) {
		t.Fatalf("getConfig expected ErrYamlSyntax, got %v", err)
	}
}

// ---------------------------------------- Logging ----------------------------------------

func TestSetupLogging(t *testing.T) {
	old := slog.Default()
	defer slog.SetDefault(old)

	t.Run("text", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "app.log")
		app := &App{loggingPath: path, loggingType: "text"}
		app.setupLogging()
		if app.loggingFile == nil {
			t.Fatal("setupLogging text did not open log file")
		}
		defer func() { _ = app.loggingFile.Close() }()
		slog.Info("hello-log")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		assertContains(t, string(data), "hello-log")
	})

	t.Run("json", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "app.log")
		app := &App{loggingPath: path, loggingType: "json"}
		app.setupLogging()
		if app.loggingFile == nil {
			t.Fatal("setupLogging json did not open log file")
		}
		defer func() { _ = app.loggingFile.Close() }()
		slog.Info("hello-log")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		assertContains(t, string(data), `"msg":"hello-log"`)
	})

	t.Run("error", func(t *testing.T) {
		app := &App{loggingPath: filepath.Join(t.TempDir(), "missing", "dir", "app.log"), loggingType: "text"}
		app.setupLogging()
		if app.loggingFile != nil {
			t.Fatal("setupLogging error must leave loggingFile nil")
		}
	})
}

// ---------------------------------------- Audit ----------------------------------------

func TestShowAudit(t *testing.T) {
	app := newTestApp()
	app.testMode = true
	app.systemDisk = "C"

	output := captureStdout(t, func() {
		app.showAudit()
	})

	assertContains(t, output, "system:")
	assertContains(t, output, "containerizationSystem:")
	if runtime.GOOS == "windows" {
		assertContains(t, output, "winEvent:")
	} else {
		assertContains(t, output, "journald:")
		assertContains(t, output, "auditd:")
	}
}
