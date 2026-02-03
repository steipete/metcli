package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	imagedraw "image/draw"
	"image/png"
	"math"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/steipete/metcli/internal/inline"
	"github.com/steipete/metcli/internal/instagram"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/term"
)

type CLI struct {
	Instagram InstagramCmd `cmd:"" help:"Instagram helpers"`
}

type InstagramCmd struct {
	Profile InstagramProfileCmd `cmd:"" help:"Show profile images"`
	Feed    InstagramFeedCmd    `cmd:"" help:"Show feed images"`
	URLs    InstagramURLsCmd    `cmd:"" name:"urls" help:"List profile image URLs"`
}

type InstagramProfileCmd struct {
	User          string `arg:"" optional:"" name:"user" help:"Username or profile URL"`
	Format        string `help:"auto|inline|url|json" default:"auto"`
	Inline        bool   `help:"shorthand for --format inline"`
	URL           bool   `help:"shorthand for --format url"`
	JSON          bool   `help:"shorthand for --format json"`
	Max           int    `help:"max items (0 = all)" default:"0"`
	Avatar        bool   `help:"include profile picture" default:"true" negatable:""`
	IncludeVideos bool   `help:"include video thumbnails" default:"true" negatable:""`
	Profile       string `help:"Chrome profile name/dir or Cookies DB path"`
	Names         string `help:"comma-separated cookie names"`
	GridCols      int    `help:"grid columns" default:"4"`
	ThumbCols     int    `help:"thumb width in cells (0 = auto)" default:"0"`
	ThumbPx       int    `help:"thumbnail size in px" default:"256"`
	PaddingPx     int    `help:"padding between thumbs in px" default:"8"`
	PageSize      int    `help:"images per grid page (0 = auto)" default:"0"`
}

type InstagramFeedCmd struct {
	User          string `arg:"" optional:"" name:"user" help:"Username or profile URL"`
	Format        string `help:"url|inline|json" default:"url"`
	Inline        bool   `help:"shorthand for --format inline"`
	URL           bool   `help:"shorthand for --format url"`
	JSON          bool   `help:"shorthand for --format json"`
	Max           int    `help:"max items (0 = all)" default:"0"`
	Avatar        bool   `help:"include profile picture" default:"true" negatable:""`
	IncludeVideos bool   `help:"include video thumbnails" default:"true" negatable:""`
	Source        string `help:"main|api" default:"api"`
	PageSize      int    `help:"items per API page (1-50)" default:"50"`
	Profile       string `help:"Chrome profile name/dir or Cookies DB path"`
	Names         string `help:"comma-separated cookie names"`
	GridCols      int    `help:"grid columns" default:"4"`
	ThumbCols     int    `help:"thumb width in cells (0 = auto)" default:"0"`
	ThumbPx       int    `help:"thumbnail size in px" default:"256"`
	PaddingPx     int    `help:"padding between thumbs in px" default:"8"`
	PageGridSize  int    `help:"images per grid page (0 = auto)" default:"0"`
}

type InstagramURLsCmd struct {
	User          string `arg:"" optional:"" name:"user" help:"Username or profile URL"`
	Max           int    `help:"max items (0 = all)" default:"0"`
	Avatar        bool   `help:"include profile picture" default:"true" negatable:""`
	IncludeVideos bool   `help:"include video thumbnails" default:"true" negatable:""`
	Source        string `help:"main|api" default:"api"`
	PageSize      int    `help:"items per API page (1-50)" default:"50"`
	Profile       string `help:"Chrome profile name/dir or Cookies DB path"`
	Names         string `help:"comma-separated cookie names"`
}

type outputItem struct {
	URL       string `json:"url"`
	Kind      string `json:"kind"`
	IsVideo   bool   `json:"is_video"`
	Shortcode string `json:"shortcode,omitempty"`
	TakenAt   int64  `json:"taken_at,omitempty"`
}

func main() {
	cli := CLI{}
	ctx := kong.Parse(&cli, kong.Name("metcli"), kong.UsageOnError())
	switch cmd := ctx.Command(); cmd {
	case "instagram profile <user>":
		if err := cli.Instagram.Profile.Run(); err != nil {
			fail(err)
		}
	case "instagram profile":
		if err := cli.Instagram.Profile.Run(); err != nil {
			fail(err)
		}
	case "instagram feed <user>":
		if err := cli.Instagram.Feed.Run(); err != nil {
			fail(err)
		}
	case "instagram feed":
		if err := cli.Instagram.Feed.Run(); err != nil {
			fail(err)
		}
	case "instagram urls <user>":
		if err := cli.Instagram.URLs.Run(); err != nil {
			fail(err)
		}
	case "instagram urls":
		if err := cli.Instagram.URLs.Run(); err != nil {
			fail(err)
		}
	default:
		fail(fmt.Errorf("unknown command: %s", cmd))
	}
}

func (cmd *InstagramProfileCmd) Run() error {
	username := instagram.ParseUsername(cmd.User)
	if username == "" {
		return fmt.Errorf("username or profile URL required")
	}

	format := strings.ToLower(strings.TrimSpace(cmd.Format))
	if cmd.Inline {
		format = "inline"
	}
	if cmd.URL {
		format = "url"
	}
	if cmd.JSON {
		format = "json"
	}
	if format == "auto" {
		if isTerminal(os.Stdout) && inline.Detect() != inline.ProtocolNone {
			format = "inline"
		} else {
			format = "url"
		}
	}
	if format != "inline" && format != "url" && format != "json" {
		return fmt.Errorf("unsupported format: %s", format)
	}

	ctx := context.Background()
	cookies, items, warnings, err := loadInstagramItems(
		ctx,
		username,
		cmd.Profile,
		cmd.Names,
		"api",
		50,
		cmd.Max,
		cmd.Avatar,
		cmd.IncludeVideos,
	)
	if err != nil {
		return err
	}
	printWarnings("[metcli]", warnings)
	if len(items) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "[metcli] no images to render")
		return nil
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
			return err
		}
		_, _ = fmt.Fprintln(os.Stdout, string(encoded))
	case "url":
		for _, item := range items {
			_, _ = fmt.Fprintln(os.Stdout, item.URL)
		}
	case "inline":
		renderGrid(items, username, cookies, gridOptions{
			GridCols:  cmd.GridCols,
			ThumbCols: cmd.ThumbCols,
			ThumbPx:   cmd.ThumbPx,
			PaddingPx: cmd.PaddingPx,
			PageSize:  cmd.PageSize,
		})
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	return nil
}

func (cmd *InstagramFeedCmd) Run() error {
	username := instagram.ParseUsername(cmd.User)
	if username == "" {
		return fmt.Errorf("username or profile URL required")
	}

	format := strings.ToLower(strings.TrimSpace(cmd.Format))
	if cmd.Inline {
		format = "inline"
	}
	if cmd.URL {
		format = "url"
	}
	if cmd.JSON {
		format = "json"
	}
	if format != "inline" && format != "url" && format != "json" {
		return fmt.Errorf("unsupported format: %s", format)
	}

	ctx := context.Background()
	cookies, items, warnings, err := loadInstagramItems(
		ctx,
		username,
		cmd.Profile,
		cmd.Names,
		cmd.Source,
		cmd.PageSize,
		cmd.Max,
		cmd.Avatar,
		cmd.IncludeVideos,
	)
	if err != nil {
		return err
	}
	printWarnings("[metcli]", warnings)
	if len(items) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "[metcli] no images to render")
		return nil
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
			return err
		}
		_, _ = fmt.Fprintln(os.Stdout, string(encoded))
	case "url":
		for _, item := range items {
			_, _ = fmt.Fprintln(os.Stdout, item.URL)
		}
	case "inline":
		renderGrid(items, username, cookies, gridOptions{
			GridCols:  cmd.GridCols,
			ThumbCols: cmd.ThumbCols,
			ThumbPx:   cmd.ThumbPx,
			PaddingPx: cmd.PaddingPx,
			PageSize:  cmd.PageGridSize,
		})
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	return nil
}

func (cmd *InstagramURLsCmd) Run() error {
	username := instagram.ParseUsername(cmd.User)
	if username == "" {
		return fmt.Errorf("username or profile URL required")
	}

	ctx := context.Background()
	_, items, warnings, err := loadInstagramItems(
		ctx,
		username,
		cmd.Profile,
		cmd.Names,
		cmd.Source,
		cmd.PageSize,
		cmd.Max,
		cmd.Avatar,
		cmd.IncludeVideos,
	)
	if err != nil {
		return err
	}
	printWarnings("[metcli]", warnings)
	for _, item := range items {
		_, _ = fmt.Fprintln(os.Stdout, item.URL)
	}
	return nil
}

type gridOptions struct {
	GridCols  int
	ThumbCols int
	ThumbPx   int
	PaddingPx int
	PageSize  int
}

func renderGrid(items []instagram.Item, username string, cookies instagram.CookieBundle, opts gridOptions) {
	protocol := inline.Detect()
	if protocol == inline.ProtocolNone {
		for _, item := range items {
			_, _ = fmt.Fprintln(os.Stdout, item.URL)
		}
		return
	}

	gridCols := opts.GridCols
	thumbPx := opts.ThumbPx
	if thumbPx < 64 {
		thumbPx = 64
	}
	paddingPx := opts.PaddingPx
	if paddingPx < 0 {
		paddingPx = 0
	}

	thumbCols := opts.ThumbCols
	if thumbCols <= 0 {
		thumbCols = autoThumbCols(gridCols)
	}
	if gridCols <= 0 {
		gridCols = 1
	}

	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = autoPageSize(gridCols, thumbCols, thumbPx, inline.CellAspectRatio("METCLI_CELL_ASPECT", 0.5))
	}
	if pageSize <= 0 {
		pageSize = len(items)
	}

	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	client := instagram.ImageClient()
	nextID := uint32(1)
	for start := 0; start < len(items); start += pageSize {
		end := start + pageSize
		if end > len(items) {
			end = len(items)
		}
		pageItems := items[start:end]
		images := make([]image.Image, 0, len(pageItems))
		for _, item := range pageItems {
			data, _, _, err := instagram.DownloadImage(context.Background(), client, item.URL, username, cookies)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "[metcli] %s\n", err.Error())
				continue
			}
			img, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "[metcli] decode image: %s\n", err.Error())
				continue
			}
			images = append(images, img)
		}

		if len(images) == 0 {
			continue
		}

		pageCols := gridCols
		if pageCols > len(images) {
			pageCols = len(images)
		}
		gridPNG, gridWidth, gridHeight, err := buildGridPNG(images, pageCols, thumbPx, paddingPx)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "[metcli] %s\n", err.Error())
			continue
		}
		colsCells := pageCols * thumbCols
		rowsCells := estimateRows(colsCells, gridWidth, gridHeight, inline.CellAspectRatio("METCLI_CELL_ASPECT", 0.5))

		switch protocol {
		case inline.ProtocolIterm:
			inline.SendItermInline(writer, inline.ItermFile{
				Name:        "instagram-grid.png",
				Data:        gridPNG,
				WidthCells:  colsCells,
				HeightCells: rowsCells,
				Stretch:     true,
			})
		case inline.ProtocolKitty:
			inline.SendKittyPNG(writer, nextID, gridPNG, colsCells, rowsCells)
			nextID++
		default:
			for _, item := range items {
				_, _ = fmt.Fprintln(os.Stdout, item.URL)
			}
			return
		}
		advanceCursor(writer, rowsCells)
		_ = writer.Flush()
	}
}

func loadInstagramItems(
	ctx context.Context,
	username string,
	profilePath string,
	namesRaw string,
	source string,
	pageSize int,
	max int,
	avatar bool,
	includeVideos bool,
) (instagram.CookieBundle, []instagram.Item, []string, error) {
	names := parseNames(namesRaw)
	cookies, warnings, err := instagram.LoadCookies(ctx, profilePath, names)
	if err != nil {
		return cookies, nil, warnings, err
	}

	profile, err := instagram.FetchProfile(ctx, username, cookies)
	if err != nil {
		return cookies, nil, warnings, err
	}

	normalizedSource := strings.ToLower(strings.TrimSpace(source))
	if normalizedSource == "" {
		normalizedSource = "api"
	}
	switch normalizedSource {
	case "main":
		// keep profile.Media as-is
	case "api":
		media, err := instagram.FetchUserMedia(ctx, username, profile, cookies, max, pageSize)
		if err != nil {
			if len(media) == 0 {
				return cookies, nil, warnings, err
			}
			warnings = append(warnings, fmt.Sprintf("media fetch warning: %s", err.Error()))
		}
		profile.Media = media
	default:
		return cookies, nil, warnings, fmt.Errorf("unsupported source: %s", source)
	}

	items := instagram.BuildItems(profile, avatar, includeVideos)
	if max > 0 && len(items) > max {
		items = items[:max]
	}
	return cookies, items, warnings, nil
}

func buildGridPNG(images []image.Image, cols, thumbPx, paddingPx int) ([]byte, int, int, error) {
	if len(images) == 0 {
		return nil, 0, 0, fmt.Errorf("no images")
	}
	rows := int(math.Ceil(float64(len(images)) / float64(cols)))
	width := cols*thumbPx + (cols-1)*paddingPx
	height := rows*thumbPx + (rows-1)*paddingPx
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))

	for i, img := range images {
		row := i / cols
		col := i % cols
		x := col * (thumbPx + paddingPx)
		y := row * (thumbPx + paddingPx)
		thumb := resizeSquare(img, thumbPx)
		rect := image.Rect(x, y, x+thumbPx, y+thumbPx)
		imagedraw.Draw(canvas, rect, thumb, image.Point{}, imagedraw.Over)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), width, height, nil
}

func resizeSquare(img image.Image, size int) image.Image {
	crop := cropSquare(img)
	thumb := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(thumb, thumb.Bounds(), crop, crop.Bounds(), xdraw.Over, nil)
	return thumb
}

func cropSquare(img image.Image) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	size := width
	if height < size {
		size = height
	}
	x0 := bounds.Min.X + (width-size)/2
	y0 := bounds.Min.Y + (height-size)/2
	rect := image.Rect(x0, y0, x0+size, y0+size)
	if sub, ok := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}); ok {
		return sub.SubImage(rect)
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	imagedraw.Draw(dst, dst.Bounds(), img, rect.Min, imagedraw.Src)
	return dst
}

func estimateRows(colsCells, widthPx, heightPx int, cellAspect float64) int {
	if colsCells <= 0 || widthPx <= 0 || heightPx <= 0 {
		return 0
	}
	aspect := float64(heightPx) / float64(widthPx)
	rows := float64(colsCells) * aspect * cellAspect
	if rows < 1 {
		rows = 1
	}
	return int(math.Round(rows))
}

func autoThumbCols(gridCols int) int {
	if gridCols <= 0 {
		return 12
	}
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 12
	}
	thumbCols := width / gridCols
	if thumbCols < 6 {
		return 6
	}
	return thumbCols
}

func autoPageSize(gridCols, thumbCols, thumbPx int, cellAspect float64) int {
	if gridCols <= 0 {
		return 0
	}
	_, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || rows <= 0 {
		return gridCols * 8
	}
	thumbRows := estimateRows(thumbCols, thumbPx, thumbPx, cellAspect)
	if thumbRows <= 0 {
		return gridCols * 8
	}
	maxTileRows := rows / thumbRows
	if maxTileRows < 1 {
		maxTileRows = 1
	}
	return gridCols * maxTileRows
}

func advanceCursor(out *bufio.Writer, rows int) {
	if out == nil {
		return
	}
	if rows < 1 {
		rows = 1
	}
	for i := 0; i < rows+1; i++ {
		_, _ = fmt.Fprintln(out)
	}
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

func isTerminal(w *os.File) bool {
	return term.IsTerminal(int(w.Fd()))
}

func printWarnings(prefix string, warnings []string) {
	if len(warnings) == 0 {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "%s warnings:\n", prefix)
	for _, warning := range warnings {
		_, _ = fmt.Fprintf(os.Stderr, "- %s\n", warning)
	}
}

func fail(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "[metcli] %s\n", err.Error())
	os.Exit(1)
}
