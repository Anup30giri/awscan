package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func confirmAction(reader io.Reader, writer io.Writer, prompt string) (bool, error) {
	if _, err := fmt.Fprintf(writer, "%s [y/N]: ", prompt); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
