/**
 * Component: Lessons CLI Confirmation Helper
 * Block-UUID: 0851b872-0971-4d42-93a4-7a8e9285d49d
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Provides yes/no confirmation handling for destructive or durable lessons CLI actions, with --yes support for scripted flows.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package lessons

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func confirm(prompt string, yes bool) bool {
	if yes {
		return true
	}
	fmt.Printf("%s [y/N] ", prompt)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}
