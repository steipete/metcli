package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/steipete/sweetcookie"
)

type outputCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain,omitempty"`
	Path     string `json:"path,omitempty"`
	Expires  *int64 `json:"expires,omitempty"`
	Secure   bool   `json:"secure"`
	HTTPOnly bool   `json:"httpOnly"`
	SameSite string `json:"sameSite,omitempty"`
}

var (
	defaultNames = []string{"sessionid", "csrftoken", "ds_user_id"}
	origins      = []string{"https://www.instagram.com", "https://instagram.com", "https://i.instagram.com"}
)

func main() {
	var (
		formatFlag  = flag.String("format", "header", "header|json")
		outFlag     = flag.String("out", "", "output path")
		profileFlag = flag.String("profile", "", "Chrome profile name/dir or Cookies DB path")
		namesFlag   = flag.String("names", "", "comma-separated cookie names")
		jsonFlag    = flag.Bool("json", false, "shorthand for --format json")
		headerFlag  = flag.Bool("header", false, "shorthand for --format header")
	)

	flag.Usage = func() {
		_, _ = fmt.Fprintln(os.Stdout, "ig-cookies")
		_, _ = fmt.Fprintln(os.Stdout, "\nUsage:\n  ig-cookies [--format header|json] [--out <path>] [--profile <nameOrPath>] [--names <csv>]")
		_, _ = fmt.Fprintf(os.Stdout, "\nDefaults:\n  --format header\n  --names %s\n", strings.Join(defaultNames, ","))
		_, _ = fmt.Fprintln(os.Stdout, "\nExamples:\n  ig-cookies --format json --out /tmp/ig-cookies.json\n  ig-cookies --profile Default\n  ig-cookies --names sessionid,csrftoken,ds_user_id,rur")
	}

	flag.Parse()

	format := strings.ToLower(strings.TrimSpace(*formatFlag))
	if *jsonFlag {
		format = "json"
	}
	if *headerFlag {
		format = "header"
	}
	if format != "json" && format != "header" {
		fail(fmt.Errorf("unsupported format: %s", format))
	}

	names := parseNames(*namesFlag)

	profiles := map[sweetcookie.Browser]string{}
	if strings.TrimSpace(*profileFlag) != "" {
		profiles[sweetcookie.BrowserChrome] = strings.TrimSpace(*profileFlag)
	}

	ctx := context.Background()
	res, err := sweetcookie.Get(ctx, sweetcookie.Options{
		URL:      origins[0],
		Origins:  origins,
		Names:    names,
		Browsers: []sweetcookie.Browser{sweetcookie.BrowserChrome},
		Mode:     sweetcookie.ModeMerge,
		Profiles: profiles,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		fail(err)
	}

	if len(res.Warnings) > 0 {
		_, _ = fmt.Fprintln(os.Stderr, "[ig-cookies] sweetcookie warnings:")
		for _, w := range res.Warnings {
			_, _ = fmt.Fprintf(os.Stderr, "- %s\n", w)
		}
	}

	cookies := dedupeCookies(res.Cookies)
	if len(cookies) == 0 {
		fail(fmt.Errorf("no Instagram cookies found; log into instagram.com in Chrome first"))
	}

	var output string
	if format == "json" {
		payload := make([]outputCookie, 0, len(cookies))
		for _, c := range cookies {
			payload = append(payload, toOutputCookie(c))
		}
		encoded, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			fail(err)
		}
		output = string(encoded)
	} else {
		output = fmt.Sprintf("Cookie: %s", toCookieHeader(cookies))
	}

	if *outFlag != "" {
		resolved := *outFlag
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(mustCwd(), resolved)
		}
		if err := os.WriteFile(resolved, []byte(output+"\n"), 0o644); err != nil {
			fail(err)
		}
		_, _ = fmt.Fprintf(os.Stderr, "[ig-cookies] wrote %d cookies to %s\n", len(cookies), resolved)
		return
	}

	_, _ = fmt.Fprintln(os.Stdout, output)
}

func parseNames(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultNames
	}
	parts := strings.Split(trimmed, ",")
	names := make([]string, 0, len(parts))
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
		names = append(names, name)
	}
	if len(names) == 0 {
		return defaultNames
	}
	return names
}

func dedupeCookies(cookies []sweetcookie.Cookie) []sweetcookie.Cookie {
	seen := map[string]struct{}{}
	out := make([]sweetcookie.Cookie, 0, len(cookies))
	for _, c := range cookies {
		key := c.Domain + ":" + c.Name
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	return out
}

func toCookieHeader(cookies []sweetcookie.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		parts = append(parts, fmt.Sprintf("%s=%s", c.Name, c.Value))
	}
	return strings.Join(parts, "; ")
}

func toOutputCookie(cookie sweetcookie.Cookie) outputCookie {
	var expires *int64
	if cookie.Expires != nil {
		value := cookie.Expires.Unix()
		expires = &value
	}
	return outputCookie{
		Name:     cookie.Name,
		Value:    cookie.Value,
		Domain:   cookie.Domain,
		Path:     cookie.Path,
		Expires:  expires,
		Secure:   cookie.Secure,
		HTTPOnly: cookie.HTTPOnly,
		SameSite: string(cookie.SameSite),
	}
}

func mustCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	return cwd
}

func fail(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "[ig-cookies] %s\n", err.Error())
	os.Exit(1)
}
