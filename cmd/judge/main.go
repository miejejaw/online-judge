package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
)

// Submission represents the structure of a code submission
type Submission struct {
	Code        string `json:"code" binding:"required"`
	Language    string `json:"language" binding:"required"`
	Input       string `json:"input" binding:"required"`
	Output      string `json:"output" binding:"required"`
	TimeLimit   int    `json:"time_limit" binding:"required"`   // in seconds
	MemoryLimit int    `json:"memory_limit" binding:"required"` // in MB (currently not enforced)
}

func main() {
	r := gin.Default()

	// Route to handle code submissions
	r.POST("/submit", handleSubmission)

	// Start the server
	r.Run(":8080") // Server runs on port 8080
}

// handleSubmission handles the incoming code submissions
func handleSubmission(c *gin.Context) {
	var submission Submission

	// Parse JSON input
	if err := c.ShouldBindJSON(&submission); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Execute the submitted code using Docker SDK
	result, execTime, output := executeWithDocker(submission)

	// Return the result as JSON
	c.JSON(http.StatusOK, gin.H{
		"result":         result,
		"execution_time": fmt.Sprintf("%v ms", execTime.Milliseconds()),
		"output":         output,
	})
}

func executeWithDocker(submission Submission) (string, time.Duration, string) {
	// Initialize Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Sprintf("Error initializing Docker client: %v", err), 0, ""
	}

	// Choose the container image based on the programming language
	var image string
	var cmd []string
	switch submission.Language {
	case "python":
		image = "python-exec"
		cmd = []string{"python3", "-c", submission.Code} // Execute Python code
	case "cpp":
		image = "gcc:latest"
		cmd = []string{"bash", "-c", fmt.Sprintf(`echo "%s" > /tmp/code.cpp && g++ /tmp/code.cpp -o /tmp/a.out && /tmp/a.out`, submission.Code)} // Compile and run C++ code
	default:
		return "Unsupported language", 0, ""
	}

	// Create the container
	resp, err := cli.ContainerCreate(context.Background(), &container.Config{
		Image:     image,
		Cmd:       cmd,
		OpenStdin: true,
	}, &container.HostConfig{
		AutoRemove: true, // Automatically remove the container after execution
	}, nil, nil, "")
	if err != nil {
		return fmt.Sprintf("Error creating Docker container: %v", err), 0, ""
	}

	// Start the container
	if err := cli.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		return fmt.Sprintf("Error starting Docker container: %v", err), 0, ""
	}

	// attach the container to stdin
	hijackedResp, err := cli.ContainerAttach(context.Background(), resp.ID, container.AttachOptions{Stream: true, Stdin: true})
	if err != nil {
		return fmt.Sprintf("Error attaching to Docker container: %v", err), 0, ""
	}

	// Send input to the container via stdin
	_, err = io.Copy(hijackedResp.Conn, strings.NewReader(submission.Input))
	if err != nil {
		return fmt.Sprintf("Error sending input to container: %v", err), 0, ""
	}
	hijackedResp.Close()

	// Set the timeout for execution
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(submission.TimeLimit)*time.Second)
	defer cancel()

	start := time.Now() // Start the timer

	// Wait for the container to finish execution
	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Sprintf("Error waiting for Docker container: %v", err), 0, ""
		}
	case <-statusCh:
	}

	// Measure execution time
	execTime := time.Since(start)

	// Fetch the container logs (output)
	out, err := cli.ContainerLogs(context.Background(), resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return fmt.Sprintf("Error fetching container logs: %v", err), 0, ""
	}

	// Read the logs (container output)
	output, err := ioutil.ReadAll(out)
	if err != nil {
		return fmt.Sprintf("Error reading container logs: %v", err), 0, ""
	}

	// Clean up the output by trimming excess whitespace
	cleanOutput := sanitizeOutput(string(output))

	// Compare the output with the expected output
	if cleanOutput == submission.Output {
		return "Accepted", execTime, cleanOutput
	} else {
		return "Wrong Answer", execTime, cleanOutput
	}
}

// sanitizeOutput removes unwanted characters from the container output
func sanitizeOutput(output string) string {
	// Remove any non-printable characters (like control characters)
	cleanOutput := strings.Map(func(r rune) rune {
		if r >= 32 && r <= 126 || r == '\n' || r == '\r' {
			return r
		}
		return -1
	}, output)

	// Trim leading/trailing spaces and newlines
	return strings.TrimSpace(cleanOutput)
}
