package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestXcodeAssetCompilation(t *testing.T) {
	if os.Getenv("ICONOTTER_TEST_XCODE") != "1" {
		t.Skip("set ICONOTTER_TEST_XCODE=1 to verify with Xcode")
	}
	opts := defaults(t)
	opts.Type = "launcher"
	opts.Platforms = []string{"ios"}
	opts.Name = "AppIcon"
	output, err := generate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	compile := t.TempDir()
	command := exec.Command("xcrun", "actool", filepath.Join(output, "ios", "Assets.xcassets"), "--compile", compile, "--platform", "iphoneos", "--minimum-deployment-target", "15.0", "--app-icon", "AppIcon", "--target-device", "iphone", "--target-device", "ipad", "--output-partial-info-plist", filepath.Join(compile, "icon-info.plist"))
	if body, err := command.CombinedOutput(); err != nil {
		t.Fatalf("actool: %v\n%s", err, body)
	}
	if _, err := os.Stat(filepath.Join(compile, "Assets.car")); err != nil {
		t.Fatal("Xcode did not compile an asset catalog:", err)
	}
}
