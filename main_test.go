package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
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
		{"Program Files 86", "ProgramFiles86"},
		{"ProgramData", "ProgramData"},
		{"AppData/Local", "AppDataLocal"},
		{"AppData/Roaming", "AppDataRoaming"},
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

			// Пропускаем тесты для CI
			if app.userName == "runneradmin" && tc.selectPath != "AppDataRoaming" {
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
		{"Users home logs", "home"},
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
				customPath:         "/opt",
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
				journalBoot:        "all",
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

func TestColor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skip unix test")
	}

	app := &App{
		testMode:           true,
		logViewCount:       "10000",
		logUpdateSeconds:   5,
		selectPath:         "home",
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
		t.Errorf("File list is null")
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

func TestExtColor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skip unix test")
	}

	app := &App{
		testMode:         true,
		colorMode:        "tailspin",
		logViewCount:     "10000",
		logUpdateSeconds: 5,
		selectPath:       "home",
		filterListText:   "color",
	}

	app.loadFiles(app.selectPath)
	app.logfilesNotFilter = app.logfiles
	app.applyFilterList()

	if len(app.logfiles) == 0 {
		t.Errorf("File list is null")
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

	app.colorMode = "bat"
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
		{"Default filter mode", "default"},
		{"Fuzzy filter mode", "fuzzy"},
		{"Regex filter mode", "regex"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := &App{
				testMode:           true,
				logViewCount:       "10000",
				logUpdateSeconds:   5,
				selectPath:         "home",
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
				t.Errorf("File list is null")
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
		selectFilterMode:             "default",
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
		selectFilterMode:             "default",
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
		selectFilterMode:             "default",
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
		selectFilterMode:             "default",
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
				textLog := "Optional package logs and custom path"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setLogFilesListRight(g, v)
			time.Sleep(10 * time.Second)
			if debug {
				textLog := "Users home logs"
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
				textLog := "Users home logs"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			_ = app.setLogFilesListLeft(g, v)
			time.Sleep(10 * time.Second)
			if debug {
				textLog := "Optional package logs and custom path"
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
			textLog := "Filter default"
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
			textLog := "Filter default"
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
		selectFilterMode:             "default",
		logViewCount:                 "10000",
		unitType:                     "service",
		colorMode:                    "disable",
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
		journalBoot:                  "all",
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

func TestRemoveANSI(t *testing.T) {
	input := "\033[31merror\033[0m \033[1mbold\033[0m"
	if got := removeANSI(input); got != "error bold" {
		t.Fatalf("unexpected removeANSI result: %q", got)
	}
}

func TestParseDateFromNameShort(t *testing.T) {
	if got := parseDateFromName("short"); !got.IsZero() {
		t.Fatalf("expected zero time for short name, got %v", got)
	}
	name := strings.Repeat("x", 22) + "01.02.2024 03:04:05"
	want, err := time.Parse("02.01.2006 15:04:05", "01.02.2024 03:04:05")
	if err != nil {
		t.Fatal(err)
	}
	if got := parseDateFromName(name); !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestShellQuote(t *testing.T) {
	tests := map[string]string{
		"":            "''",
		"/var/log":    "'/var/log'",
		"it's a path": "'it'\"'\"'s a path'",
	}
	for input, want := range tests {
		if got := shellQuote(input); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSSHBuildArgs(t *testing.T) {
	options := []string{"-o", "BatchMode=yes", "root@example.com"}
	optionsSnapshot := strings.Join(options, " ")
	parts := []string{"tail", "-n", "10", "/var/log/my app's $HOME.log"}
	got := sshBuildArgs(options, parts...)
	want := []string{
		"-o", "BatchMode=yes", "root@example.com",
		"'tail' '-n' '10' '/var/log/my app'\"'\"'s $HOME.log'",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("sshBuildArgs() = %q, want %q", got, want)
	}
	if strings.Join(options, " ") != optionsSnapshot {
		t.Fatalf("sshBuildArgs mutated options: %q", options)
	}
}

func TestSSHCommandBuilders(t *testing.T) {
	app := &App{sshOptions: []string{"-o", "BatchMode=yes", "root@example.com"}}
	parts := []string{"find", "/var/log/my app", "-name", "*.log"}
	want := append([]string{"ssh"}, sshBuildArgs(app.sshOptions, parts...)...)
	wantLine := strings.Join(want, "\n")

	cmd := app.sshCommand(parts...)
	if got := strings.Join(cmd.Args, "\n"); got != wantLine {
		t.Errorf("sshCommand args = %q, want %q", got, wantLine)
	}

	cmdCtx := app.sshCommandContext(context.Background(), parts...)
	if got := strings.Join(cmdCtx.Args, "\n"); got != wantLine {
		t.Errorf("sshCommandContext args = %q, want %q", got, wantLine)
	}
}

func TestFindLogFilesArgs(t *testing.T) {
	app := &App{}
	cmd := app.findLogFiles([]string{"/var/log"}, []string{"*.log", "*.pcap.gz"})
	want := []string{"find", "/var/log", "-type", "f", "(", "-name", "*.log", "-o", "-name", "*.pcap.gz", ")"}
	if got := strings.Join(cmd.Args, " "); got != strings.Join(want, " ") {
		t.Fatalf("unexpected local find args: %q", got)
	}

	app.sshMode = true
	app.sshOptions = []string{"-o", "BatchMode=yes", "root@example.com"}
	cmd = app.findLogFiles([]string{"/var/log"}, []string{"*.log", "*.pcap.gz"})
	if cmd.Args[0] != "ssh" {
		t.Fatalf("expected ssh command, got %q", cmd.Args[0])
	}
	remote := cmd.Args[len(cmd.Args)-1]
	wantRemote := "'find' '/var/log' '-type' 'f' '(' '-name' '*.log' '-o' '-name' '*.pcap.gz' ')'"
	if remote != wantRemote {
		t.Fatalf("unexpected remote find command: %q", remote)
	}
}

func TestHasLogSuffix(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"file.log", true},
		{"file.LOG", true},
		{".log", true},
		{"file.log.gz", false},
		{"log", false},
		{"", false},
	}
	for _, tc := range tests {
		if got := hasLogSuffix(tc.name); got != tc.want {
			t.Errorf("hasLogSuffix(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestCloneStringMap(t *testing.T) {
	if got := cloneStringMap(nil); got != nil {
		t.Fatalf("expected nil clone, got %v", got)
	}
	source := map[string]string{"a": "1"}
	clone := cloneStringMap(source)
	clone["a"] = "2"
	if source["a"] != "1" {
		t.Fatal("clone shares storage with source")
	}
}

func TestTailBytes(t *testing.T) {
	input := []byte("1\n2\n3\n4\n")
	if got := string(tailBytes(input, 2)); got != "3\n4\n" {
		t.Fatalf("unexpected tail with trailing newline: %q", got)
	}
	if got := string(tailBytes([]byte("1\n2\n3\n4"), 2)); got != "3\n4" {
		t.Fatalf("unexpected tail without trailing newline: %q", got)
	}
	if got := tailBytes(input, 0); got != nil {
		t.Fatalf("expected nil for zero limit, got %q", got)
	}
}

func TestReadLastLinesExactCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tail.log")
	var builder strings.Builder
	for i := 1; i <= 1000; i++ {
		_, _ = fmt.Fprintf(&builder, "%d\n", i)
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0644); err != nil {
		t.Fatal(err)
	}
	output, err := readLastLines(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if len(lines) != 10 || lines[0] != "991" || lines[9] != "1000" {
		t.Fatalf("unexpected tail: %q", lines)
	}
}

// Проверяем, что первая строка не обрезается на границе блока в 1 МБ
func TestReadLastLinesLargeChunkBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large-tail.log")
	prefix := "old line\nPARTIAL"
	suffix := strings.Repeat("x", 1024*1024-len("\nA\n")) + "\nA\n"
	if err := os.WriteFile(path, []byte(prefix+suffix), 0644); err != nil {
		t.Fatal(err)
	}
	output, err := readLastLines(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "PARTIAL") || lines[1] != "A" {
		t.Fatalf("unexpected tail at chunk boundary: %q", lines)
	}
}

func TestLoadWinFileLogTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "win.log")
	var builder strings.Builder
	for i := 1; i <= 2500; i++ {
		_, _ = fmt.Fprintf(&builder, "line %d\n", i)
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0644); err != nil {
		t.Fatal(err)
	}
	app := &App{logViewCount: "7", testMode: true}
	output, stringErrors := app.loadWinFileLog(path)
	if stringErrors != "nil" {
		t.Fatalf("unexpected error: %s", stringErrors)
	}
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if len(lines) != 7 || lines[0] != "line 2494" || lines[6] != "line 2500" {
		t.Fatalf("unexpected tail: %q", lines)
	}
}

// Проверяем, что loadWinFileLog не обрезает первую строку на границе чанка
func TestLoadWinFileLogLargeChunkBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large-win.log")
	prefix := "old line\nPARTIAL"
	suffix := strings.Repeat("x", 1024*1024-len("\nA\n")) + "\nA\n"
	if err := os.WriteFile(path, []byte(prefix+suffix), 0644); err != nil {
		t.Fatal(err)
	}
	app := &App{logViewCount: "2", testMode: true}
	output, stringErrors := app.loadWinFileLog(path)
	if stringErrors != "nil" {
		t.Fatalf("unexpected error: %s", stringErrors)
	}
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "PARTIAL") || lines[1] != "A" {
		t.Fatalf("unexpected tail at chunk boundary: %q", lines)
	}
}

func BenchmarkTailBytes(b *testing.B) {
	input := bytes.Repeat([]byte("2026-01-01 00:00:00 message\n"), 50000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tailBytes(input, 1000)
	}
}

func BenchmarkReadLastLines(b *testing.B) {
	path := filepath.Join(b.TempDir(), "tail.log")
	var builder strings.Builder
	for i := 1; i <= 100000; i++ {
		_, _ = fmt.Fprintf(&builder, "%d\n", i)
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0644); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := readLastLines(path, 1000); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadWinFileLog(b *testing.B) {
	path := filepath.Join(b.TempDir(), "win.log")
	var builder strings.Builder
	for i := 1; i <= 100000; i++ {
		_, _ = fmt.Fprintf(&builder, "line %d\n", i)
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0644); err != nil {
		b.Fatal(err)
	}
	app := &App{logViewCount: "1000", testMode: true}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := app.loadWinFileLog(path); err != "nil" {
			b.Fatal(err)
		}
	}
}

func TestUpdateDelimiterHeadless(t *testing.T) {
	app := &App{
		testMode:        true,
		headless:        true,
		windowWidth:     80,
		lastUpdateLine:  "line 2",
		updateTime:      "12:00:00",
		currentLogLines: []string{"line 1", "line 2", "line 3", "line 4"},
	}
	app.updateDelimiter(false)
	if len(app.currentLogLines) != 5 {
		t.Fatalf("expected delimiter insertion, got %d lines", len(app.currentLogLines))
	}
	if !strings.Contains(app.currentLogLines[2], "12:00:00") {
		t.Fatalf("delimiter not inserted at expected position: %q", app.currentLogLines)
	}
	app.windowWidth = 10
	app.currentLogLines = []string{"line 1", "line 2", "line 3", "line 4"}
	app.updateDelimiter(false)
	if len(app.currentLogLines) != 5 {
		t.Fatalf("expected delimiter insertion for narrow view, got %d lines", len(app.currentLogLines))
	}
}

// Проверяем, что ошибка headless-загрузки файла отменяет коммит результата
func TestLoadFileLogsHeadlessError(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.log")
	app := &App{
		testMode:     true,
		headless:     true,
		logViewCount: "10",
		logfiles:     []Logfile{{name: "missing.log", path: missingPath}},
	}
	app.loadFileLogs("missing.log", true)
	if !app.loadFailed {
		t.Fatal("expected loadFailed after file stat error")
	}
}

// Проверяем, что ошибка чтения json-файла Docker переключает загрузку на docker logs
func TestLoadDockerLogsFallsBackToStream(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skip Unix shell script test")
	}
	binDir := t.TempDir()
	dockerScript := filepath.Join(binDir, "docker")
	script := `#!/bin/sh
case "$1" in
  inspect) echo /nonexistent/lazyjournal-test.log; exit 0;;
  *) echo "2024-01-02T03:04:05.000000000Z test log line"; exit 0;;
esac
`
	if err := os.WriteFile(dockerScript, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	app := &App{
		testMode:                     true,
		headless:                     true,
		selectContainerizationSystem: "docker",
		dockerContext:                "default",
		dockerStreamLogs:             false,
		dockerStreamLogsStatus:       "json-file",
		dockerStreamMode:             "stdout",
		logViewCount:                 "10",
		filterText:                   "",
		colorMode:                    "disable",
		dockerContainers: []DockerContainers{{
			name:    "test-container",
			rawName: "test-container",
			id:      "container-id",
		}},
	}

	app.loadDockerLogs("test-container", true)

	if app.loadFailed {
		t.Fatalf("expected fallback to stream, got load error: %s", app.loadError)
	}
	if !app.dockerStreamLogs || app.dockerStreamLogsStatus != "stdout" {
		t.Fatalf("expected stream fallback, got stream=%t status=%s", app.dockerStreamLogs, app.dockerStreamLogsStatus)
	}
	found := false
	for _, line := range app.currentLogLines {
		if strings.Contains(line, "test log line") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected docker logs output, got %q", app.currentLogLines)
	}
}

// Проверяем, что ошибка загрузки списка служб сохраняется для headless-коммита
func TestLoadServicesHeadlessError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skip Unix shell script test")
	}
	binDir := t.TempDir()
	journalctlScript := filepath.Join(binDir, "journalctl")
	script := "#!/bin/sh\necho 'journalctl: not found' >&2\nexit 1\n"
	if err := os.WriteFile(journalctlScript, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	app := &App{testMode: true, headless: true}
	app.loadServices("systemUnits")

	if app.serviceListError == "" {
		t.Fatal("expected service list error for headless commit")
	}
	if len(app.journals) != 0 {
		t.Fatalf("expected empty service list, got %d entries", len(app.journals))
	}
}

// Проверяем, что при ошибке версии Docker список не загружается вторым вызовом
func TestLoadDockerContainerVersionErrorStops(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skip Unix shell script test")
	}
	binDir := t.TempDir()
	dockerScript := filepath.Join(binDir, "docker")
	script := `#!/bin/sh
case "$1" in
  version) echo "docker: not found" >&2; exit 1;;
  *) echo "container-id test-container running"; exit 0;;
esac
`
	if err := os.WriteFile(dockerScript, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	app := &App{testMode: true, headless: true, dockerContext: "default"}
	app.loadDockerContainer("docker")

	if app.dockerListError == "" {
		t.Fatal("expected docker list error")
	}
	if len(app.dockerContainers) != 0 {
		t.Fatalf("expected no containers after version error, got %d", len(app.dockerContainers))
	}
}

func TestApplyFilterHeadless(t *testing.T) {
	app := &App{
		testMode:         true,
		headless:         true,
		selectFilterMode: "fuzzy",
		minSymbolFilter:  1,
		filterText:       "error",
		colorMode:        "disable",
		currentLogLines:  []string{"info line", "error line", "warning line"},
	}
	app.applyFilter(false)
	found := false
	for _, line := range app.filteredLogLines {
		if strings.Contains(removeANSI(line), "error line") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("headless filter did not return matching line: %q", app.filteredLogLines)
	}
	if app.filterDirty {
		t.Fatal("filterDirty must be reset after successful headless filter")
	}
}
