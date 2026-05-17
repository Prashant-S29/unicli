// Copyright © 2026 Prashant Singh
package cmd

import (
	"github.com/prashant-s29/unicli/internal/download"
	"github.com/prashant-s29/unicli/internal/ui"
	"github.com/spf13/cobra"
)

// Flag values
var (
	downloadOutput    string
	downloadFormat    string
	downloadQuality   string
	downloadAudioOnly bool
	downloadNoMeta    bool
)

var downloadCmd = &cobra.Command{
	Use:   "download <url>",
	Short: "Download from any URL — YouTube, Instagram, Twitter/X, direct files and more",
	Long: `Auto-detects the platform and routes to the right engine.
Supports YouTube, Instagram, Twitter/X, TikTok, Reddit, Vimeo,
direct file URLs (.zip, .pdf, .mp4, etc.), and 1000+ other platforms.

Examples:
  unicli download https://youtube.com/watch?v=...
  unicli download https://instagram.com/p/... -o ~/Downloads
  unicli download https://example.com/file.zip
  unicli download https://youtube.com/watch?v=... --audio-only
  unicli download https://youtube.com/watch?v=... --quality 1080p
  unicli download https://youtube.com/watch?v=... --dry-run`,
	Args: cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		err := download.Run(download.Request{
			URL:       args[0],
			OutputDir: downloadOutput,
			Format:    downloadFormat,
			Quality:   downloadQuality,
			AudioOnly: downloadAudioOnly,
			NoMeta:    downloadNoMeta,
			DryRun:    DryRun,
			Quiet:     Quiet,
			Verbose:   Verbose,
		})
		if err != nil {
			ui.Error("Download failed", err.Error(), "try --verbose for more detail")
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)
	downloadCmd.Flags().StringVarP(&downloadOutput, "output", "o", "", "output directory (default: current directory)")
	downloadCmd.Flags().StringVarP(&downloadFormat, "format", "f", "", "force output format (e.g. mp4, mp3, webm)")
	downloadCmd.Flags().StringVar(&downloadQuality, "quality", "", "video quality (e.g. best, 1080p, 720p)")
	downloadCmd.Flags().BoolVar(&downloadAudioOnly, "audio-only", false, "extract audio only")
	downloadCmd.Flags().BoolVar(&downloadNoMeta, "no-metadata", false, "skip embedding metadata")

	// Dynamic completions for flag values
	_ = downloadCmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"mp4", "mp3", "webm", "mkv", "m4a", "flac", "wav", "ogg"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = downloadCmd.RegisterFlagCompletionFunc("quality", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"best", "1080p", "720p", "480p", "360p", "240p"}, cobra.ShellCompDirectiveNoFileComp
	})
}
