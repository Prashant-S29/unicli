package cmd

import (
	"fmt"
	"strings"

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
  convert   Convert images between formats

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

// ---- image convert -------------------------------------------------------

var (
	imageConvertTo        string
	imageConvertFrom      string
	imageConvertOutput    string
	imageConvertReplace   bool
	imageConvertRecursive bool
)

var imageConvertCmd = &cobra.Command{
	Use:   "convert [target]",
	Short: "Convert images between formats",
	Long: `Convert one or more images to a different format.

target can be a file, directory, or glob. Defaults to the current directory.

Examples:
  unicli image convert photo.png --to webp
  unicli image convert ./assets --to webp
  unicli image convert ./assets --from png,jpg --to webp
  unicli image convert ./assets --to webp --recursive
  unicli image convert ./assets --to webp -o ./out
  unicli image convert ./assets --to webp --replace
  unicli image convert ./assets --to webp --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if imageConvertTo == "" {
			return fmt.Errorf("--to is required")
		}

		var fromFormats []string
		if imageConvertFrom != "" {
			for _, f := range strings.Split(imageConvertFrom, ",") {
				f = strings.TrimSpace(f)
				if f != "" {
					fromFormats = append(fromFormats, f)
				}
			}
		}

		err := image.RunConvert(image.ConvertRequest{
			Targets:     args,
			ToFormat:    imageConvertTo,
			FromFormats: fromFormats,
			OutputDir:   imageConvertOutput,
			Replace:     imageConvertReplace,
			Recursive:   imageConvertRecursive,
			DryRun:      DryRun,
			Yes:         Yes,
			Quiet:       Quiet,
			Verbose:     Verbose,
		})
		if err != nil {
			ui.Error("image convert failed", err.Error(), "try --verbose for more detail")
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(imageCmd)

	// info
	imageCmd.AddCommand(imageInfoCmd)
	imageInfoCmd.Flags().BoolVar(&imageInfoAll, "all", false, "show all metadata including EXIF")

	// convert
	imageCmd.AddCommand(imageConvertCmd)
	imageConvertCmd.Flags().StringVar(&imageConvertTo, "to", "", "output format (required)")
	imageConvertCmd.Flags().StringVar(&imageConvertFrom, "from", "", "only convert files of this format, comma-separated")
	imageConvertCmd.Flags().StringVarP(&imageConvertOutput, "output", "o", "", "output file or directory")
	imageConvertCmd.Flags().BoolVar(&imageConvertReplace, "replace", false, "overwrite originals in place")
	imageConvertCmd.Flags().BoolVar(&imageConvertRecursive, "recursive", false, "include subdirectories")

	// dynamic completions
	imageConvertCmd.RegisterFlagCompletionFunc("to", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return image.SupportedFormats, cobra.ShellCompDirectiveNoFileComp
	})
	imageConvertCmd.RegisterFlagCompletionFunc("from", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return image.SupportedFormats, cobra.ShellCompDirectiveNoFileComp
	})
}
