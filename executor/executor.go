package executor

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"runtime"
)

// RunCommand executes a command in a specific directory.
func RunCommand(workDir string, cmdArgs []string) error {
	if len(cmdArgs) == 0 {
		return fmt.Errorf("no command arguments provided")
	}

	command := cmdArgs[0]
	args := cmdArgs[1:]

	cmd := exec.Command(command, args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("Executing command in %s: %v", workDir, cmdArgs)
	err := cmd.Run()

	if err != nil {
		log.Printf("Command failed. Error: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
		return fmt.Errorf("command execution failed: %w", err)
	}

	log.Printf("Command succeeded. Stdout: %s", stdout.String())
	return nil
}

// RunShellCommand executes a raw string command in a specific directory.
func RunShellCommand(workDir string, command string) error {
	if command == "" {
		return fmt.Errorf("no command provided")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}

	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("Executing shell command in %s: %s", workDir, command)
	err := cmd.Run()

	if err != nil {
		log.Printf("Command failed. Error: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
		return fmt.Errorf("command execution failed: %w", err)
	}

	log.Printf("Command succeeded. Stdout: %s", stdout.String())
	return nil
}

// PullImage executes `docker pull <image>` in the specified directory.
func PullImage(workDir string, image string) error {
	log.Printf("Pulling docker image %s", image)
	return RunShellCommand(workDir, fmt.Sprintf("docker pull %s", image))
}

// LoadTarImage executes `docker load -i <path>` in the specified directory.
func LoadTarImage(workDir string, tarPath string) error {
	log.Printf("Loading docker image from tar: %s", tarPath)
	return RunShellCommand(workDir, fmt.Sprintf("docker load -i %s", tarPath))
}

// DownloadGdown uses the 'gdown' cli to download a given file ID from google drive.
func DownloadGdown(workDir string, fileID string, outPath string) error {
	log.Printf("Downloading file from Google Drive (ID: %s) to %s", fileID, outPath)
	return RunShellCommand(workDir, fmt.Sprintf("gdown %s -O %s", fileID, outPath))
}
