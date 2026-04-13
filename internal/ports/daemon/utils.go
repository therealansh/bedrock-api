package daemon

import (
	"fmt"
	"os"
)

// returns the path to the lock file for a session volume.
func lockFilePath(base, sessionId string) string {
	return fmt.Sprintf("%s/%s/.lock", base, sessionId)
}

// creates a .lock file in the session volume directory.
func createLockFile(base, sessionId string) error {
	return os.WriteFile(lockFilePath(base, sessionId), nil, 0644)
}

// removes the .lock file from the session volume directory.
func removeLockFile(base, sessionId string) error {
	return os.Remove(lockFilePath(base, sessionId))
}

// creates the output directory for the tracer if it doesn't exist.
func createTracerOutputDir(base, sessionId string) error {
	outputDir := fmt.Sprintf("%s/%s", base, sessionId)
	return os.MkdirAll(outputDir, 0755)
}

// returns the default flags for the tracer container.
func defaultContainerFlags() map[string]any {
	return map[string]any{
		"pid":        "host",
		"privileged": true,
	}
}

// returns the default volume mappings for the tracer container.
func defaultTracerVolumes(base, sessionId string) map[string]string {
	return map[string]string{
		"/sys":                 "/sys:rw",
		"/lib/modules":         "/lib/modules:ro",
		"/var/run/docker.sock": "/var/run/docker.sock",
		base + "/" + sessionId: "/logs",
	}
}

// returns the default command to run the tracer container.
func defaultTracerCommand(targetContainerName string) []string {
	return []string{
		"bdtrace",
		"--container",
		targetContainerName,
		"-o",
		"/logs",
	}
}
