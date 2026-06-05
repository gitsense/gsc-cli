/**
 * Component: App Utilities
 * Block-UUID: ccd4a047-7259-4d27-a62f-907674f5bd87
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Shared utility functions for the app package, including time formatting helpers.
 * Language: Go
 * Created-at: 2026-05-30T17:35:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package app

import (
	"fmt"
	"time"
)

// FormatUptime formats a duration into a human-readable string
func FormatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm %.0fs", d.Minutes(), d.Seconds()-float64(int(d.Minutes()))*60)
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.0fh %.0fm", d.Hours(), d.Minutes()-float64(int(d.Hours()))*60)
	} else {
		return fmt.Sprintf("%.0fd %.0fh", d.Hours()/24, d.Hours()-float64(int(d.Hours()/24))*24)
	}
}
