package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

func main() {
	r := gin.Default()

	// API route for code submission
	r.POST("/submit", func(c *gin.Context) {
		var submission Submission
		if err := c.ShouldBindJSON(&submission); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		// Create unique file for the submitted code
		filename, err := saveCode(submission.Language, submission.Code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save code"})
			return
		}

		// Run the code inside the Isolate sandbox with input and resource limits
		result, err := runCodeInIsolate(
			submission.Language, filename, submission.Input,
			submission.CPUTimeLimit, submission.WallTimeLimit, submission.MemoryLimit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Return the result of the execution
		c.JSON(http.StatusOK, gin.H{"result": result})
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

func runCodeInIsolate(language, filename, input string, cpuTimeLimit, wallTimeLimit, memoryLimit int) (string, error) {
	// Isolate boxId (you can manage different boxes for users if needed)
	boxId := "0"

	// Initialize the isolate box
	initCmd := exec.Command("isolate", "--init", "--box-id", boxId)
	if err := initCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to initialize sandbox: %v", err)
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
		return "", fmt.Errorf("failed to copy code to sandbox: %v", err)
	}

	// Base isolate command with time and memory limits
	command := []string{
		"isolate",
		"--run",
		"--box-id", boxId,
		"--time", fmt.Sprintf("%d", cpuTimeLimit), // CPU time limit
		"--wall-time", fmt.Sprintf("%d", wallTimeLimit), // Real-time limit
		"--mem", fmt.Sprintf("%d", memoryLimit), // Memory limit in KB
		"--"}

	// Add language-specific commands
	switch language {
	case "c":
		// Compile C program
		cmd := exec.Command("gcc", codeFilePath, "-o", filepath.Join(boxPath, "a.out"))
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("compilation failed")
		}
		command = append(command, "./a.out")
	case "cpp":
		// Compile C++ program
		cmd := exec.Command("g++", codeFilePath, "-o", filepath.Join(boxPath, "a.out"))
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("compilation failed")
		}
		command = append(command, "./a.out")
	case "python":
		// Run Python script directly
		command = append(command, "/usr/bin/python3", filepath.Base(codeFilePath))
	case "java":
		// Compile and run Java program
		cmd := exec.Command("javac", codeFilePath)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("compilation failed")
		}
		command = append(command, "/usr/bin/java", "Main") // Assuming the main class is "Main"
	default:
		return "", fmt.Errorf("unsupported language")
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
		return "", fmt.Errorf("execution failed: %v", stderr.String())
	}

	return stdout.String(), nil
}
