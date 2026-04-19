// internal/fleet/output.go
package fleet

import "fmt"

var colorPalette = []string{
	"\033[31m", // red
	"\033[32m", // green
	"\033[33m", // yellow
	"\033[34m", // blue
	"\033[35m", // magenta
	"\033[36m", // cyan
	"\033[91m", // bright red
	"\033[92m", // bright green
	"\033[93m", // bright yellow
	"\033[94m", // bright blue
}

const colorReset = "\033[0m"

func AssignColors(servers []string) map[string]string {
	colors := make(map[string]string)
	for i, s := range servers {
		colors[s] = colorPalette[i%len(colorPalette)]
	}
	return colors
}

func FormatLine(serverName, color, line string, showBorder bool) string {
	if !showBorder {
		return line
	}
	return fmt.Sprintf("%s█%s %-15s %s", color, colorReset, serverName, line)
}
