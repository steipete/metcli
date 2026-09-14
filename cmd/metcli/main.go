package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/steipete/metcli/internal/inline"
	"github.com/steipete/metcli/internal/instagram"
	"github.com/steipete/metcli/internal/version"
	"golang.org/x/term"
)

type CLI struct {
	Version   kong.VersionFlag `help:"Print version and exit"`
	Instagram InstagramCmd     `cmd:"" help:"Instagram helpers"`
}

type InstagramCmd struct {
	Profile InstagramProfileCmd `cmd:"" help:"Show profile images"`
	Feed    InstagramFeedCmd    `cmd:"" help:"Show feed images"`
	Home    InstagramHomeCmd    `cmd:"" help:"Show home timeline images"`
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

type InstagramHomeCmd struct {
	Format        string `help:"url|inline|json" default:"inline"`
	Inline        bool   `help:"shorthand for --format inline"`
	URL           bool   `help:"shorthand for --format url"`
	JSON          bool   `help:"shorthand for --format json"`
	Max           int    `help:"max items (0 = all)" default:"0"`
	IncludeVideos bool   `help:"include video thumbnails" default:"true" negatable:""`
	Text          bool   `help:"show username + caption" default:"true" negatable:""`
	PageSize      int    `help:"items per API page (1-50)" default:"50"`
	Profile       string `help:"Chrome profile name/dir or Cookies DB path"`
	Names         string `help:"comma-separated cookie names"`
	GridCols      int    `help:"grid columns" default:"4"`
	ThumbCols     int    `help:"thumb width in cells (0 = auto)" default:"0"`
	ThumbPx       int    `help:"thumbnail size in px" default:"256"`
	PaddingPx     int    `help:"padding between thumbs in px" default:"8"`
	PageGridSize  int    `help:"images per grid page (0 = auto)" default:"0"`
}

type outputItem struct {
	URL       string `json:"url"`
	Kind      string `json:"kind"`
	IsVideo   bool   `json:"is_video"`
	Shortcode string `json:"shortcode,omitempty"`
	TakenAt   int64  `json:"taken_at,omitempty"`
	Username  string `json:"username,omitempty"`
	Caption   string `json:"caption,omitempty"`
}

func main() {
	cli := CLI{}
	ctx := kong.Parse(&cli, kong.Name("metcli"), kong.UsageOnError(), kong.Vars{"version": "metcli " + version.Version})
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
	case "instagram home":
		if err := cli.Instagram.Home.Run(); err != nil {
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
				Username:  item.Username,
				Caption:   item.Caption,
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
				Username:  item.Username,
				Caption:   item.Caption,
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

func (cmd *InstagramHomeCmd) Run() error {
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
	if format == "inline" {
		return cmd.runInlineStream(ctx)
	}
	_, items, warnings, err := loadHomeItems(
		ctx,
		cmd.Profile,
		cmd.Names,
		cmd.PageSize,
		cmd.Max,
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
				Username:  item.Username,
				Caption:   item.Caption,
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
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	return nil
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

func loadHomeItems(
	ctx context.Context,
	profilePath string,
	namesRaw string,
	pageSize int,
	max int,
	includeVideos bool,
) (instagram.CookieBundle, []instagram.Item, []string, error) {
	names := parseNames(namesRaw)
	cookies, warnings, err := instagram.LoadCookies(ctx, profilePath, names)
	if err != nil {
		return cookies, nil, warnings, err
	}

	media, err := instagram.FetchHomeFeed(ctx, cookies, max, pageSize)
	if err != nil {
		if len(media) == 0 {
			return cookies, nil, warnings, err
		}
		warnings = append(warnings, fmt.Sprintf("home feed warning: %s", err.Error()))
	}

	profile := instagram.Profile{Media: media}
	items := instagram.BuildItems(profile, false, includeVideos)
	if max > 0 && len(items) > max {
		items = items[:max]
	}
	return cookies, items, warnings, nil
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
