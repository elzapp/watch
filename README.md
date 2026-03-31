# GOWATCH(1)

## NAME

**gowatch** - execute a program periodically, showing output fullscreen

## SYNOPSIS

**gowatch** [**-d**] [**-w**] [**-n** *seconds*] *command* [*args*...]

## DESCRIPTION

**gowatch** runs *command* repeatedly, displaying its output in a fullscreen terminal UI. This allows you to watch the program output change over time. By default, the program is run every 2 seconds; use **-n** to specify a different interval.

The command is given to `sh -c`, which means shell features like pipes, redirections, and environment variable expansion work as expected.

**gowatch** will run until interrupted.

## FLAGS

| Flag | Description |
|------|-------------|
| **-n** *seconds* | Set the update interval in seconds. Default is **2.0**. Fractional values are accepted. |
| **-d** | Enable difference highlighting. Changes between successive updates are highlighted using an LCS-based diff algorithm. |
| **-w** | Turn off line wrapping. Long lines are truncated to the viewport width instead of wrapping. Can also be toggled at runtime with **w**. |
| **--license** | Display the license and exit. |

## KEYS

| Key | Action |
|-----|--------|
| **q**, **Ctrl+C** | Quit |
| **Ctrl+Z** | Suspend |
| **Space** | Force an immediate refresh |
| **+** / **-** | Increase / decrease update frequency by 0.2s |
| **d** | Toggle difference highlighting |
| **w** | Toggle line wrapping |
| **f** | Toggle fullscreen (hides header, footer, and border for clean text selection) |
| **?** | Toggle full help view |
| **j/k**, **Up/Down** | Scroll output |
| **PgUp/PgDn** | Scroll by page |
| **Home/End** | Jump to top / bottom |
| Mouse wheel | Scroll output |

## EXAMPLES

Watch the date update every second:

    gowatch -n 1 date

Watch directory contents with difference highlighting:

    gowatch -d ls -l

Watch a filtered process list:

    gowatch -d 'ps aux | grep nginx'

Watch disk usage, refreshing every 5 seconds:

    gowatch -n 5 df -h

Watch container status:

    gowatch -n 3 docker ps

Watch Flux kustomizations with diff highlighting:

    gowatch -d kubectl get kustomization -A

## DIFFERENCES FROM GNU WATCH

**gowatch** is modelled after GNU **watch**(1), with the following differences:

- **Smarter diff highlighting.** Uses an LCS (Longest Common Subsequence) algorithm instead of positional line comparison. Inserted or removed lines do not cause all subsequent lines to appear changed. Consecutive removed lines are collapsed into a single summary (e.g., "3 lines removed").

- **Runtime controls.** The interval can be adjusted at runtime with **+**/**-**. Diff mode can be toggled with **d**. An immediate refresh can be forced with **Space**.

- **Fullscreen mode.** Pressing **f** hides all UI chrome (header, footer, border) so you can select and copy multi-line text cleanly.

- **Scrollable output.** Output longer than the terminal is scrollable via keyboard or mouse wheel, rather than being silently truncated.

- **Modern TUI.** Built with Bubble Tea and Lip Gloss. Features a styled header bar, bordered viewport, animated spinner during execution, and a contextual help bar.

- **No ANSI color passthrough (`-c`/`--color`).** ANSI color/style sequences from command output are not currently interpreted or preserved.

- **No `--exec` / `-x` flag.** Commands are always passed to `sh -c`.

- **No `--beep` / `-b` flag.** No audible alert on non-zero exit status.

- **No `--errexit` / `-e` flag.** gowatch does not exit when the command returns a non-zero exit code.

- **No `--chgexit` / `-g` flag.** gowatch does not exit when the command output changes.

- **No `--equexit` / `-q` flag.** gowatch does not exit after a given number of unchanged cycles.

- **No `--no-title` / `-t` flag.** The header bar cannot be hidden via a flag (though pressing **f** hides all chrome at runtime).


- **No `--precise` / `-p` flag.** Intervals are measured from command completion, not wall-clock aligned.

- **No permanent diff mode.** GNU watch supports `--differences=permanent`, which keeps all positions that have ever changed highlighted since the first iteration. gowatch only highlights differences between the two most recent runs.

## SEE ALSO

watch(1)

## AUTHORS

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), and [Glamour](https://github.com/charmbracelet/glamour).

## LICENSE

**gowatch** is licensed under the GNU General Public License v3.0. See the [`LICENSE`](LICENSE) file for the full text, or run **gowatch --license**. Third-party library notices are in [`THIRD-PARTY-NOTICES`](THIRD-PARTY-NOTICES).
