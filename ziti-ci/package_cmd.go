package main

import (
	"archive/tar"
	"compress/gzip"
	"github.com/spf13/cobra"
	"io"
	"os"
)

type packageCmd struct {
	baseCommand
}

func (cmd *packageCmd) execute() {
	outputFile, err := os.Create(cmd.args[0])
	if err != nil {
		cmd.failf("unexpected err trying to write to %v. err: %+v", cmd.args[0], err)
	}
	gzw := gzip.NewWriter(outputFile)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	for _, fileName := range cmd.args[1:] {
		file, err := os.Open(fileName)
		if err != nil {
			cmd.failf("unexpected err trying to open file %v. err: %+v", fileName, err)
		}
		fileInfo, err := file.Stat()
		if err != nil {
			file.Close()
			cmd.failf("unexpected err trying to read state file %v. err: %+v", fileName, err)
		}

		header, err := tar.FileInfoHeader(fileInfo, file.Name())
		if err != nil {
			file.Close()
			cmd.failf("unexpected err trying to create tar header for %v. err: %+v", fileName, err)
		}
		if err = tw.WriteHeader(header); err != nil {
			file.Close()
			cmd.failf("unexpected err trying to write tar header for %v. err: %+v", fileName, err)
		}

		_, err = io.Copy(tw, file)
		file.Close()
		if err != nil {
			cmd.failf("unexpected err trying to write file %v to tar file. err: %+v", fileName, err)
		}
	}
}

func newPackageCmd(root *rootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "package <destination> <files>",
		Short: "Packages files for release",
		Args:  cobra.MinimumNArgs(2),
	}

	result := &packageCmd{
		baseCommand: baseCommand{
			rootCommand: root,
			cmd:         cobraCmd,
		},
	}

	return finalize(result)
}
