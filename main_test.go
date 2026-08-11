package main_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNodeCLIValidation(t *testing.T) {
	// Build the binary
	tempDir, err := os.MkdirTemp("", "blockchain-cli-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	binName := "toy-blockchain"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tempDir, binName)

	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v, output: %s", err, string(output))
	}

	tests := []struct {
		name           string
		args           []string
		expectedOutput string
		expectedExit   bool
	}{
		{
			name:           "Missing ID",
			args:           []string{"node", "--host", "127.0.0.1", "--port", "8001"},
			expectedOutput: "Error: Node ID cannot be empty.",
			expectedExit:   true,
		},
		{
			name:           "Missing Host",
			args:           []string{"node", "--id", "A", "--port", "8001"},
			expectedOutput: "Error: Node host cannot be empty.",
			expectedExit:   true,
		},
		{
			name:           "Invalid Port Low",
			args:           []string{"node", "--id", "A", "--host", "127.0.0.1", "--port", "0"},
			expectedOutput: "Error: Invalid port number: 0.",
			expectedExit:   true,
		},
		{
			name:           "Invalid Port High",
			args:           []string{"node", "--id", "A", "--host", "127.0.0.1", "--port", "70000"},
			expectedOutput: "Error: Invalid port number: 70000.",
			expectedExit:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binPath, tc.args...)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()

			if tc.expectedExit {
				if err == nil {
					t.Error("expected command to exit with error, but it succeeded")
				}
			}

			outputStr := stderr.String()
			if !strings.Contains(outputStr, tc.expectedOutput) {
				t.Errorf("expected output to contain %q, got %q", tc.expectedOutput, outputStr)
			}
		})
	}
}

func TestNodeCLILifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "blockchain-cli-lifecycle")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	binName := "toy-blockchain"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tempDir, binName)

	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v, output: %s", err, string(output))
	}

	// Choose a high port likely to be free
	port := "28999"

	cmd := exec.Command(binPath, "node", "--id", "TestNode", "--host", "127.0.0.1", "--port", port)

	// Capture stdout
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to get stdout pipe: %v", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start node: %v", err)
	}

	// We want to kill/stop the process at the end of the test
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// Wait for the node to print "Node is running"
	startedChan := make(chan bool, 1)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdoutPipe.Read(buf)
			if err != nil {
				if err != io.EOF {
					t.Logf("stdout read error: %v", err)
				}
				break
			}
			output := string(buf[:n])
			if strings.Contains(output, "Node is running") {
				startedChan <- true
				break
			}
		}
	}()

	select {
	case <-startedChan:
		// Started successfully!
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for node to start. Stderr: %s", stderr.String())
	}

	// Verify the endpoints
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/health", port))
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health check returned status %d", resp.StatusCode)
	}

	// Send Interrupt signal to trigger graceful shutdown
	if runtime.GOOS == "windows" {
		_ = cmd.Process.Kill()
	} else {
		err = cmd.Process.Signal(os.Interrupt)
		if err != nil {
			t.Logf("Process.Signal failed: %v. Falling back to Kill.", err)
			_ = cmd.Process.Kill()
		}
	}

	// Wait for process to exit
	exitChan := make(chan error, 1)
	go func() {
		exitChan <- cmd.Wait()
	}()

	select {
	case err := <-exitChan:
		if err != nil {
			t.Logf("Process exited with: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for node to shut down gracefully")
	}
}
