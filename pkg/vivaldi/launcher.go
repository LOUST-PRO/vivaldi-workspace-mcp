package vivaldi

import (
	"fmt"
	"os/exec"
)

// LaunchURLsInVivaldi opens one or more URLs in Vivaldi using CLI execution.
func LaunchURLsInVivaldi(urls []string) error {
	if len(urls) == 0 {
		return fmt.Errorf("no URLs provided to launch")
	}

	args := append([]string{}, urls...)
	cmd := exec.Command("vivaldi", args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Vivaldi process: %w", err)
	}

	return nil
}
