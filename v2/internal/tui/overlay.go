package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/v4run/hangar/v2/internal/tui/theme"
)

// overlayOn paints `overlay` centered on top of `base`, with the
// base's visible region dimmed. Unlike lipgloss.Place, which simply
// outputs the overlay against whitespace, this walks `base` line-by-
// line, dims each rune to the muted color, and splices the overlay
// into the middle. The result reads as "that dialog is in front of
// something" rather than "that dialog replaced the screen."
func overlayOn(base, overlay string, width, height int) string {
	baseLines := padLines(strings.Split(base, "\n"), width, height)
	ovLines := strings.Split(strings.TrimRight(overlay, "\n"), "\n")
	ovH := len(ovLines)
	ovW := 0
	for _, l := range ovLines {
		if w := lipgloss.Width(l); w > ovW {
			ovW = w
		}
	}
	if ovH > height {
		ovH = height
	}
	if ovW > width {
		ovW = width
	}
	top := (height - ovH) / 2
	left := (width - ovW) / 2
	if top < 0 {
		top = 0
	}
	if left < 0 {
		left = 0
	}
	dim := theme.Hint
	out := make([]string, len(baseLines))
	for i, line := range baseLines {
		if i >= top && i < top+ovH {
			ov := ovLines[i-top]
			// Dim the portion of the base to the left of the overlay,
			// splice the overlay, then dim the right remainder.
			leftSeg := truncateExact(line, left)
			rightSeg := trimLeftWidth(line, left+ovW)
			out[i] = dim.Render(leftSeg) + ov + dim.Render(rightSeg)
		} else {
			out[i] = dim.Render(line)
		}
	}
	return strings.Join(out, "\n")
}

func padLines(lines []string, width, height int) []string {
	if len(lines) < height {
		pad := make([]string, height-len(lines))
		lines = append(lines, pad...)
	} else if len(lines) > height {
		lines = lines[:height]
	}
	for i, l := range lines {
		w := lipgloss.Width(l)
		if w < width {
			lines[i] = l + strings.Repeat(" ", width-w)
		}
	}
	return lines
}

// truncateExact returns the first `w` display cells of s. ANSI escape
// sequences inside s pass through unchanged.
func truncateExact(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	// Drop runes from the end until we fit.
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > w {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}

// trimLeftWidth drops the first `w` display cells and returns the rest.
func trimLeftWidth(s string, w int) string {
	runes := []rune(s)
	acc := 0
	for i, r := range runes {
		acc += lipgloss.Width(string(r))
		if acc > w {
			return string(runes[i:])
		}
	}
	return ""
}
