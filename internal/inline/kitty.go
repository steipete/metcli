package inline

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"strings"
)

type kittyData struct {
	Action   string
	ID       uint32
	Data     []byte
	Cols     int
	Rows     int
	NoCursor bool
}

// SendKittyPNG emits a Kitty graphics protocol image (PNG bytes expected).
func SendKittyPNG(out *bufio.Writer, id uint32, png []byte, cols, rows int) {
	if out == nil || len(png) == 0 {
		return
	}
	sendKittyData(out, kittyData{
		Action:   "T",
		ID:       id,
		Data:     png,
		Cols:     cols,
		Rows:     rows,
		NoCursor: true,
	})
}

func sendKittyData(out *bufio.Writer, data kittyData) {
	encoded := base64.StdEncoding.EncodeToString(data.Data)
	const chunkSize = 4096
	first := true
	for len(encoded) > 0 {
		chunk := encoded
		if len(chunk) > chunkSize {
			chunk = chunk[:chunkSize]
		}
		encoded = encoded[len(chunk):]
		more := 0
		if len(encoded) > 0 {
			more = 1
		}
		if first {
			params := []string{
				fmt.Sprintf("a=%s", data.Action),
				"f=100",
				fmt.Sprintf("i=%d", data.ID),
				fmt.Sprintf("m=%d", more),
				"q=2",
			}
			if data.Cols > 0 {
				params = append(params, fmt.Sprintf("c=%d", data.Cols))
			}
			if data.Rows > 0 {
				params = append(params, fmt.Sprintf("r=%d", data.Rows))
			}
			if data.NoCursor {
				params = append(params, "C=1")
			}
			_, _ = fmt.Fprintf(out, "\x1b_G%s;", strings.Join(params, ","))
			first = false
		} else {
			_, _ = fmt.Fprintf(out, "\x1b_Gm=%d;", more)
		}
		_, _ = fmt.Fprint(out, chunk)
		_, _ = fmt.Fprint(out, "\x1b\\")
	}
}
