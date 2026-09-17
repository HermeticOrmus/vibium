package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	_ "embed"
)

// Brand colors from vibium.com landing CSS (--landing-accent #ff6700,
// --landing-text #fafafa). The ASCII art is the official mascot and V mark
// from ~/Documents/vibium-profile-pictures/ascii (24-bit color on █).
var (
	brandAccent = rgb{255, 103, 0}
	brandText   = rgb{250, 250, 250}
	brandMuted  = rgb{178, 178, 178}
	brandOK     = rgb{163, 223, 187}
	brandFail   = rgb{239, 68, 68}
)

type rgb struct{ r, g, b int }

const ansiReset = "\x1b[0m"

//go:embed brand/mascot-color.txt
var mascotColor string

//go:embed brand/mascot.txt
var mascotPlain string

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

func setupBanner(color bool) string {
	art := strings.TrimRight(mascotPlain, "\n")
	if color {
		art = strings.TrimRight(mascotColor, "\n")
	}
	word := maybePaint(color, brandText, "vibium")
	sub := maybePaint(color, brandAccent, "setup")
	tag := maybePaint(color, brandMuted, "Agents make it. We check it.")
	var b strings.Builder
	b.WriteByte('\n')
	b.WriteString(art)
	b.WriteString("\n\n  ")
	b.WriteString(word)
	b.WriteByte(' ')
	b.WriteString(sub)
	b.WriteString("\n  ")
	b.WriteString(tag)
	b.WriteString("\n\n")
	return b.String()
}
