package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

func selectPlatforms(kind string, requested []string) ([]string, error) {
	if kind != "launcher" && kind != "notification" {
		return nil, errors.New("Choose the launcher or notification command.")
	}
	if len(requested) == 0 {
		return nil, errors.New("Choose android, ios, macos, windows, linux or web with --platform.")
	}
	selected := make([]string, 0, 6)
	seen := map[string]bool{}
	for _, value := range requested {
		platform := strings.ToLower(strings.TrimSpace(value))
		if platform != "android" && platform != "ios" && platform != "macos" && platform != "windows" && platform != "linux" && platform != "web" {
			return nil, fmt.Errorf("Unknown platform %q. Choose android, ios, macos, windows, linux or web with --platform.", value)
		}
		if kind == "notification" && platform != "android" {
			return nil, fmt.Errorf("%s notification icons are not supported. Use 'iconotter launcher --platform %s' instead.", platform, platform)
		}
		if !seen[platform] {
			selected = append(selected, platform)
			seen[platform] = true
		}
	}
	return selected, nil
}

func platformName(name, kind, platform string) string {
	if name != "" {
		return name
	}
	if platform == "ios" || platform == "macos" || platform == "windows" {
		return "AppIcon"
	}
	if platform == "linux" {
		return "app-icon"
	}
	if kind == "notification" {
		return "ic_stat_notification"
	}
	return "ic_launcher"
}

// Publish all requested platforms together so an export cannot leave a partial
// bundle behind when any platform rejects the artwork or options.
func generate(ctx context.Context, opts options) (string, error) {
	platforms, err := selectPlatforms(opts.Type, opts.Platforms)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(opts.Output) == "" {
		return "", errors.New("Choose a new output folder with --output DIR.")
	}
	output, err := filepath.Abs(opts.Output)
	if err != nil {
		return "", err
	}
	if err := newDestination(output); err != nil {
		return "", err
	}
	return publishBundle(ctx, output, func(root string) error {
		for _, platform := range platforms {
			target := opts
			target.Name = platformName(opts.Name, opts.Type, platform)
			target.Output = filepath.Join(root, platform)
			if platform == "android" {
				_, err = generateAndroid(ctx, target)
			} else if platform == "macos" {
				err = writeMacOS(ctx, target)
			} else if platform == "web" {
				err = writeWeb(ctx, target)
			} else if platform == "windows" {
				err = writeWindows(ctx, target)
			} else if platform == "linux" {
				err = writeLinux(ctx, target)
			} else {
				err = writeIOS(ctx, target)
			}
			if err != nil {
				return err
			}
		}
		guide := "# Icon Otter\n\nEach platform folder contains resources, a preview, and integration instructions. Copy resources into the corresponding native project; the output works with native apps, React Native, Flutter, and Compose Multiplatform. Review existing resources with the same names before replacing them.\n"
		return writeBundleFile(ctx, root, "README.md", []byte(guide))
	})
}
