package cmd

import (
	"github.com/prashant-s29/unicli/internal/image"
	"github.com/prashant-s29/unicli/internal/ui"
	"github.com/spf13/cobra"
)

// ---- image root command --------------------------------------------------

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Image operations — convert, info and more",
	Long: `A suite of image operations powered by ffmpeg.

Available commands:
  info      Show image metadata and technical details
  convert   Convert images between formats (coming in M9a)

Run 'unicli image <command> --help' for details on each command.`,
}

// ---- image info ----------------------------------------------------------

var imageInfoAll bool

var imageInfoCmd = &cobra.Command{
	Use:   "info [file or directory]",
	Short: "Show image metadata and technical details",
	Long: `Display metadata for one or more image files.

Without --all, shows the essential fields: filename, format, dimensions,
filesize, and color space.

With --all, shows everything ffprobe can read: codec details, bit depth,
color range, frame rate, and all embedded EXIF metadata (camera make/model,
GPS, timestamp, lens info, exposure, ISO, and more).

Examples:
  unicli image info photo.jpg
  unicli image info photo.jpg --all
  unicli image info ./photos
  unicli image info ./photos --all
  unicli image info *.png`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := image.RunInfo(image.InfoRequest{
			Targets: args,
			Full:    imageInfoAll,
			Quiet:   Quiet,
			Verbose: Verbose,
		})
		if err != nil {
			ui.Error("image info failed", err.Error(), "try --verbose for more detail")
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(imageCmd)

	// info subcommand
	imageCmd.AddCommand(imageInfoCmd)
	imageInfoCmd.Flags().BoolVar(&imageInfoAll, "all", false, "show all metadata including EXIF")

	// Static completions for image subcommands are handled automatically
	// by Cobra from the registered command tree.
	//
	// Dynamic completions — format lists source from image.SupportedFormats
	// and are registered here so they're available as soon as cmd/image.go loads.
	// Convert flags (--to, --from) will be registered here when M9a convert lands.
}
