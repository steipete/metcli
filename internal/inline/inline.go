package inline

import (
	"os"
	"strings"
)

type Protocol int

const (
	ProtocolNone Protocol = iota
	ProtocolKitty
	ProtocolIterm
)

func (p Protocol) String() string {
	switch p {
	case ProtocolKitty:
		return "kitty"
	case ProtocolIterm:
		return "iterm"
	default:
		return "none"
	}
}

func Detect() Protocol {
	return detectInline(os.Getenv)
}

func detectInline(getenv func(string) string) Protocol {
	override := strings.ToLower(strings.TrimSpace(getenv("METCLI_INLINE")))
	switch override {
	case "kitty":
		return ProtocolKitty
	case "iterm", "iterm2":
		return ProtocolIterm
	case "none", "off", "false", "0":
		return ProtocolNone
	case "", "auto":
	default:
		return ProtocolNone
	}

	if strings.TrimSpace(getenv("KITTY_WINDOW_ID")) != "" {
		return ProtocolKitty
	}

	termProgram := strings.ToLower(getenv("TERM_PROGRAM"))
	if strings.Contains(termProgram, "ghostty") {
		return ProtocolKitty
	}
	if strings.Contains(termProgram, "iterm") || strings.TrimSpace(getenv("ITERM_SESSION_ID")) != "" {
		return ProtocolIterm
	}
	if strings.Contains(termProgram, "apple_terminal") {
		return ProtocolNone
	}

	term := strings.ToLower(getenv("TERM"))
	if strings.Contains(term, "xterm-kitty") || strings.Contains(term, "ghostty") {
		return ProtocolKitty
	}

	return ProtocolNone
}
