package instagram

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/steipete/sweetcookie"
)

var (
	defaultCookieNames = []string{"sessionid", "csrftoken", "ds_user_id", "rur"}
	cookieOrigins      = []string{"https://www.instagram.com", "https://instagram.com", "https://i.instagram.com"}
)

type CookieBundle struct {
	Header    string
	CSRFToken string
	Cookies   []sweetcookie.Cookie
}

func DefaultCookieNames() []string {
	out := make([]string, len(defaultCookieNames))
	copy(out, defaultCookieNames)
	return out
}

func LoadCookies(
	ctx context.Context,
	chromeProfile string,
	names []string,
) (CookieBundle, []string, error) {
	resolvedNames := normalizeNames(names)
	profiles := map[sweetcookie.Browser]string{}
	if strings.TrimSpace(chromeProfile) != "" {
		profiles[sweetcookie.BrowserChrome] = strings.TrimSpace(chromeProfile)
	}

	res, err := sweetcookie.Get(ctx, sweetcookie.Options{
		URL:      cookieOrigins[0],
		Origins:  cookieOrigins,
		Names:    resolvedNames,
		Browsers: []sweetcookie.Browser{sweetcookie.BrowserChrome},
		Mode:     sweetcookie.ModeMerge,
		Profiles: profiles,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		return CookieBundle{}, nil, err
	}

	selected := selectBestCookies(res.Cookies)
	if len(selected) == 0 {
		return CookieBundle{}, res.Warnings, fmt.Errorf("no Instagram cookies found; log into instagram.com in Chrome first")
	}

	orderedNames := resolvedNames
	if len(orderedNames) == 0 {
		orderedNames = make([]string, 0, len(selected))
		for name := range selected {
			orderedNames = append(orderedNames, name)
		}
		sort.Strings(orderedNames)
	}

	parts := make([]string, 0, len(selected))
	for _, name := range orderedNames {
		cookie, ok := selected[name]
		if !ok {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}

	csrf := ""
	if cookie, ok := selected["csrftoken"]; ok {
		csrf = cookie.Value
	}

	return CookieBundle{
		Header:    strings.Join(parts, "; "),
		CSRFToken: csrf,
		Cookies:   mapValues(selected),
	}, res.Warnings, nil
}

func normalizeNames(names []string) []string {
	if len(names) == 0 {
		return DefaultCookieNames()
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func selectBestCookies(cookies []sweetcookie.Cookie) map[string]sweetcookie.Cookie {
	selected := map[string]sweetcookie.Cookie{}
	selectedScore := map[string]int{}
	for _, cookie := range cookies {
		name := strings.TrimSpace(cookie.Name)
		if name == "" {
			continue
		}
		score := scoreCookie(cookie)
		if existingScore, ok := selectedScore[name]; ok && existingScore >= score {
			continue
		}
		selected[name] = cookie
		selectedScore[name] = score
	}
	return selected
}

func scoreCookie(cookie sweetcookie.Cookie) int {
	score := 0
	domain := strings.ToLower(strings.TrimSpace(cookie.Domain))
	if strings.HasPrefix(domain, ".") {
		score += 1
	}
	if strings.HasSuffix(domain, "instagram.com") {
		score += 10
	}
	if strings.TrimSpace(cookie.Path) == "/" {
		score += 1
	}
	if cookie.Secure {
		score += 1
	}
	return score
}

func mapValues(cookies map[string]sweetcookie.Cookie) []sweetcookie.Cookie {
	out := make([]sweetcookie.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		out = append(out, cookie)
	}
	return out
}
