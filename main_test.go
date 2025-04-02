package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
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
	file.WriteString("## Windows File Logs\n")
	file.WriteString("| Path | Lines | Read | Color |\n")
	file.WriteString("|------|-------|------|-------|\n")

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
				colorMode:        true,
				tailSpinMode:     false,
				logViewCount:     "100000",
				logUpdateSeconds: 5,
				getOS:            "windows",
				// Режим и текст для фильтрации
				selectFilterMode: "fuzzy",
				filterText:       "",
				// Инициализируем переменные с регулярными выражениями
				trimHttpRegex:        trimHttpRegex,
				trimHttpsRegex:       trimHttpsRegex,
				trimPrefixPathRegex:  trimPrefixPathRegex,
				trimPostfixPathRegex: trimPostfixPathRegex,
				hexByteRegex:         hexByteRegex,
				dateTimeRegex:        dateTimeRegex,
				timeMacAddressRegex:  timeMacAddressRegex,
				dateIpAddressRegex:   dateIpAddressRegex,
				ipAddressRegex:       ipAddressRegex,
				procRegex:            procRegex,
				integersInputRegex:   integersInputRegex,
				syslogUnitRegex:      syslogUnitRegex,
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
				file.WriteString(fmt.Sprintf("| %s | %d | %s | %s |\n", app.lastLogPath, len(app.currentLogLines), endTime, endTime2))
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
	file.WriteString("| Event Name | Lines | Read | Color |\n")
	file.WriteString("|------------|-------|------|-------|\n")

	app := &App{
		testMode:             true,
		colorMode:            true,
		tailSpinMode:         false,
		logViewCount:         "100000",
		logUpdateSeconds:     5,
		getOS:                "windows",
		systemDisk:           "C",
		userName:             "lifailon",
		selectFilterMode:     "fuzzy",
		filterText:           "",
		trimHttpRegex:        trimHttpRegex,
		trimHttpsRegex:       trimHttpsRegex,
		trimPrefixPathRegex:  trimPrefixPathRegex,
		trimPostfixPathRegex: trimPostfixPathRegex,
		hexByteRegex:         hexByteRegex,
		dateTimeRegex:        dateTimeRegex,
		timeMacAddressRegex:  timeMacAddressRegex,
		dateIpAddressRegex:   dateIpAddressRegex,
		ipAddressRegex:       ipAddressRegex,
		procRegex:            procRegex,
		integersInputRegex:   integersInputRegex,
		syslogUnitRegex:      syslogUnitRegex,
	}

	app.loadWinEvents()
	if len(app.journals) == 0 {
		t.Errorf("File list is null")
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

		file.WriteString(fmt.Sprintf("| %s | %d | %s | %s |\n", serviceName, len(app.currentLogLines), endTime, endTime2))
	}
}

func TestUnixFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skip Linux test")
	}

	file, _ := os.OpenFile("test-report.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()
	file.WriteString("## Unix File Logs\n")
	file.WriteString("| Path | Lines | Read | Color |\n")
	file.WriteString("|------|-------|------|-------|\n")

	testCases := []struct {
		name       string
		selectPath string
	}{
		{"System var logs", "/var/log/"},
		{"Optional package logs", "/opt/"},
		{"Users home logs", "/home/"},
		{"Process descriptor logs", "descriptor"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := &App{
				selectPath:           tc.selectPath,
				testMode:             true,
				colorMode:            true,
				tailSpinMode:         false,
				logViewCount:         "100000",
				logUpdateSeconds:     5,
				getOS:                "linux",
				userName:             "lifailon",
				selectFilterMode:     "fuzzy",
				filterText:           "",
				trimHttpRegex:        trimHttpRegex,
				trimHttpsRegex:       trimHttpsRegex,
				trimPrefixPathRegex:  trimPrefixPathRegex,
				trimPostfixPathRegex: trimPostfixPathRegex,
				hexByteRegex:         hexByteRegex,
				dateTimeRegex:        dateTimeRegex,
				timeMacAddressRegex:  timeMacAddressRegex,
				dateIpAddressRegex:   dateIpAddressRegex,
				ipAddressRegex:       ipAddressRegex,
				procRegex:            procRegex,
				integersInputRegex:   integersInputRegex,
				syslogUnitRegex:      syslogUnitRegex,
			}

			// Пропускаем тесты в macOS/BSD
			if runtime.GOOS != "linux" && tc.selectPath != "/var/log/" {
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

				file.WriteString(fmt.Sprintf("| %s | %d | %s | %s |\n", app.lastLogPath, len(app.currentLogLines), endTime, endTime2))
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
	file.WriteString("## Linux journals\n")
	file.WriteString("| Journal Name | Lines | Read | Color |\n")
	file.WriteString("|--------------|-------|------|-------|\n")

	testCases := []struct {
		name        string
		journalName string
	}{
		{"Unit list", "services"},
		{"System journals", "UNIT"},
		{"User journals", "USER_UNIT"},
		{"Kernel boot", "kernel"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := &App{
				selectUnits:          tc.journalName,
				testMode:             true,
				colorMode:            true,
				tailSpinMode:         false,
				logViewCount:         "100000",
				logUpdateSeconds:     5,
				getOS:                "linux",
				selectFilterMode:     "fuzzy",
				filterText:           "",
				trimHttpRegex:        trimHttpRegex,
				trimHttpsRegex:       trimHttpsRegex,
				trimPrefixPathRegex:  trimPrefixPathRegex,
				trimPostfixPathRegex: trimPostfixPathRegex,
				hexByteRegex:         hexByteRegex,
				dateTimeRegex:        dateTimeRegex,
				timeMacAddressRegex:  timeMacAddressRegex,
				dateIpAddressRegex:   dateIpAddressRegex,
				ipAddressRegex:       ipAddressRegex,
				procRegex:            procRegex,
				integersInputRegex:   integersInputRegex,
				syslogUnitRegex:      syslogUnitRegex,
			}

			app.loadServices(app.selectUnits)
			if len(app.journals) == 0 {
				t.Errorf("File list is null")
			} else {
				t.Log("Journal count:", len(app.journals))
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

				file.WriteString(fmt.Sprintf("| %s | %d | %s | %s |\n", serviceName, len(app.currentLogLines), endTime, endTime2))
			}
		})
	}
}

func TestDockerContainer(t *testing.T) {
	file, _ := os.OpenFile("test-report.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()
	file.WriteString("## Containers\n")
	file.WriteString("| Container Name | Lines | Read | Color |\n")
	file.WriteString("|----------------|-------|------|-------|\n")

	testCases := []struct {
		name                         string
		selectContainerizationSystem string
	}{
		{"Docker", "docker"},
		// {"Podman", "podman"},
		// {"Kubernetes", "kubectl"},
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
				colorMode:                    true,
				tailSpinMode:                 false,
				logViewCount:                 "100000",
				logUpdateSeconds:             5,
				selectFilterMode:             "fuzzy",
				filterText:                   "",
				trimHttpRegex:                trimHttpRegex,
				trimHttpsRegex:               trimHttpsRegex,
				trimPrefixPathRegex:          trimPrefixPathRegex,
				trimPostfixPathRegex:         trimPostfixPathRegex,
				hexByteRegex:                 hexByteRegex,
				dateTimeRegex:                dateTimeRegex,
				timeMacAddressRegex:          timeMacAddressRegex,
				dateIpAddressRegex:           dateIpAddressRegex,
				ipAddressRegex:               ipAddressRegex,
				procRegex:                    procRegex,
				integersInputRegex:           integersInputRegex,
				syslogUnitRegex:              syslogUnitRegex,
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

				file.WriteString(fmt.Sprintf("| %s | %d | %s | %s |\n", containerName, len(app.currentLogLines), endTime, endTime2))
			}
		})
	}
}

func TestColor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skip unix test")
	}

	app := &App{
		testMode:             true,
		colorMode:            true,
		tailSpinMode:         false,
		logViewCount:         "100000",
		logUpdateSeconds:     5,
		selectPath:           "/home/",
		filterListText:       "color",
		trimHttpRegex:        trimHttpRegex,
		trimHttpsRegex:       trimHttpsRegex,
		trimPrefixPathRegex:  trimPrefixPathRegex,
		trimPostfixPathRegex: trimPostfixPathRegex,
		hexByteRegex:         hexByteRegex,
		dateTimeRegex:        dateTimeRegex,
		timeMacAddressRegex:  timeMacAddressRegex,
		dateIpAddressRegex:   dateIpAddressRegex,
		ipAddressRegex:       ipAddressRegex,
		procRegex:            procRegex,
		integersInputRegex:   integersInputRegex,
		syslogUnitRegex:      syslogUnitRegex,
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
	files, _ := os.ReadDir("/")
	for _, file := range files {
		if file.IsDir() {
			app.rootDirArray = append(app.rootDirArray, "/"+file.Name())
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

func TestTailSpinColor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skip unix test")
	}

	app := &App{
		testMode:         true,
		colorMode:        true,
		tailSpinMode:     true,
		logViewCount:     "100000",
		logUpdateSeconds: 5,
		selectPath:       "/home/",
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
				testMode:             true,
				colorMode:            true,
				tailSpinMode:         false,
				logViewCount:         "100000",
				logUpdateSeconds:     5,
				selectPath:           "/home/",
				filterListText:       "color",
				selectFilterMode:     tc.selectFilterMode,
				filterText:           "true",
				trimHttpRegex:        trimHttpRegex,
				trimHttpsRegex:       trimHttpsRegex,
				trimPrefixPathRegex:  trimPrefixPathRegex,
				trimPostfixPathRegex: trimPostfixPathRegex,
				hexByteRegex:         hexByteRegex,
				dateTimeRegex:        dateTimeRegex,
				timeMacAddressRegex:  timeMacAddressRegex,
				dateIpAddressRegex:   dateIpAddressRegex,
				ipAddressRegex:       ipAddressRegex,
				procRegex:            procRegex,
				integersInputRegex:   integersInputRegex,
				syslogUnitRegex:      syslogUnitRegex,
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
			files, _ := os.ReadDir("/")
			for _, file := range files {
				if file.IsDir() {
					app.rootDirArray = append(app.rootDirArray, "/"+file.Name())
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
	showHelp()
	app.showAudit()
}

func TestCommandColor(t *testing.T) {
	app := &App{
		testMode:                     false,
		colorMode:                    true,
		tailSpinMode:                 false,
		startServices:                0,
		selectedJournal:              0,
		startFiles:                   0,
		selectedFile:                 0,
		startDockerContainers:        0,
		selectedDockerContainer:      0,
		selectUnits:                  "services",
		selectPath:                   "/var/log/",
		selectContainerizationSystem: "docker",
		selectFilterMode:             "default",
		logViewCount:                 "100000",
		logUpdateSeconds:             5,
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
		timeMacAddressRegex:          timeMacAddressRegex,
		dateIpAddressRegex:           dateIpAddressRegex,
		ipAddressRegex:               ipAddressRegex,
		procRegex:                    procRegex,
		integersInputRegex:           integersInputRegex,
		syslogUnitRegex:              syslogUnitRegex,
		keybindingsEnabled:           true,
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
	// Список корневых каталогов (ls -d /*/) с приставкой "/"
	files, _ := os.ReadDir("/")
	for _, file := range files {
		if file.IsDir() {
			app.rootDirArray = append(app.rootDirArray, "/"+file.Name())
		}
	}

	app.commandLineColor()
	// Восстанавливаем оригинальный stdin
	os.Stdin = originalStdin

}

func TestMainInterface(t *testing.T) {
	go runGoCui(true)
	time.Sleep(3 * time.Second)
}

func TestMockInterface(t *testing.T) {
	app := &App{
		testMode:                     false,
		colorMode:                    true,
		tailSpinMode:                 false,
		startServices:                0,
		selectedJournal:              0,
		startFiles:                   0,
		selectedFile:                 0,
		startDockerContainers:        0,
		selectedDockerContainer:      0,
		selectUnits:                  "services",
		selectPath:                   "/var/log/",
		selectContainerizationSystem: "docker",
		selectFilterMode:             "default",
		logViewCount:                 "100000",
		logUpdateSeconds:             5,
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
		timeMacAddressRegex:          timeMacAddressRegex,
		dateIpAddressRegex:           dateIpAddressRegex,
		ipAddressRegex:               ipAddressRegex,
		procRegex:                    procRegex,
		integersInputRegex:           integersInputRegex,
		syslogUnitRegex:              syslogUnitRegex,
		keybindingsEnabled:           true,
	}

	app.getOS = runtime.GOOS
	app.getArch = runtime.GOARCH

	var err error
	// Отключение tcell для CI
	g, err = gocui.NewGui(gocui.OutputSimulator, true)
	// Включить отображение интерфейса
	// g, err = gocui.NewGui(gocui.OutputNormal, true)
	if err != nil {
		log.Panicln(err)
	}
	// defer g.Close()

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
	files, _ := os.ReadDir("/")
	for _, file := range files {
		if file.IsDir() {
			app.rootDirArray = append(app.rootDirArray, file.Name())
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
		v.Title = " < Windows Event Logs (0) > "
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
		app.updateLogBack(app.secondsChan, false)
	}()

	go func() {
		app.updateWindowSize(1)
	}()

	// Отображение GUI в режиме OutputNormal
	go g.MainLoop()

	time.Sleep(3 * time.Second)

	// F1
	app.showInterfaceHelp(g)
	app.closeHelp(g)

	// Проверяем покраску
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
		"1.0, 1.0.7, 1.0-build",
		"20.03.2025, 2025.03.20, 2025-03-20, 20-03-2025",
		"20/03/2025, 2025/03/20",
		"11:11, 11:11:11",
		"11:11:11:11:11:11, 11-11-11-11-11-11",
		"10%, 50%, 100%",
		"1, 10, 100, (1), [10], {100}",
	}
	app.updateDelimiter(true)
	app.applyFilter(true)
	time.Sleep(3 * time.Second)
	t.Log("PASS: test coloring")

	// Проверяем фильтрацию текста для списков
	app.filterListText = "a"
	app.createFilterEditor("lists")
	time.Sleep(1 * time.Second)
	app.filterListText = ""
	app.applyFilterList()
	time.Sleep(1 * time.Second)
	t.Log("PASS: test filter list")

	// TAB journal
	app.nextView(g, nil)
	time.Sleep(1 * time.Second)
	if v, err := g.View("services"); err == nil {
		// DOWN
		app.nextService(v, 100)
		time.Sleep(1 * time.Second)
		// Загружаем журнал
		app.selectService(g, v)
		time.Sleep(3 * time.Second)
		// UP
		app.prevService(v, 100)
		time.Sleep(1 * time.Second)
		// Переключаем списки
		if runtime.GOOS != "windows" {
			// Right
			app.setUnitListRight(g, v)
			time.Sleep(3 * time.Second)
			t.Log("PASS: UNIT")
			app.setUnitListRight(g, v)
			time.Sleep(3 * time.Second)
			t.Log("PASS: USER_UNIT")
			app.setUnitListRight(g, v)
			time.Sleep(3 * time.Second)
			t.Log("PASS: Kernel")
			app.setUnitListRight(g, v)
			time.Sleep(3 * time.Second)
			t.Log("PASS: Services")
			app.setUnitListLeft(g, v)
			time.Sleep(3 * time.Second)
			t.Log("PASS: Kernel")
			app.setUnitListLeft(g, v)
			time.Sleep(3 * time.Second)
			t.Log("PASS: USER_UNIT")
			app.setUnitListLeft(g, v)
			time.Sleep(3 * time.Second)
			t.Log("PASS: UNIT")
			app.setUnitListLeft(g, v)
			time.Sleep(3 * time.Second)
			t.Log("PASS: Services")
		}
	}
	t.Log("PASS: test journals")

	// TAB filesystem
	app.nextView(g, nil)
	time.Sleep(1 * time.Second)
	if v, err := g.View("varLogs"); err == nil {
		app.nextFileName(v, 100)
		time.Sleep(1 * time.Second)
		app.selectFile(g, v)
		time.Sleep(3 * time.Second)
		app.prevFileName(v, 100)
		time.Sleep(1 * time.Second)
		if runtime.GOOS != "windows" {
			app.setLogFilesListRight(g, v)
			time.Sleep(5 * time.Second)
			t.Log("PASS: /opt/log")
			app.setLogFilesListRight(g, v)
			time.Sleep(10 * time.Second)
			t.Log("PASS: /home")
			app.setLogFilesListRight(g, v)
			time.Sleep(5 * time.Second)
			t.Log("PASS: descriptor")
			app.setLogFilesListRight(g, v)
			time.Sleep(5 * time.Second)
			t.Log("PASS: /var/log")
			app.setLogFilesListLeft(g, v)
			time.Sleep(5 * time.Second)
			t.Log("PASS: descriptor")
			app.setLogFilesListLeft(g, v)
			time.Sleep(5 * time.Second)
			t.Log("PASS: /home")
			app.setLogFilesListLeft(g, v)
			time.Sleep(5 * time.Second)
			t.Log("PASS: /opt/log")
			app.setLogFilesListLeft(g, v)
			time.Sleep(5 * time.Second)
			t.Log("PASS: /var/log")
		}
	}
	t.Log("PASS: test filesystem")

	// TAB docker
	app.nextView(g, nil)
	time.Sleep(1 * time.Second)
	if v, err := g.View("docker"); err == nil {
		app.nextDockerContainer(v, 100)
		time.Sleep(1 * time.Second)
		app.prevDockerContainer(v, 100)
		time.Sleep(1 * time.Second)
		if runtime.GOOS != "windows" {
			app.setContainersListRight(g, v)
			time.Sleep(1 * time.Second)
			t.Log("PASS: Podman")
			app.setContainersListRight(g, v)
			time.Sleep(1 * time.Second)
			t.Log("PASS: Kubernetes")
			app.setContainersListRight(g, v)
			time.Sleep(1 * time.Second)
			t.Log("PASS: Docker")
			app.setContainersListLeft(g, v)
			time.Sleep(1 * time.Second)
			t.Log("PASS: Kubernetes")
			app.setContainersListLeft(g, v)
			time.Sleep(1 * time.Second)
			t.Log("PASS: Podman")
			app.setContainersListLeft(g, v)
			time.Sleep(1 * time.Second)
			t.Log("PASS: Docker")
		}
		time.Sleep(1 * time.Second)
		app.selectDocker(g, v)
		time.Sleep(3 * time.Second)
	}
	t.Log("PASS: test containers")

	// TAB filter logs
	app.nextView(g, nil)

	// Проверяем фильтрацию текста для вывода журнала
	app.filterText = "a"
	app.applyFilter(true)
	time.Sleep(3 * time.Second)
	// Ctrl+W
	app.clearFilterEditor(g)
	app.applyFilter(true)
	time.Sleep(3 * time.Second)
	t.Log("PASS: test filter logs")

	// Проверяем режимы фильтрации
	if v, err := g.View("filter"); err == nil {
		app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Filter fuzzy")
		app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Filter regex")
		app.setFilterModeRight(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Filter default")
		app.setFilterModeLeft(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Filter regex")
		app.setFilterModeLeft(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Filter fuzzy")
		app.setFilterModeLeft(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Filter default")
	}
	t.Log("PASS: test filter modes")

	// TAB logs output
	app.nextView(g, nil)
	time.Sleep(1 * time.Second)
	if v, err := g.View("logs"); err == nil {
		// Alt+Right
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Tail 150000")
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Tail 200000")
		// Alt+Left
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Tail 150000")
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Tail 100000")
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Tail 50000")
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Tail 30000")
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Tail 20000")
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Tail 10000")
		app.setCountLogViewDown(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Tail 5000")
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Tail 10000")
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Tail 20000")
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Tail 30000")
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Tail 50000")
		app.setCountLogViewUp(g, v)
		time.Sleep(1 * time.Second)
		t.Log("PASS: Tail 100000")
		// UP output
		app.scrollUpLogs(1)
		time.Sleep(1 * time.Second)
		// DOWN output
		app.scrollDownLogs(1)
		time.Sleep(1 * time.Second)
		// Ctrl+A
		app.pageUpLogs()
		time.Sleep(1 * time.Second)
		// Ctrl+E
		app.updateLogsView(true)
		time.Sleep(1 * time.Second)
	}
	t.Log("PASS: test log output")

	// TAB filter list
	app.nextView(g, nil)
	time.Sleep(1 * time.Second)

	// Shift+Tab
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
	t.Log("PASS: test Shift+Tab")

	// Select window use mouse
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
	t.Log("PASS: test mouse")

	quit(g, nil)
}
