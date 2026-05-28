package executor

import (
	"bytes"
	"fmt"
	"log/slog"
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

	slog.Info("executing command", "dir", workDir, "cmd", cmdArgs)
	err := cmd.Run()

	if err != nil {
		slog.Error("command failed", "error", err, "stdout", stdout.String(), "stderr", stderr.String())
		return fmt.Errorf("command execution failed: %w", err)
	}

	slog.Info("command succeeded", "stdout", stdout.String())
	return nil
}

// RunShellCommand executes a raw string command via the system shell in a specific directory.
// Only call this with trusted input (e.g. config-sourced deploy_command), not with HTTP payload data.
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

	slog.Info("executing shell command", "dir", workDir, "cmd", command)
	err := cmd.Run()

	if err != nil {
		slog.Error("command failed", "error", err, "stdout", stdout.String(), "stderr", stderr.String())
		return fmt.Errorf("command execution failed: %w", err)
	}

	slog.Info("command succeeded", "stdout", stdout.String())
	return nil
}

// PullImage executes `docker pull <image>` in the specified directory.
func PullImage(workDir string, image string) error {
	return RunCommand(workDir, []string{"docker", "pull", image})
}

// LoadTarImage executes `docker load -i <path>` in the specified directory.
func LoadTarImage(workDir string, tarPath string) error {
	return RunCommand(workDir, []string{"docker", "load", "-i", tarPath})
}

// DownloadGdown uses the 'gdown' cli to download a given file ID from google drive.
func DownloadGdown(workDir string, fileID string, outPath string) error {
	return RunCommand(workDir, []string{"gdown", fileID, "-O", outPath})
}
