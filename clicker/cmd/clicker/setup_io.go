package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type setupUI struct {
	in          io.Reader
	out         io.Writer
	err         io.Writer
	interactive bool
}

func newSetupUI(cmd *cobra.Command, interactive bool) *setupUI {
	return &setupUI{
		in:          cmd.InOrStdin(),
		out:         cmd.OutOrStdout(),
		err:         cmd.ErrOrStderr(),
		interactive: interactive,
	}
}

func inputIsTTY(cmd *cobra.Command) bool {
	f, ok := cmd.InOrStdin().(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func (ui *setupUI) println(format string, args ...any) {
	if jsonOutput {
		return
	}
	fmt.Fprintf(ui.out, format+"\n", args...)
}

func (ui *setupUI) prompt(label, def string) (string, error) {
	if !ui.interactive {
		return def, nil
	}
	if def != "" {
		fmt.Fprintf(ui.out, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(ui.out, "%s: ", label)
	}
	line, err := readLine(ui.in)
	if err != nil {
		return "", err
	}
	if line == "" {
		return def, nil
	}
	return line, nil
}

func (ui *setupUI) promptSecret(label string) (string, error) {
	if !ui.interactive {
		return "", nil
	}
	fmt.Fprintf(ui.out, "%s: ", label)
	restore := func() {}
	if f, ok := ui.in.(*os.File); ok {
		restore = disableEcho(f)
	}
	line, err := readLine(ui.in)
	restore()
	fmt.Fprintln(ui.out)
	return line, err
}

func (ui *setupUI) confirm(label string, defYes bool) (bool, error) {
	hint := "Y/n"
	if !defYes {
		hint = "y/N"
	}
	ans, err := ui.prompt(label+" ["+hint+"]", "")
	if err != nil {
		return false, err
	}
	ans = strings.TrimSpace(strings.ToLower(ans))
	if ans == "" {
		return defYes, nil
	}
	return ans == "y" || ans == "yes", nil
}

func readLine(r io.Reader) (string, error) {
	s, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(s, "\r\n"), nil
}
