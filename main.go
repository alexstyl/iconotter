package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"

	"charm.land/fang/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

// version is set by release builds; go install builds report their module version.
var version string

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := execute(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}

func execute(ctx context.Context, args []string, in io.Reader, out, stderr io.Writer) error {
	root := newCommand()
	root.SetArgs(args)
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(stderr)
	return fang.Execute(ctx, root, fang.WithVersion(version), fang.WithoutManpage(), fang.WithoutCompletions(), fang.WithErrorHandler(func(w io.Writer, styles fang.Styles, err error) {
		_, noColor := os.LookupEnv("NO_COLOR")
		if !noColor && os.Getenv("TERM") != "dumb" && terminal(stderr) {
			fang.DefaultErrorHandler(w, styles, err)
		} else {
			fmt.Fprintln(w, err)
		}
	}))
}

func terminal(stream any) bool {
	f, ok := stream.(*os.File)
	return ok && term.IsTerminal(f.Fd())
}

func newCommand() *cobra.Command {
	root := &cobra.Command{
		Use: "iconotter", Short: "Icon Otter",
		Long:         "Create platform-ready icons from your artwork. All files are created locally; no account is required.",
		SilenceUsage: true, SilenceErrors: true,
		Example: "  iconotter launcher --foreground logo.png --background '#FFFFFF' --platform android,ios\n  iconotter notification --foreground bell.svg --platform android",
	}
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return fmt.Errorf("%w\nRun '%s --help' for usage.", err, cmd.CommandPath())
	})
	root.AddCommand(newIconCommand("launcher"), newIconCommand("notification"))
	return root
}

func newIconCommand(kind string) *cobra.Command {
	opts := options{Type: kind}
	var padding float64
	defaults := []string{"android", "ios"}
	output, size := "app-icons", 56.0
	short := "Create Android, iOS, macOS, Windows, Linux and web icons"
	long := "Create Android launcher icons, an iOS asset catalog, macOS ICNS and iconsets, Windows ICO, Linux PNG icons, or web favicons.\nWindows and Linux icons keep source transparency unless --background is supplied.\n--background takes a #RRGGBB color or an opaque image, which is scaled to cover the icon and cropped to its center. macOS glass backgrounds use Apple’s Icon Composer.\nTransparent padding is trimmed and the mark is centered. Use --monochrome for multicolor Android artwork."
	example := "  iconotter launcher --foreground logo.png --background '#FFFFFF' --platform android,ios\n  iconotter launcher --foreground logo.svg --platform macos --name Studio --output mac-icons\n  iconotter launcher --foreground logo.svg --platform windows,linux --name my-app --output desktop-icons\n  iconotter launcher --foreground logo.svg --platform web --output web-icons\n  iconotter launcher --foreground mascot.png --background pattern.png --platform ios,macos"
	if kind == "notification" {
		defaults, output, size = []string{"android"}, "notification-icons", 22
		short = "Create Android notification small icons"
		long = "Create white notification icons on a transparent 24dp canvas for Android's status bar.\nSupply single-color artwork, or a dedicated silhouette with --monochrome. iOS uses its app icon for notifications."
		example = "  iconotter notification --foreground bell.svg --platform android"
	}
	cmd := &cobra.Command{
		Use: kind, Short: short, Long: long, Example: example, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.BackgroundSet = cmd.Flags().Changed("background")
			if kind == "launcher" && cmd.Flags().Changed("padding") {
				if cmd.Flags().Changed("size") {
					return errors.New("Choose either --padding or --size.")
				}
				opts.Padding = &padding
			}
			platforms, err := selectPlatforms(opts.Type, opts.Platforms)
			if err != nil {
				return err
			}
			opts.Platforms = platforms
			if kind == "launcher" && cmd.Flags().Changed("macos-style") && !slices.Contains(platforms, "macos") {
				return errors.New("--macos-style applies to macOS. Include macos in --platform.")
			}
			if kind == "launcher" && !slices.Contains(platforms, "android") {
				for _, flag := range []string{"monochrome", "min-sdk", "size"} {
					if cmd.Flags().Changed(flag) {
						return fmt.Errorf("--%s applies to Android. Remove it or include android in --platform.", flag)
					}
				}
			}
			if opts.Foreground == "" && os.Getenv("TERM") != "dumb" && terminal(cmd.InOrStdin()) && terminal(cmd.OutOrStdout()) {
				if err := promptArtwork(cmd, &opts); err != nil {
					return err
				}
			}
			if strings.TrimSpace(opts.Foreground) == "" {
				return errors.New("Choose your artwork with --foreground FILE.")
			}
			path, err := generate(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Created %s icons in %s\n", kind, path); err != nil {
				return err
			}
			for _, platform := range platforms {
				name := platformName(opts.Name, kind, platform)
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Preview: %s/%s/preview.png\n", path, platform); err != nil {
					return err
				}
				switch {
				case platform == "windows":
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "Use windows/%s.ico as your Windows application icon.\n", name)
				case platform == "linux":
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "Use linux/%s.png as your Linux application icon. For desktop menus, install the matching files from linux/hicolor into the hicolor theme.\n", name)
				case platform == "web":
					_, err = fmt.Fprintln(cmd.OutOrStdout(), "Copy web/favicon.ico, web/favicon-16x16.png, web/favicon-32x32.png, and web/apple-touch-icon.png into your site's public root. Link the PNG and touch icons from your page head.")
				case platform == "macos":
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "Use macos/%s.icns as your macOS app icon, or add macos/Assets.xcassets/%s.appiconset to Xcode.\n", name, name)
				case platform == "ios":
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "Add ios/Assets.xcassets/%s.appiconset to your asset catalog and select %s as your app icon.\n", name, name)
				case kind == "notification":
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "Copy android/res into your app's source set. Set the notification's small icon with:\n  .setSmallIcon(R.drawable.%s)\n", name)
				default:
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "Copy android/res into your app's source set. Set these attributes on <application> in AndroidManifest.xml:\n  android:icon=\"@mipmap/%s\"\n  android:roundIcon=\"@mipmap/%s_round\"\n", name, name)
				}
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
	platformHelp := "Target platforms: android,ios,macos,windows,linux,web (comma-separated or repeated; default android,ios)"
	nameHelp := "Icon name (defaults: ic_launcher on Android, AppIcon on iOS/macOS/Windows, app-icon on Linux)"
	monoHelp := "Single-color silhouette for Android themed icons"
	if kind == "notification" {
		platformHelp = "Target platform: android"
		nameHelp = "Android resource name (default ic_stat_notification)"
		monoHelp = "Single-color silhouette for the notification icon"
	}
	cmd.Flags().StringSliceVar(&opts.Platforms, "platform", defaults, platformHelp)
	cmd.Flags().StringVar(&opts.Foreground, "foreground", "", "Artwork file (SVG, PNG, or JPEG)")
	cmd.Flags().StringVarP(&opts.Output, "output", "o", output, "New folder for platform resources and previews")
	cmd.Flags().StringVar(&opts.Name, "name", "", nameHelp)
	cmd.Flags().StringVar(&opts.Monochrome, "monochrome", "", monoHelp)
	cmd.Flags().IntVar(&opts.MinSDK, "min-sdk", 21, "Minimum Android API level")
	sizeHelp := "Android mark bounding diagonal in dp (48–66)"
	if kind == "notification" {
		sizeHelp = "Mark's longest side in dp (1–24)"
	}
	cmd.Flags().Float64Var(&opts.MarkSize, "size", size, sizeHelp)
	if kind == "launcher" {
		cmd.Flags().StringVar(&opts.MacOSStyle, "macos-style", "glass", "macOS style: glass (Apple-rendered background, flat foreground) or flat")
		cmd.Flags().StringVar(&opts.Background, "background", "#FFFFFF", "Opaque background color (#RRGGBB) or image file (SVG, PNG, or JPEG)")
		cmd.Flags().Float64Var(&padding, "padding", 0, "Content inset per side as a percent of the visible icon; negative values crop artwork")
	}
	return cmd
}

func promptArtwork(cmd *cobra.Command, opts *options) error {
	fields := []huh.Field{huh.NewInput().Title("Where is your icon artwork?").Placeholder("fox.svg").Value(&opts.Foreground).Validate(func(value string) error {
		if strings.TrimSpace(value) == "" {
			return errors.New("Enter an SVG or PNG file path.")
		}
		return nil
	})}
	needsBackground := slices.Contains(opts.Platforms, "android") || slices.Contains(opts.Platforms, "ios") || slices.Contains(opts.Platforms, "macos") || slices.Contains(opts.Platforms, "web")
	if opts.Type != "notification" && !cmd.Flags().Changed("background") && needsBackground {
		fields = append(fields, huh.NewInput().Title("Background color or image").Value(&opts.Background).Validate(validateBackground))
		opts.BackgroundSet = true
	}
	form := huh.NewForm(huh.NewGroup(fields...)).WithInput(cmd.InOrStdin()).WithOutput(cmd.OutOrStdout())
	if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
		form.WithTheme(huh.ThemeFunc(huh.ThemeBase)).WithAccessible(true)
	}
	if os.Getenv("ACCESSIBLE") == "1" || os.Getenv("ACCESSIBLE") == "true" {
		form.WithAccessible(true)
	}
	if err := form.RunWithContext(cmd.Context()); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return errors.New("Canceled.")
		}
		return err
	}
	return nil
}

// validateBackground checks a prompted background without decoding images:
// generation reports problems with the image itself.
func validateBackground(value string) error {
	if strings.HasPrefix(value, "#") {
		_, err := parseColor(value)
		return err
	}
	if _, err := os.Stat(value); err != nil {
		return errors.New("Enter a color in #RRGGBB format, or the path to a background image.")
	}
	return nil
}
