package sapp

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Theme holds tokens from the service provider's branding (the bootstrap's client.settings.branding): colours with
// a readable text colour on each, plus type, radius and spacing.
type Theme struct {
	DisplayName string
	LogoURL     string
	Color       map[string]string
	Radius      map[string]float64
	Spacing     map[string]float64
	Type        map[string]float64
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func channel(hex string, i int) float64 {
	v, _ := strconv.ParseUint(hex[1+i*2:3+i*2], 16, 8)
	return float64(v)
}

func luminance(hex string) float64 {
	c := [3]float64{}
	for i := range c {
		v := channel(hex, i) / 255
		if v <= 0.03928 {
			c[i] = v / 12.92
		} else {
			c[i] = math.Pow((v+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*c[0] + 0.7152*c[1] + 0.0722*c[2]
}

func mix(hex string, target, amount float64) string {
	out := "#"
	for i := 0; i < 3; i++ {
		v := channel(hex, i)
		out += fmt.Sprintf("%02X", int(math.Floor(v+(target-v)*amount+0.5)))
	}
	return out
}

// ThemeFromBranding answers theme tokens for the branding. Rebranding is configuration, never code.
func ThemeFromBranding(branding map[string]any) Theme {
	str := func(k string) string { s, _ := branding[k].(string); return s }
	primary := "#E02020"
	if p := str("primary_color"); hexColor.MatchString(p) {
		primary = strings.ToUpper(p)
	}
	onPrimary := "#FFFFFF"
	if luminance(primary) > 0.4 {
		onPrimary = "#111111"
	}
	return Theme{
		DisplayName: str("display_name"),
		LogoURL:     str("logo_url"),
		Color: map[string]string{
			"primary": primary, "onPrimary": onPrimary,
			"primaryContainer": mix(primary, 255, 0.85), "onPrimaryContainer": mix(primary, 0, 0.55),
			"surface": "#FFFFFF", "onSurface": "#1C1C1E", "muted": "#6B7280",
			"success": "#16A34A", "warning": "#FD7E14", "danger": "#DC2626",
		},
		Radius:  map[string]float64{"small": 6, "medium": 10, "large": 16},
		Spacing: map[string]float64{"xs": 4, "s": 8, "m": 12, "l": 16, "xl": 24},
		Type:    map[string]float64{"body": 15, "small": 13, "title": 20, "headline": 26},
	}
}
