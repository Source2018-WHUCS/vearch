// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"github.com/vearch/vearch/tools/cbdbimport/logic"
	"os"

	"github.com/spf13/cobra"
)

var master string
var router string
var datafile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cbdbexport",
	Short: "A brief description of your application",
	Long:  `Export data from cbdb in JSON format.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		normal := logic.NewNormal(master, router, datafile)
		err := normal.Load()
		if err != nil {
			panic(err)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&master, "master", "m", "", "cbdb master to connect")
	rootCmd.Flags().StringVarP(&router, "router", "r", "", "cbdb router to connect")
	rootCmd.Flags().StringVarP(&datafile, "datafile", "o", "", "cbdb datafile")
}
