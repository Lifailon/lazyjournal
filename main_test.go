package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/awesome-gocui/gocui"
)

func TestCreatReport(t *testing.T) {
	file, _ := os.Create("test-report.md")
	defer file.Close()
}

func TestWinFiles(t *testing.T) {
	// Пропускаем тест целиком для Linux/macOS/bsd
	if runtime.GOOS != "windows" {
		t.Skip("Skip Windows test")
	}

	// Создаем файл отчета
	file, _ := os.OpenFile("test-report.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()
	file.WriteString("## Windows file logs\n")
	file.WriteString("| Lines | Read | Color | Path |\n")
	file.WriteString("|-------|------|-------|------|\n")

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
				// Записываем в отчет путь, количество строк в массиве прочитанных из файла, время чтения и фильтрации + покраски
				fmt.Fprintf(file, "| %d | %s | %s | %s |\n", len(app.currentLogLines), endTime, endTime2, app.lastLogPath)
			}
		})
	}
}

func TestWinEvents(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skip Windows test")
	}

	file, _ := os.OpenFile("test-report.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()
	file.WriteString("## Windows Event Logs\n")
	file.WriteString("| Lines | Read | Color | Event Name |\n")
	file.WriteString("|-------|------|-------|------------|\n")

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

		fmt.Fprintf(file, "| %d | %s | %s | %s |\n", len(app.currentLogLines), endTime, endTime2, serviceName)
	}
}

func TestUnixFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skip Linux test")
	}

	file, _ := os.OpenFile("test-report.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()
	file.WriteString("## Unix file logs\n")
	file.WriteString("| Lines | Read | Color | Path |\n")
	file.WriteString("|-------|------|-------|------|\n")

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

				fmt.Fprintf(file, "| %d | %s | %s | %s |\n", len(app.currentLogLines), endTime, endTime2, app.lastLogPath)
			}
		})
	}
}

func TestLinuxJournal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skip Linux test")
	}

	file, _ := os.OpenFile("test-report.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()
	file.WriteString("## System logs\n")
	file.WriteString("| Lines | Read | Color | Journal Name |\n")
	file.WriteString("|-------|------|-------|--------------|\n")

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
				file.WriteString("## Audit rules\n")
				file.WriteString("| Lines | Read | Color | Rules keys   |\n")
				file.WriteString("|-------|------|-------|--------------|\n")
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

				fmt.Fprintf(file, "| %d | %s | %s | %s |\n", len(app.currentLogLines), endTime, endTime2, serviceName)
			}
		})
	}
}

func TestDockerContainer(t *testing.T) {
	file, _ := os.OpenFile("test-report.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()
	file.WriteString("## Container logs\n")
	file.WriteString("| Lines | Read | Color | Container Name |\n")
	file.WriteString("|-------|------|-------|----------------|\n")

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

				fmt.Fprintf(file, "| %d | %s | %s | %s |\n", len(app.currentLogLines), endTime, endTime2, containerName)
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
	defer app.loggingFile.Close()
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
		pw.Close()
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
		pw.Close()
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
		pw.Close()
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
	defer app.loggingFile.Close()
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
			fmt.Fprintln(selectedVarLog, "Searching log files...")
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
	go g.MainLoop()

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
	app.createFilterEditor("lists")
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
	app.timestampFilterEditor("sinceFilter")
	app.timestampFilterEditor("untilFilter")
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
	app.nextView(g, nil)
	time.Sleep(1 * time.Second)

	// Journals (services)
	if v, err := g.View("services"); err == nil {
		// Перемещаемся по списку вниз
		app.nextService(v, 100)
		time.Sleep(1 * time.Second)
		// Загружаем журнал
		app.selectService(g, v)
		time.Sleep(3 * time.Second)
		// Перемещаемся по списку вверх
		app.prevService(v, 100)
		time.Sleep(1 * time.Second)
		// Переключаем списки (только для Linux)
		if runtime.GOOS != "windows" {
			// Вправо
			if debug {
				textLog := "List next (right)"
				t.Log(debugLog + textLog)
				slog.Debug(textLog)
			}
			app.setUnitListRight(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "User units (UNIT)"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setUnitListRight(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "System journals (USER_UNIT)"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setUnitListRight(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "Kernel boot"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setUnitListRight(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "Audit rules keys (auditd)"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setUnitListRight(g, v)
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
			app.setUnitListLeft(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "Audit rules keys (auditd)"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setUnitListLeft(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "Kernel boot"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setUnitListLeft(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "System journals (USER_UNIT)"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setUnitListLeft(g, v)
			time.Sleep(3 * time.Second)
			if debug {
				textLog := "User units (UNIT)"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setUnitListLeft(g, v)
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
	app.nextView(g, nil)
	time.Sleep(1 * time.Second)

	// File System (varLogs)
	if v, err := g.View("varLogs"); err == nil {
		// Перемещаемся по списку вниз
		app.nextFileName(v, 100)
		time.Sleep(1 * time.Second)
		// Загружаем журнал
		app.selectFile(g, v)
		time.Sleep(3 * time.Second)
		// Перемещаемся по списку вверх
		app.prevFileName(v, 100)
		time.Sleep(1 * time.Second)
		if runtime.GOOS != "windows" {
			// Вправо
			if debug {
				textLog := "List next (right)"
				t.Log(debugLog + textLog)
				slog.Debug(textLog)
			}
			app.setLogFilesListRight(g, v)
			time.Sleep(10 * time.Second)
			if debug {
				textLog := "Optional package logs and custom path"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setLogFilesListRight(g, v)
			time.Sleep(10 * time.Second)
			if debug {
				textLog := "Users home logs"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setLogFilesListRight(g, v)
			time.Sleep(10 * time.Second)
			if debug {
				textLog := "Process descriptor logs"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setLogFilesListRight(g, v)
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
			app.setLogFilesListLeft(g, v)
			time.Sleep(10 * time.Second)
			if debug {
				textLog := "Process descriptor logs"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setLogFilesListLeft(g, v)
			time.Sleep(10 * time.Second)
			if debug {
				textLog := "Users home logs"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setLogFilesListLeft(g, v)
			time.Sleep(10 * time.Second)
			if debug {
				textLog := "Optional package logs and custom path"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setLogFilesListLeft(g, v)
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
	app.nextView(g, nil)
	time.Sleep(1 * time.Second)

	// Containerization System (docker)
	if v, err := g.View("docker"); err == nil {
		// Перемещаемся по списку вниз
		app.nextDockerContainer(v, 100)
		time.Sleep(1 * time.Second)
		// Загружаем журнал (ВРЕМЕННО ОТКЛЮЧЕНО)
		app.selectDocker(g, v)
		time.Sleep(3 * time.Second)
		// Перемещаемся по списку вверх
		app.prevDockerContainer(v, 100)
		time.Sleep(1 * time.Second)
		if runtime.GOOS != "windows" {
			// Вправо
			if debug {
				textLog := "List next (right)"
				t.Log(debugLog + textLog)
				slog.Debug(textLog)
			}
			app.setContainersListRight(g, v)
			time.Sleep(2 * time.Second)
			if debug {
				textLog := "Compose"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setContainersListRight(g, v)
			time.Sleep(2 * time.Second)
			if debug {
				textLog := "Podman"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setContainersListRight(g, v)
			time.Sleep(2 * time.Second)
			if debug {
				textLog := "Kubernetes"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setContainersListRight(g, v)
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
			app.setContainersListLeft(g, v)
			time.Sleep(2 * time.Second)
			if debug {
				textLog := "Kubernetes"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setContainersListLeft(g, v)
			time.Sleep(2 * time.Second)
			if debug {
				textLog := "Podman"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setContainersListLeft(g, v)
			time.Sleep(2 * time.Second)
			if debug {
				textLog := "Compose"
				t.Log(passLog + textLog)
				slog.Debug(textLog)
			}
			app.setContainersListLeft(g, v)
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
	app.nextView(g, nil)

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
		app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Filter fuzzy"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Filter regex"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Filter timestamp"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setFilterModeRight(g, v)
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
		app.setFilterModeLeft(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Filter timestamp"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setFilterModeLeft(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Filter regex"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setFilterModeLeft(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Filter fuzzy"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setFilterModeLeft(g, v)
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
	app.nextView(g, nil)
	time.Sleep(1 * time.Second)
	if v, err := g.View("logs"); err == nil {
		// Up tail
		if debug {
			textLog := "Up tail"
			t.Log(debugLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 20K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 30K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 40K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 50K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 100K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 150K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		app.setCountLogViewUp(g, v)
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
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 150K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 100K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 50K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 40K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 30K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 20K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 10K (default)"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 5K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 1K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 500"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		app.setCountLogViewDown(g, v)
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
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 500"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 1K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		if debug {
			textLog := "Tail 5K"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.setCountLogViewUp(g, v)
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
		app.scrollUpLogs(1)
		time.Sleep(1 * time.Second)
		// Up logs output on 10
		if debug {
			textLog := "Up logs output on 10 (Shift+Up)"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.scrollUpLogs(10)
		time.Sleep(1 * time.Second)
		// Up logs output on 500
		if debug {
			textLog := "Up logs output on 500 (Alt+Up)"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.scrollUpLogs(500)
		time.Sleep(1 * time.Second)
		app.scrollUpLogs(500)
		time.Sleep(1 * time.Second)
		// Down logs output on 1
		if debug {
			textLog := "Down logs output on 1"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.scrollDownLogs(1)
		time.Sleep(1 * time.Second)
		// Down logs output on 10
		if debug {
			textLog := "Down logs output on 10 (Shift+Down)"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.scrollDownLogs(10)
		time.Sleep(1 * time.Second)
		// Down logs output on 500
		if debug {
			textLog := "Down logs output on 500 (Alt+Down)"
			t.Log(passLog + textLog)
			slog.Debug(textLog)
		}
		app.scrollDownLogs(500)
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
	app.nextView(g, nil)
	time.Sleep(1 * time.Second)

	// Back Tab
	app.backView(g, nil)
	time.Sleep(1 * time.Second)
	app.backView(g, nil)
	time.Sleep(1 * time.Second)
	app.backView(g, nil)
	time.Sleep(1 * time.Second)
	app.backView(g, nil)
	time.Sleep(1 * time.Second)
	app.backView(g, nil)
	time.Sleep(1 * time.Second)
	app.backView(g, nil)
	time.Sleep(1 * time.Second)
	app.backView(g, nil)
	time.Sleep(1 * time.Second)
	if debug {
		textLog := "test back tab (Shift+Tab)"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// Проверяем переключение окон с помощью мыши
	app.setSelectView(g, "filterList")
	time.Sleep(1 * time.Second)
	app.setSelectView(g, "services")
	time.Sleep(1 * time.Second)
	app.setSelectView(g, "varLogs")
	time.Sleep(1 * time.Second)
	app.setSelectView(g, "docker")
	time.Sleep(1 * time.Second)
	app.setSelectView(g, "filter")
	time.Sleep(1 * time.Second)
	app.setSelectView(g, "logs")
	time.Sleep(1 * time.Second)
	if debug {
		textLog := "test mouse"
		t.Log(passLog + textLog)
		slog.Debug(textLog)
	}

	// Переключаем режим фильтрации на timestamp
	g.SetCurrentView("filter")
	if v, err := g.View("filter"); err == nil {
		app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
		app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
		app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
	}

	// Проверяем переключение окон в режиме timestamp
	app.nextView(g, nil)
	time.Sleep(1 * time.Second)
	app.nextView(g, nil)
	time.Sleep(1 * time.Second)
	app.setSelectView(g, "logs")
	time.Sleep(1 * time.Second)
	app.setSelectView(g, "logs")
	time.Sleep(1 * time.Second)
	app.setSelectView(g, "logs")
	time.Sleep(1 * time.Second)
	app.nextView(g, nil)
	time.Sleep(1 * time.Second)

	quit(g, nil)
}
