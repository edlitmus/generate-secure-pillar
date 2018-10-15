// Copyright © 2018 Everbridge, Inc.
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
	"github.com/Everbridge/generate-secure-pillar/sls"
	"github.com/Everbridge/generate-secure-pillar/utils"
	"github.com/spf13/cobra"
)

// rotateCmd represents the rotate command
var rotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "decrypt existing files and re-encrypt with a new key",
	Run: func(cmd *cobra.Command, args []string) {
		pk := getPki()
		recurseDir = cmd.Flag("dir").Value.String()
		inputFilePath = cmd.Flag("file").Value.String()

		if inputFilePath != "" {
			s := sls.New(inputFilePath, pk, topLevelElement)
			buf, err := s.PerformAction("rotate")
			utils.SafeWrite(buf, outputFilePath, err)
		} else {
			recurseDir = cmd.Flag("dir").Value.String()
			err := utils.ProcessDir(recurseDir, ".sls", "rotate", outputFilePath, topLevelElement, pk)
			if err != nil {
				logger.Warnf("rotate: %s", err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(rotateCmd)
	rotateCmd.PersistentFlags().StringP("dir", "d", "", "recurse over all .sls files in the given directory")
	rotateCmd.PersistentFlags().StringP("file", "f", "", "input file (defaults to STDIN)")
}