package cmd

import (
	"fmt"
	"os"

	"github.com/2mal3/iris/pkg"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "iris",
	Short:         "Moin",
	Long:          "",
	RunE:          run,
	SilenceErrors: true,
}

var inputPaths []string
var outputPath string
var moveFiles bool
var removeDuplicates bool

func init() {
	rootCmd.Flags().StringSliceVarP(&inputPaths, "inputPaths", "i", []string{}, "Input paths")
	rootCmd.MarkFlagRequired("inputPaths")

	rootCmd.Flags().StringVarP(&outputPath, "outputPath", "o", "", "Output path")
	rootCmd.MarkFlagRequired("outputPath")

	rootCmd.Flags().BoolVarP(&moveFiles, "moveFiles", "m", false, "Move files")

	rootCmd.Flags().BoolVarP(&removeDuplicates, "removeDuplicates", "r", false, "Remove duplicate files")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Fatal:", err.Error())
		os.Exit(1)
	}
	rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) error {
	config := pkg.Config{
		InputPaths:       inputPaths,
		OutputPath:       outputPath,
		MoveFiles:        moveFiles,
		RemoveDuplicates: removeDuplicates,
	}

	if err := pkg.Main(config); err != nil {
		return err
	}
	return nil
}
