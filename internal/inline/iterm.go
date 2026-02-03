package inline

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"path"
	"strings"
)

type ItermFile struct {
	Name        string
	Data        []byte
	WidthCells  int
	HeightCells int
	Stretch     bool
}

// SendItermInline emits iTerm2's OSC 1337 inline file sequence.
func SendItermInline(out *bufio.Writer, f ItermFile) {
	if out == nil || len(f.Data) == 0 {
		return
	}
	name := strings.TrimSpace(f.Name)
	if name == "" {
		name = "metcli.bin"
	}
	name = path.Base(name)

	preserveAspectRatio := 1
	if f.Stretch {
		preserveAspectRatio = 0
	}
	args := []string{
		"name=" + base64.StdEncoding.EncodeToString([]byte(name)),
		fmt.Sprintf("size=%d", len(f.Data)),
		"inline=1",
		fmt.Sprintf("preserveAspectRatio=%d", preserveAspectRatio),
	}
	if f.WidthCells > 0 {
		args = append(args, fmt.Sprintf("width=%d", f.WidthCells))
	}
	if f.HeightCells > 0 {
		args = append(args, fmt.Sprintf("height=%d", f.HeightCells))
	}

	encoded := base64.StdEncoding.EncodeToString(f.Data)
	_, _ = fmt.Fprintf(out, "\x1b]1337;File=%s:%s\x1b\\", strings.Join(args, ";"), encoded)
}
