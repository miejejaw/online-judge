package controllers

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os/exec"
)

type RunController struct{}

func NewRunController() *RunController {
	return &RunController{}
}

func (ctrl *RunController) RunCode(c *gin.Context) {

	// Execute the Python script
	cmd := exec.Command("python3", "a.py")
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Printf("Error executing Python script: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to run the Python script",
		})
		return
	}

	// Return the output of the Python script
	c.JSON(http.StatusOK, gin.H{
		"output": string(output),
	})
}
