package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ANSI color codes for highlighting and clearing the screen.
const (
	Reset       = "\033[0m"
	Red         = "\033[31m"
	Green       = "\033[32m"
	Yellow      = "\033[33m"
	Blue        = "\033[34m"
	Magenta     = "\033[35m"
	Cyan        = "\033[36m"
	ClearScreen = "\033[H\033[2J"
)

// Config holds filtering and multiple highlighting rules.
type Config struct {
	Filter     string
	Highlights map[string]string // Map of words to highlight with their colors
}

// Mutexes for thread-safe access to config and logs.
var configMutex sync.RWMutex
var logsMutex sync.RWMutex

var currentConfig Config
var storedLogs []string
var lastConfigContent string

// highlightText highlights matched keywords using ANSI escape codes.
func highlightText(line string, highlights map[string]string) string {
	for word, color := range highlights {
		re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(word))
		line = re.ReplaceAllString(line, color+word+Reset)
	}
	return line
}

// filterAndHighlight applies the current configuration to format a log line.
func filterAndHighlight(line string) string {
	configMutex.RLock()
	cfg := currentConfig
	configMutex.RUnlock()

	if strings.Contains(strings.ToLower(line), strings.ToLower(cfg.Filter)) {
		return highlightText(line, cfg.Highlights)
	}
	return ""
}

// getColor returns the ANSI color code for a given color name.
func getColor(color string) string {
	switch strings.ToLower(color) {
	case "red":
		return Red
	case "green":
		return Green
	case "yellow":
		return Yellow
	case "blue":
		return Blue
	case "magenta":
		return Magenta
	case "cyan":
		return Cyan
	default:
		return Reset
	}
}

// loadConfig reads the config file and updates the global configuration.
func loadConfig(configPath string) bool {
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error reading config file:", err)
		return false
	}

	// Compare with the last config content to avoid unnecessary reloads.
	newContent := string(content)
	if newContent == lastConfigContent {
		return false
	}
	lastConfigContent = newContent

	newConfig := Config{Highlights: make(map[string]string)}
	scanner := bufio.NewScanner(strings.NewReader(newContent))

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])

		switch key {
		case "filter":
			newConfig.Filter = value
		default:
			// Assume the key is a word to highlight, and value is its color.
			newConfig.Highlights[key] = getColor(value)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing config file:", err)
		return false
	}

	configMutex.Lock()
	currentConfig = newConfig
	configMutex.Unlock()

	return true
}

// reprintLogs clears the terminal and reprints all logs with the current configuration.
func reprintLogs() {
	logsMutex.RLock()
	defer logsMutex.RUnlock()

	fmt.Print(ClearScreen)
	for _, log := range storedLogs {
		if formattedLog := filterAndHighlight(log); formattedLog != "" {
			fmt.Println(formattedLog)
		}
	}
}

// appendLog stores a log line and triggers reprint of all logs.
func appendLog(line string) {
	logsMutex.Lock()
	storedLogs = append(storedLogs, line)
	logsMutex.Unlock()

	reprintLogs()
}

// readLogs continuously reads logs from the input and stores them.
func readLogs(scanner *bufio.Scanner) {
	for scanner.Scan() {
		line := scanner.Text()
		appendLog(line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error reading logs:", err)
	}
}

// pollConfig periodically checks for changes in the configuration file.
func pollConfig(configPath string, interval time.Duration) {
	for {
		if loadConfig(configPath) {
			fmt.Println("Config file reloaded.")
			reprintLogs()
		}
		time.Sleep(interval)
	}
}

func main() {
	// Command-line flags for config and input files.
	configPath := flag.String("config", "config.txt", "Path to the configuration file")
	inputPath := flag.String("input", "", "Path to the input log file (optional)")
	pollInterval := flag.Duration("interval", 2*time.Second, "Polling interval for config file changes")

	flag.Parse()

	// Load the initial configuration.
	loadConfig(*configPath)

	// Start polling the config file for changes.
	go pollConfig(*configPath, *pollInterval)

	// Use standard input or read from a file.
	var scanner *bufio.Scanner
	if *inputPath != "" {
		file, err := os.Open(*inputPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error opening input file:", err)
			os.Exit(1)
		}
		defer file.Close()
		scanner = bufio.NewScanner(file)
	} else {
		scanner = bufio.NewScanner(os.Stdin)
	}

	// Continuously read logs.
	readLogs(scanner)
}
