package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	imagedraw "image/draw"
	"image/png"
	"math"
	"os"
	"path"
	"strings"

	"github.com/steipete/metcli/internal/inline"
	"github.com/steipete/metcli/internal/instagram"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/term"
)

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

func (cmd *InstagramHomeCmd) runInlineStream(ctx context.Context) error {
	names := parseNames(cmd.Names)
	cookies, warnings, err := instagram.LoadCookies(ctx, cmd.Profile, names)
	if err != nil {
		return err
	}
	printWarnings("[metcli]", warnings)

	protocol := inline.Detect()
	cols := cmd.ThumbCols
	if cols <= 0 {
		cols = autoStreamCols()
	}
	cellAspect := inline.CellAspectRatio("METCLI_CELL_ASPECT", 0.5)

	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	client := instagram.ImageClient()
	nextID := uint32(1)
	rendered := 0
	_, err = instagram.StreamHomeFeed(ctx, cookies, cmd.Max, cmd.PageSize, cmd.IncludeVideos, func(item instagram.MediaItem) error {
		if item.URL == "" {
			return nil
		}

		if protocol == inline.ProtocolNone {
			if cmd.Text {
				renderItemText(writer, item)
			}
			_, _ = fmt.Fprintln(writer, item.URL)
			_, _ = fmt.Fprintln(writer)
			_ = writer.Flush()
			rendered++
			return nil
		}

		if cmd.Text {
			renderItemText(writer, item)
		}

		data, width, height, err := instagram.DownloadImage(ctx, client, item.URL, item.Username, cookies)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "[metcli] %s\n", err.Error())
			return nil
		}
		if protocol == inline.ProtocolKitty {
			data, err = instagram.EnsurePNG(data)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "[metcli] %s\n", err.Error())
				return nil
			}
		}
		if width <= 0 || height <= 0 {
			cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
			if err == nil {
				width = cfg.Width
				height = cfg.Height
			}
		}
		if width <= 0 || height <= 0 {
			width = 1
			height = 1
		}

		rows := estimateRows(cols, width, height, cellAspect)
		if rows < 1 {
			rows = 1
		}

		switch protocol {
		case inline.ProtocolIterm:
			inline.SendItermInline(writer, inline.ItermFile{
				Name:        inlineName(item.Shortcode),
				Data:        data,
				WidthCells:  cols,
				HeightCells: rows,
				Stretch:     true,
			})
		case inline.ProtocolKitty:
			inline.SendKittyPNG(writer, nextID, data, cols, rows)
			nextID++
		default:
			_, _ = fmt.Fprintln(os.Stdout, item.URL)
			return nil
		}

		advanceCursor(writer, rows)
		_, _ = fmt.Fprintln(writer)
		_ = writer.Flush()
		rendered++
		return nil
	})
	if err != nil {
		if rendered == 0 {
			return err
		}
		_, _ = fmt.Fprintf(os.Stderr, "[metcli] home feed warning: %s\n", err.Error())
	}
	if rendered == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "[metcli] no images to render")
	}
	return nil
}

func renderItemText(writer *bufio.Writer, item instagram.MediaItem) {
	if writer == nil {
		return
	}
	username := stripTerminalControls(strings.TrimSpace(item.Username))
	caption := stripTerminalControls(compactWhitespace(item.Caption))
	if username != "" {
		_, _ = fmt.Fprintf(writer, "@%s\n", username)
	}
	if caption != "" {
		_, _ = fmt.Fprintln(writer, caption)
	}
	if username != "" || caption != "" {
		_, _ = fmt.Fprintln(writer)
	}
}

func stripTerminalControls(input string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return -1
		}
		return r
	}, input)
}

func compactWhitespace(input string) string {
	fields := strings.Fields(strings.TrimSpace(input))
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

func inlineName(shortcode string) string {
	name := strings.TrimSpace(shortcode)
	if name == "" {
		name = "instagram"
	}
	return path.Base(name + ".img")
}

func autoStreamCols() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 80
	}
	if width < 20 {
		return width
	}
	if width > 100 {
		return 100
	}
	return width
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
