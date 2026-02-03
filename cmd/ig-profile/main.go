package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/steipete/metcli/internal/inline"
	"github.com/steipete/metcli/internal/instagram"
	"golang.org/x/term"
)

type outputItem struct {
	URL       string `json:"url"`
	Kind      string `json:"kind"`
	IsVideo   bool   `json:"is_video"`
	Shortcode string `json:"shortcode,omitempty"`
	TakenAt   int64  `json:"taken_at,omitempty"`
}

func main() {
	var (
		formatFlag        = flag.String("format", "auto", "auto|inline|url|json")
		maxFlag           = flag.Int("max", 12, "max items (0 = all)")
		profileFlag       = flag.String("profile", "", "Chrome profile name/dir or Cookies DB path")
		namesFlag         = flag.String("names", "", "comma-separated cookie names")
		userFlag          = flag.String("user", "", "Instagram username or profile URL")
		avatarFlag        = flag.Bool("avatar", true, "include profile picture")
		includeVideosFlag = flag.Bool("include-videos", true, "include video thumbnails")
		colsFlag          = flag.Int("cols", 28, "inline width in cells")
		rowsFlag          = flag.Int("rows", 0, "inline height in cells (0 = auto)")
		jsonFlag          = flag.Bool("json", false, "shorthand for --format json")
		urlFlag           = flag.Bool("url", false, "shorthand for --format url")
		inlineFlag        = flag.Bool("inline", false, "shorthand for --format inline")
	)

	flag.Usage = func() {
		_, _ = fmt.Fprintln(os.Stdout, "ig-profile")
		_, _ = fmt.Fprintln(os.Stdout, "\nUsage:\n  ig-profile [--format auto|inline|url|json] [--max N] [--avatar] [--profile <name|path>] <username|url>")
		_, _ = fmt.Fprintln(os.Stdout, "\nExamples:\n  ig-profile sportg33k --inline\n  ig-profile https://www.instagram.com/sportg33k/ --format url\n  ig-profile --avatar --max 6 sportg33k")
	}

	flag.Parse()

	username := strings.TrimSpace(*userFlag)
	if username == "" {
		args := flag.Args()
		if len(args) > 0 {
			username = strings.TrimSpace(args[0])
		}
	}
	username = instagram.ParseUsername(username)
	if username == "" {
		fail(fmt.Errorf("username or profile URL required"))
	}

	format := strings.ToLower(strings.TrimSpace(*formatFlag))
	if *jsonFlag {
		format = "json"
	}
	if *urlFlag {
		format = "url"
	}
	if *inlineFlag {
		format = "inline"
	}
	if format == "auto" {
		if isTerminal(os.Stdout) && inline.Detect() != inline.ProtocolNone {
			format = "inline"
		} else {
			format = "url"
		}
	}
	if format != "inline" && format != "url" && format != "json" {
		fail(fmt.Errorf("unsupported format: %s", format))
	}

	names := parseNames(*namesFlag)
	ctx := context.Background()
	cookies, warnings, err := instagram.LoadCookies(ctx, *profileFlag, names)
	if err != nil {
		fail(err)
	}
	if len(warnings) > 0 {
		_, _ = fmt.Fprintln(os.Stderr, "[ig-profile] sweetcookie warnings:")
		for _, w := range warnings {
			_, _ = fmt.Fprintf(os.Stderr, "- %s\n", w)
		}
	}

	profile, err := instagram.FetchProfile(ctx, username, cookies)
	if err != nil {
		fail(err)
	}
	media, err := instagram.FetchUserMedia(ctx, username, profile, cookies, *maxFlag, 50)
	if err != nil {
		if len(media) == 0 {
			fail(err)
		}
		_, _ = fmt.Fprintf(os.Stderr, "[ig-profile] media fetch warning: %s\n", err.Error())
	}
	profile.Media = media
	if len(profile.Media) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "[ig-profile] no profile media returned")
	}

	items := instagram.BuildItems(profile, *avatarFlag, *includeVideosFlag)
	if *maxFlag > 0 && len(items) > *maxFlag {
		items = items[:*maxFlag]
	}
	if len(items) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "[ig-profile] no images to render")
		return
	}

	switch format {
	case "json":
		payload := make([]outputItem, 0, len(items))
		for _, item := range items {
			payload = append(payload, outputItem{
				URL:       item.URL,
				Kind:      item.Kind,
				IsVideo:   item.IsVideo,
				Shortcode: item.Shortcode,
				TakenAt:   item.TakenAt,
			})
		}
		encoded, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			fail(err)
		}
		_, _ = fmt.Fprintln(os.Stdout, string(encoded))
	case "url":
		for _, item := range items {
			_, _ = fmt.Fprintln(os.Stdout, item.URL)
		}
	case "inline":
		renderInline(items, username, cookies, *colsFlag, *rowsFlag)
	default:
		fail(fmt.Errorf("unsupported format: %s", format))
	}
}

func renderInline(items []instagram.Item, username string, cookies instagram.CookieBundle, cols, rows int) {
	protocol := inline.Detect()
	if protocol == inline.ProtocolNone {
		for _, item := range items {
			_, _ = fmt.Fprintln(os.Stdout, item.URL)
		}
		return
	}

	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	client := instagram.ImageClient()
	nextID := uint32(1)
	for _, item := range items {
		data, width, height, err := instagram.DownloadImage(
			context.Background(),
			client,
			item.URL,
			username,
			cookies,
		)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "[ig-profile] %s\n", err.Error())
			continue
		}
		imageCols := cols
		imageRows := rows
		if imageRows == 0 && imageCols > 0 && width > 0 && height > 0 {
			imageRows = estimateRows(imageCols, width, height)
		}

		switch protocol {
		case inline.ProtocolIterm:
			inline.SendItermInline(writer, inline.ItermFile{
				Name:        instagram.InlineName(item),
				Data:        data,
				WidthCells:  imageCols,
				HeightCells: imageRows,
				Stretch:     true,
			})
		case inline.ProtocolKitty:
			pngData, err := instagram.EnsurePNG(data)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "[ig-profile] %s\n", err.Error())
				continue
			}
			inline.SendKittyPNG(writer, nextID, pngData, imageCols, imageRows)
			nextID++
		default:
			_, _ = fmt.Fprintln(os.Stdout, item.URL)
			continue
		}
		_, _ = fmt.Fprintln(writer)
		_ = writer.Flush()
	}
}

func estimateRows(cols, width, height int) int {
	if cols <= 0 || width <= 0 || height <= 0 {
		return 0
	}
	aspect := float64(height) / float64(width)
	rows := float64(cols) * aspect * cellAspectRatio()
	if rows < 1 {
		rows = 1
	}
	return int(math.Round(rows))
}

func cellAspectRatio() float64 {
	return inline.CellAspectRatio("METCLI_CELL_ASPECT", 0.5)
}

func parseNames(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return instagram.DefaultCookieNames()
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if len(out) == 0 {
		return instagram.DefaultCookieNames()
	}
	return out
}

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

func fail(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "[ig-profile] %s\n", err.Error())
	os.Exit(1)
}
