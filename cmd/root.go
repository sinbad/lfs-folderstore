// Copyright © 2018 Steve Streeting
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/sinbad/lfs-folderstore/service"
	"github.com/spf13/cobra"
)

var (
	baseDir      string
	printVersion bool
)

// RootCmd represents the base command when called without any subcommands
var RootCmd *cobra.Command

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	RootCmd = &cobra.Command{
		Use:   "lfs-folderstore",
		Short: "git-lfs custom transfer adapter to store all data in a folder",
		Long: `lfs-folderstore treats a simple folder, probably a shared one,
		as the remote store for all LFS object data. Upload and download functions
		are turned into simple file copies to destinations determined by the id
		of the object.`,
		Run: rootCommand,
	}

	RootCmd.Flags().StringVarP(&baseDir, "basedir", "d", "", "Base directory for all file operations")
	RootCmd.Flags().BoolVarP(&printVersion, "version", "", false, "Print version")
	RootCmd.SetUsageFunc(usageCommand)

}

func usageCommand(cmd *cobra.Command) error {
	usage := `
Usage:
  lfs-folderstore [options] <basedir>

Arguments:
  basedir      Base directory for the object store (required)

Options:
  --version    Report the version number and exit

Note:
  This tool should only be called by git-lfs as documented in Custom Transfers:
  https://github.com/git-lfs/git-lfs/blob/master/docs/custom-transfers.md

  The arguments should be provided via gitconfig at lfs.customtransfer.<name>.args
`
	fmt.Fprintf(os.Stderr, usage)
	return nil
}

func rootCommand(cmd *cobra.Command, args []string) {

	if printVersion {
		os.Stderr.WriteString(fmt.Sprintf("lfs-folder %v", Version))
		os.Exit(0)
	}
	var baseDir string
	if len(args) > 0 {
		baseDir = strings.TrimSpace(args[0])
	}
	if len(baseDir) == 0 {
		os.Stderr.WriteString("Required: base directory")
		cmd.Usage()
		os.Exit(1)
	}
	stat, err := os.Stat(baseDir)
	if err != nil || !stat.IsDir() {
		os.Stderr.WriteString(fmt.Sprintf("%q does not exist or is not a directory", baseDir))
		cmd.Usage()
		os.Exit(3)
	}
	service.Serve(baseDir, os.Stdin, os.Stdout, os.Stderr)
}
