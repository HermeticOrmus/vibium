package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Brand colors from vibium.com landing CSS (--landing-accent #ff6700,
// --landing-text #fafafa, --landing-bg #0a0a0b) and the V-mark gradient
// (left diamond red-orange, right diamond amber).
var (
	brandAccent = rgb{255, 103, 0}
	brandRed    = rgb{226, 61, 18}
	brandAmber  = rgb{255, 138, 0}
	brandText   = rgb{250, 250, 250}
	brandMuted  = rgb{178, 178, 178}
	brandOK     = rgb{163, 223, 187}
	brandFail   = rgb{239, 68, 68}
)

type rgb struct{ r, g, b int }

const ansiReset = "\x1b[0m"

func (c rgb) paint(s string) string {
	if s == "" {
		return s
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s%s", c.r, c.g, c.b, s, ansiReset)
}

func stdoutColor() bool {
	if jsonOutput {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func writerColor(w io.Writer) bool {
	if !stdoutColor() {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func maybePaint(on bool, c rgb, s string) string {
	if !on {
		return s
	}
	return c.paint(s)
}

// setupBanner is the vibium.com wordmark: geometric V (favicon) plus the
// low-poly orange robot (GitHub org avatar), then the site tagline.
func setupBanner(color bool) string {
	v := setupMarkV(color)
	bot := setupMascot(color)
	word := maybePaint(color, brandText, "vibium")
	tag := maybePaint(color, brandMuted, "Agents make it. We check it.")
	sub := maybePaint(color, brandAccent, "setup")

	vLines := strings.Split(strings.TrimRight(v, "\n"), "\n")
	bLines := strings.Split(strings.TrimRight(bot, "\n"), "\n")
	botW := 0
	for _, line := range bLines {
		if w := visibleWidth(line); w > botW {
			botW = w
		}
	}
	for len(bLines) < len(vLines) {
		bLines = append(bLines, "")
	}
	var b strings.Builder
	b.WriteByte('\n')
	for i, vl := range vLines {
		b.WriteString(padVisible(bLines[i], botW))
		b.WriteString("  ")
		b.WriteString(vl)
		if i == 2 {
			b.WriteString("  ")
			b.WriteString(word)
		}
		if i == 3 {
			b.WriteString("  ")
			b.WriteString(sub)
		}
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString("  ")
	b.WriteString(tag)
	b.WriteString("\n\n")
	return b.String()
}

func padVisible(s string, width int) string {
	w := visibleWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func visibleWidth(s string) int {
	n := 0
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		n++
	}
	return n
}

func setupMarkV(color bool) string {
	// Geometric V from vibium.com / favicon-512: left diamond red-orange,
	// right diamond amber, meeting at a point.
	left := func(s string) string { return maybePaint(color, brandRed, s) }
	right := func(s string) string { return maybePaint(color, brandAmber, s) }
	mid := func(s string) string { return maybePaint(color, brandAccent, s) }
	return strings.Join([]string{
		left("█▀▀▄") + "        " + right("▄▀▀█"),
		" " + left("█▄") + "  ▀▄  ▄▀  " + right("▄█"),
		"  " + left("▀█▄") + "  " + mid("▀▀") + "  " + right("▄█▀"),
		"    " + left("▀█▄") + "  " + right("▄█▀"),
		"      " + mid("▀██▀"),
		"        " + mid("▀▀"),
	}, "\n") + "\n"
}

func setupMascot(color bool) string {
	// Low-poly orange robot from https://github.com/VibiumDev.png
	h := func(s string) string { return maybePaint(color, brandAmber, s) }
	d := func(s string) string { return maybePaint(color, brandRed, s) }
	v := func(s string) string { return maybePaint(color, brandAccent, s) }
	return strings.Join([]string{
		"   " + h("▄▄██▄▄"),
		"  " + h("█") + " " + v("▄████▄") + " " + h("█"),
		"  " + h("█") + " " + d("▀████▀") + " " + h("█"),
		"   " + h("▀▄") + v("████") + h("▄▀"),
		"    " + v("██████"),
		"   " + h("█") + " " + d("▐██▌") + " " + h("█"),
	}, "\n") + "\n"
}
