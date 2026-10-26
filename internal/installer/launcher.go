package installer

import (
	"fmt"
	"strings"
)

// claudeCommand is the program every generated launcher runs.
const claudeCommand = "claude"

// SplitLauncherArgs splits the --launcher-args value into individual arguments.
//
// Rules (deliberately simple and identical on every platform):
//   - Unquoted whitespace separates arguments.
//   - Single quotes or double quotes group characters, including whitespace,
//     into one argument; the quotes themselves are removed. Inside single
//     quotes a double quote is literal and vice versa.
//   - Backslashes are always literal (Windows paths need no escaping), so there
//     is no escape character: use the other quote type to embed a quote.
//   - Shell metacharacters ($, `, %, ^, !, &, ...) are always literal.
//
// An unterminated quote is an error.
func SplitLauncherArgs(s string) ([]string, error) {
	var (
		args    []string
		current strings.Builder
		inWord  bool
		quote   rune
	)
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inWord = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if inWord {
				args = append(args, current.String())
				current.Reset()
				inWord = false
			}
		default:
			current.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("invalid --launcher-args %q: unterminated %c quote", s, quote)
	}
	if inWord {
		args = append(args, current.String())
	}
	return args, nil
}

// posixQuote returns s as a single POSIX sh word: everything inside single
// quotes is literal, and each embedded single quote is closed, backslash-
// escaped, and reopened (the four-character sequence: quote, backslash,
// quote, quote).
func posixQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// powerShellQuote returns s as a PowerShell verbatim (single-quoted) string.
// PowerShell also treats the typographic single quotes U+2018..U+201B as quote
// characters, so each of them is doubled like the ASCII apostrophe.
func powerShellQuote(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\'', '‘', '’', '‚', '‛':
			b.WriteRune(r)
		}
		b.WriteRune(r)
	}
	b.WriteByte('\'')
	return b.String()
}

// cmdQuote returns s as one double-quoted argument on a cmd.exe batch-file
// line. Two parsers read that line, and both must see s literally:
//
//   - The called program's Windows argument parser: the argument is wrapped in
//     double quotes, an embedded " becomes \" (backslashes before it doubled),
//     and trailing backslashes are doubled before the closing quote.
//   - cmd.exe: % is always doubled so batch variable expansion leaves it
//     literal. cmd.exe toggles its own quoted state on every unescaped ", so
//     the first embedded \" leaves cmd.exe's quoted region; from then on the
//     metacharacters & | < > ( ) ^ and every further quote are escaped with ^
//     (a ^" passes " through without toggling). Inside cmd.exe's quoted region
//     ^ and ! need no escaping because launchers disable delayed expansion.
func cmdQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	cmdQuoted := true
	backslashes := 0
	for _, r := range s {
		switch r {
		case '\\':
			backslashes++
			continue
		case '"':
			b.WriteString(strings.Repeat(`\`, backslashes*2+1))
			if cmdQuoted {
				b.WriteByte('"')
				cmdQuoted = false
			} else {
				b.WriteString(`^"`)
			}
		case '%':
			b.WriteString(strings.Repeat(`\`, backslashes))
			b.WriteString("%%")
		case '&', '|', '<', '>', '(', ')', '^':
			b.WriteString(strings.Repeat(`\`, backslashes))
			if !cmdQuoted {
				b.WriteByte('^')
			}
			b.WriteRune(r)
		default:
			b.WriteString(strings.Repeat(`\`, backslashes))
			b.WriteRune(r)
		}
		backslashes = 0
	}
	b.WriteString(strings.Repeat(`\`, backslashes*2))
	if cmdQuoted {
		b.WriteByte('"')
	} else {
		b.WriteString(`^"`)
	}
	return b.String()
}

// launcherScripts holds the three generated claude-pcs launcher bodies.
type launcherScripts struct {
	posix      string
	cmd        string
	powerShell string
}

// renderLaunchers builds the POSIX sh, cmd.exe, and PowerShell launchers. Each
// one scopes CLAUDE_CONFIG_DIR to the profile without leaking it into the
// calling shell and runs:
//
//	claude <launcher args> <user args> --mcp-config <shared-mcp.json>
//
// --mcp-config is variadic in Claude Code, so it must come last with exactly
// one value; otherwise positional user arguments are parsed as config files.
func renderLaunchers(targetDir, sharedMCPPath string, launcherArgs []string) launcherScripts {
	quoteAll := func(quote func(string) string) string {
		var b strings.Builder
		for _, a := range launcherArgs {
			b.WriteString(quote(a))
			b.WriteByte(' ')
		}
		return b.String()
	}

	// POSIX: process-scoped via exec env so variables never leak into the calling shell.
	posix := fmt.Sprintf("#!/usr/bin/env sh\nexec env CLAUDE_CONFIG_DIR=%s %s %s\"$@\" --mcp-config %s\n",
		posixQuote(targetDir), posixQuote(claudeCommand), quoteAll(posixQuote), posixQuote(sharedMCPPath))

	// cmd.exe: scoped via setlocal/endlocal; delayed expansion disabled so ! and ^ stay literal.
	// Windows paths cannot contain ", so the set "VAR=value" form only needs % doubled.
	cmd := fmt.Sprintf("@echo off\r\nsetlocal DisableDelayedExpansion\r\nset \"CLAUDE_CONFIG_DIR=%s\"\r\n%s %s%%* --mcp-config %s\r\nendlocal\r\n",
		strings.ReplaceAll(targetDir, "%", "%%"), cmdQuote(claudeCommand), quoteAll(cmdQuote), cmdQuote(sharedMCPPath))

	// PowerShell: saves and restores $env:CLAUDE_CONFIG_DIR in a finally block.
	powerShell := fmt.Sprintf("$prev = $env:CLAUDE_CONFIG_DIR\ntry {\n    $env:CLAUDE_CONFIG_DIR = %s\n    & %s %s@args --mcp-config %s\n} finally {\n    $env:CLAUDE_CONFIG_DIR = $prev\n}\n",
		powerShellQuote(targetDir), powerShellQuote(claudeCommand), quoteAll(powerShellQuote), powerShellQuote(sharedMCPPath))

	return launcherScripts{posix: posix, cmd: cmd, powerShell: powerShell}
}
