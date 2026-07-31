package vivaldi

import (
	"fmt"
	"os/exec"
)

func LaunchURLsInVivaldi(urls []string) error {
	if len(urls) == 0 {
		return fmt.Errorf("no se proporcionaron URLs para abrir")
	}

	args := append([]string{}, urls...)
	cmd := exec.Command("vivaldi", args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error iniciando Vivaldi: %w", err)
	}

	return nil
}
