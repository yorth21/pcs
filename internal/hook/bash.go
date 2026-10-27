package hook

import (
	"path/filepath"
	"strings"
)

// bashRule decides whether one command segment, given its program name and the
// remaining arguments, is an allowlisted non-reading command.
type bashRule func(args []string) bool

func anyArgs([]string) bool { return true }

func subcommandIn(set ...string) bashRule {
	return func(args []string) bool {
		sub := firstPositional(args, nil)
		for _, s := range set {
			if sub == s {
				return true
			}
		}
		return false
	}
}

// bashAllowlist maps a program name to the rule that allows it. Every rule
// describes a command that does not read project source into the main thread.
var bashAllowlist = map[string]bashRule{
	"pcs":       anyArgs,
	"gh":        anyArgs,
	"codegraph": anyArgs,
	"engram":    anyArgs,
	"curl":      anyArgs,
	"npx":       anyArgs,
	"git": func(args []string) bool {
		sub := firstPositional(args, map[string]bool{"-C": true, "-c": true})
		switch sub {
		case "status", "log", "diff", "rev-parse", "branch", "switch", "checkout", "add", "commit", "stash", "show", "fetch", "pull", "worktree":
			return true
		}
		return false
	},
	"npm":    subcommandIn("run", "test", "dev", "build", "install", "exec"),
	"pnpm":   subcommandIn("run", "test", "dev", "build", "install", "exec"),
	"yarn":   subcommandIn("run", "test", "dev", "build", "install", "exec"),
	"bun":    subcommandIn("run", "test", "dev", "build", "install", "exec"),
	"dotnet": subcommandIn("build", "run", "test"),
	"go":     subcommandIn("build", "test", "vet", "run"),
	"node": func(args []string) bool {
		return len(args) == 1 && (args[0] == "--version" || args[0] == "-v")
	},
	"command": func(args []string) bool { return len(args) >= 1 && args[0] == "-v" },
	"echo":    anyArgs,
	"pwd":     anyArgs,
	"which":   anyArgs,
	"sleep":   anyArgs,
	"kill":    anyArgs,
	"mkdir":   anyArgs,
	"test":    anyArgs,
	"[":       anyArgs,
	"true":    anyArgs,
}

// BashAllowed reports whether every segment of command (split on &&, ||, ;, |
// and newlines, outside quotes) is an allowlisted command. `cd <dir>` segments
// are ignored. Command substitution and output redirection to a file are never
// allowlisted, since they can read or write project files.
func BashAllowed(command string) bool {
	if strings.Contains(command, "$(") || strings.Contains(command, "`") || strings.Contains(command, "<(") {
		return false
	}
	segments, ok := splitSegments(command)
	if !ok {
		return false
	}
	sawCommand := false
	for _, seg := range segments {
		words := shellWords(seg)
		words = stripRedirections(words)
		if words == nil {
			return false
		}
		// Skip leading environment assignments (NAME=value).
		for len(words) > 0 && isAssignment(words[0]) {
			words = words[1:]
		}
		if len(words) == 0 {
			continue
		}
		prog := programName(words[0])
		if prog == "cd" {
			continue
		}
		rule, ok := bashAllowlist[prog]
		if !ok || !rule(words[1:]) {
			return false
		}
		sawCommand = true
	}
	return sawCommand
}

func programName(word string) string {
	base := filepath.Base(filepath.FromSlash(strings.ReplaceAll(word, `\`, "/")))
	base = strings.ToLower(base)
	base = strings.TrimSuffix(base, ".exe")
	return base
}

func isAssignment(word string) bool {
	eq := strings.IndexByte(word, '=')
	if eq <= 0 {
		return false
	}
	for _, r := range word[:eq] {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func firstPositional(args []string, valueFlags map[string]bool) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if valueFlags[a] {
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a
	}
	return ""
}

// splitSegments splits command on unquoted &&, ||, ;, |, & and newlines. It
// reports false for an unterminated quote.
func splitSegments(command string) ([]string, bool) {
	var segs []string
	var cur strings.Builder
	var quote byte
	for i := 0; i < len(command); i++ {
		c := command[i]
		if quote != 0 {
			cur.WriteByte(c)
			if c == quote {
				quote = 0
			} else if c == '\\' && quote == '"' && i+1 < len(command) {
				i++
				cur.WriteByte(command[i])
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			cur.WriteByte(c)
		case '\\':
			cur.WriteByte(c)
			if i+1 < len(command) {
				i++
				cur.WriteByte(command[i])
			}
		case ';', '|', '\n', '&':
			// "2>&1" and ">&2" keep their ampersand inside the redirection.
			if c == '&' && i > 0 && command[i-1] == '>' {
				cur.WriteByte(c)
				continue
			}
			segs = append(segs, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if quote != 0 {
		return nil, false
	}
	segs = append(segs, cur.String())
	return segs, true
}

// shellWords splits a segment into words, removing quotes.
func shellWords(seg string) []string {
	var words []string
	var cur strings.Builder
	inWord := false
	var quote byte
	for i := 0; i < len(seg); i++ {
		c := seg[i]
		if quote != 0 {
			if c == quote {
				quote = 0
				continue
			}
			if c == '\\' && quote == '"' && i+1 < len(seg) {
				i++
				c = seg[i]
			}
			cur.WriteByte(c)
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			inWord = true
		case ' ', '\t', '\r':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words
}

var nullTargets = map[string]bool{"/dev/null": true, "nul": true, "NUL": true, "&1": true, "&2": true}

// stripRedirections removes redirections to the null device or between file
// descriptors. It returns nil when any other redirection (a file read or
// write) is present.
func stripRedirections(words []string) []string {
	out := []string{}
	for i := 0; i < len(words); i++ {
		w := words[i]
		idx := strings.IndexAny(w, "<>")
		if idx < 0 {
			out = append(out, w)
			continue
		}
		op := strings.TrimLeft(w[idx:], "<>")
		prefix := w[:idx]
		if prefix != "" && strings.Trim(prefix, "0123456789") != "" {
			return nil
		}
		if strings.Contains(w[idx:idx+1], "<") {
			return nil
		}
		target := op
		if target == "" && i+1 < len(words) {
			i++
			target = words[i]
		}
		if !nullTargets[target] {
			return nil
		}
	}
	return out
}
