package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/v4run/hangar/v2/internal/tui/theme"
)

// overlayOn paints `overlay` on top of `base`, dimmed-backdrop
// style. The overlay is centered inside the HORIZONTAL window
// [xMin, xMax) rather than the full terminal, so callers can
// anchor it over the right pane while the sidebar stays legible on
// the left.
func overlayOn(base, overlay string, width, height, xMin, xMax int) string {
	// Strip ANSI from base once — otherwise slicing by cell count
	// can split an escape mid-sequence and leak `\x1b[38;…` into
	// the user's terminal, which is exactly the garbage you'd see
	// behind a modal.
	plain := ansi.Strip(base)
	baseLines := padLines(strings.Split(plain, "\n"), width, height)
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
	// Clamp horizontal anchor + width to the requested window.
	if xMin < 0 {
		xMin = 0
	}
	if xMax <= xMin || xMax > width {
		xMax = width
	}
	windowW := xMax - xMin
	if ovW > windowW {
		ovW = windowW
	}
	top := (height - ovH) / 2
	left := xMin + (windowW-ovW)/2
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
			leftSeg := truncatePlain(line, left)
			rightSeg := trimLeftPlain(line, left+ovW)
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

// truncatePlain keeps the first `w` display cells of a plain
// (ANSI-free) string. Caller must have stripped escapes first.
func truncatePlain(s string, w int) string {
	if w <= 0 {
		return ""
	}
	runes := []rune(s)
	acc := 0
	for i := 0; i < len(runes); i++ {
		cw := lipgloss.Width(string(runes[i]))
		if acc+cw > w {
			return string(runes[:i])
		}
		acc += cw
	}
	return s
}

// trimLeftPlain drops the first `w` display cells of a plain
// (ANSI-free) string and returns the rest.
func trimLeftPlain(s string, w int) string {
	runes := []rune(s)
	acc := 0
	for i, r := range runes {
		cw := lipgloss.Width(string(r))
		if acc+cw > w {
			return string(runes[i:])
		}
		acc += cw
	}
	return ""
}
