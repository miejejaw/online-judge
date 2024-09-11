package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Submission struct {
	Language      string `json:"language"`
	Code          string `json:"code"`
	Input         string `json:"input"`
	CPUTimeLimit  int    `json:"cpuTimeLimit"`  // CPU time limit in seconds
	WallTimeLimit int    `json:"wallTimeLimit"` // Wall clock time limit in seconds
	MemoryLimit   int    `json:"memoryLimit"`   // Memory limit in KB
}

type ExecutionResult struct {
	Message       string `json:"message"`
	Status        string `json:"status"` // Exit code of the program
	Stdout        string `json:"stdout"`
	Stderr        string `json:"stderr"`
	CompileOutput string `json:"compile_output"`
	Time          string `json:"time"`
	Memory        int    `json:"memory"`
}

func main() {
	r := gin.Default()

	// API route for code submission
	r.POST("/submit", func(c *gin.Context) {
		var submission Submission
		if err := c.ShouldBindJSON(&submission); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body", "status": "error"})
			return
		}

		// Create unique file for the submitted code
		filename, err := saveCode(submission.Language, submission.Code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save code", "status": "error"})
			return
		}

		// Run the code inside the Isolate sandbox with input and resource limits
		result, err := runCodeInIsolate(
			submission.Language, filename, submission.Input,
			submission.CPUTimeLimit, submission.WallTimeLimit, submission.MemoryLimit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error(), "status": "error"})
			return
		}

		// Return the result of the execution
		c.JSON(http.StatusOK, result)
	})

	err := r.Run()
	if err != nil {
		return
	} // Default to :8080
}

// saveCode saves the submitted code to a file
func saveCode(language, code string) (string, error) {
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("internal/submissions/code_%d.%s", timestamp, getExtension(language))
	err := os.WriteFile(filename, []byte(code), 0644)
	if err != nil {
		return "", err
	}
	return filename, nil
}

// getExtension returns file extension based on language
func getExtension(language string) string {
	switch language {
	case "c":
		return "c"
	case "cpp":
		return "cpp"
	case "python":
		return "py"
	case "java":
		return "java"
	default:
		return "txt"
	}
}

func runCodeInIsolate(language, filename, input string, cpuTimeLimit, wallTimeLimit, memoryLimit int) (ExecutionResult, error) {
	// Initialize the result struct
	result := ExecutionResult{
		Message:       "",
		Stdout:        "",
		Stderr:        "",
		CompileOutput: "",
		Time:          "",
		Memory:        0,
	}

	// Isolate boxId (you can manage different boxes for users if needed)
	boxId := "0"

	// Initialize the isolate box
	initCmd := exec.Command("isolate", "--init", "--box-id", boxId)
	if err := initCmd.Run(); err != nil {
		return ExecutionResult{}, fmt.Errorf("failed to initialize sandbox: %v", err)
	}

	// Defer cleanup to remove the box after execution
	defer func() {
		cleanupCmd := exec.Command("isolate", "--cleanup", "--box-id", boxId)
		_ = cleanupCmd.Run() // Cleanup errors can be ignored
	}()

	// Path to the sandbox
	boxPath := fmt.Sprintf("/var/local/lib/isolate/%s/box", boxId)

	// Move the code file to the box
	codeFilePath := fmt.Sprintf("%s/%s", boxPath, filepath.Base(filename))
	if err := exec.Command("cp", filename, codeFilePath).Run(); err != nil {
		return ExecutionResult{}, fmt.Errorf("failed to copy code to sandbox: %v", err)
	}

	// Create a path for the metadata file to capture execution statistics
	metaFilePath := fmt.Sprintf("/var/local/lib/isolate/%s/meta", boxId)

	// Base isolate command with time and memory limits
	command := []string{
		"isolate",
		"--run",
		"--box-id", boxId,
		"--time", fmt.Sprintf("%d", cpuTimeLimit), // CPU time limit
		"--wall-time", fmt.Sprintf("%d", wallTimeLimit), // Real-time limit
		"--mem", fmt.Sprintf("%d", memoryLimit), // Memory limit in KB
		"--meta", metaFilePath, // Metadata file for stats
		"--"}

	var compileOutput bytes.Buffer

	// Add language-specific commands
	switch language {
	case "c":
		// Compile C program
		cmd := exec.Command("gcc", codeFilePath, "-o", filepath.Join(boxPath, "a.out"))
		cmd.Stderr = &compileOutput
		if err := cmd.Run(); err != nil {
			result.Status = "compilation_error"
			result.CompileOutput = compileOutput.String()
			return result, nil
		}
		command = append(command, "./a.out")
	case "cpp":
		// Compile C++ program
		cmd := exec.Command("g++", codeFilePath, "-o", filepath.Join(boxPath, "a.out"))
		cmd.Stderr = &compileOutput
		if err := cmd.Run(); err != nil {
			result.Status = "compilation_error"
			result.CompileOutput = compileOutput.String()
			return result, nil
		}
		command = append(command, "./a.out")
	case "python":
		// Run Python script directly
		command = append(command, "/usr/bin/python3", filepath.Base(codeFilePath))
	case "java":
		// Compile and run Java program
		cmd := exec.Command("javac", codeFilePath)
		cmd.Stderr = &compileOutput
		if err := cmd.Run(); err != nil {
			result.Status = "compilation_error"
			result.CompileOutput = compileOutput.String()
			return result, nil
		}
		command = append(command, "/usr/bin/java", "Main") // Assuming the main class is "Main"
	default:
		return ExecutionResult{}, fmt.Errorf("unsupported language")
	}

	// Prepare to execute the code with input and limits
	cmd := exec.Command(command[0], command[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = bytes.NewBufferString(input) // Pass input to the program's stdin

	// Run the program inside Isolate
	if err := cmd.Run(); err != nil {
		// Return detailed error with stderr
		result.Stderr = stderr.String()
		return result, nil
	}

	// Read the Isolate metadata file to get memory and time usage
	metaData, err := os.ReadFile(metaFilePath)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("failed to read metadata file: %v", err)
	}

	// Parse the metadata file
	var maxMemory int
	var maxTime string
	//var status string
	lines := bytes.Split(metaData, []byte("\n"))
	for _, line := range lines {
		parts := bytes.SplitN(line, []byte(":"), 2)
		if len(parts) != 2 {
			continue
		}

		key := string(parts[0])
		value := string(parts[1])
		switch key {
		case "max-rss":
			maxMemory, _ = strconv.Atoi(value) // Maximum memory usage in KB
		case "time":
			maxTime = value // Execution time (in seconds)
			//case "status":
			//	status = value // Status of execution
		}
	}

	// Fill result fields
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	result.Time = maxTime
	result.Memory = maxMemory // Use real memory usage from the metadata

	// Set the status based on metadata
	meta, err := getMetadata(metaFilePath)
	if err != nil {
		return result, nil
	}
	result.Status = meta["status"]
	result.Message = meta["message"]

	return result, nil
}

func getMetadata(metadataFilePath string) (map[string]string, error) {
	metadata := make(map[string]string)

	file, err := os.Open(metadataFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open metadata file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			metadata[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading metadata file: %v", err)
	}

	return metadata, nil
}
