/*
Copyright © 2023 Jean-Marc Meessen jean-marc@meessen-web.org

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"

	"regexp"
	"strings"

	//See https://github.com/schollz/progressbar
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

// commentersCmd represents the commenters command
var commentersCmd = &cobra.Command{
	Use:   "commenters [PR list CSV filename]",
	Short: "Get the commenters  for a single PR or a set of PRs listed in a CSV file",
	Long: `Retrieve the Pull Request commenters. 
It is possible to either pass a (CSV) list of PRs or to specify a single PR. 

The CSV list of PRs must be in the form of \"org,repository,number,url,state,created_at,merged_at,user.login,month_year,title\"
Such a CSV is generated by the jenkins submitter extractions tool (\"jenkins-stats get submitters\").

To extract the commenters for a single PR, use the "forPR" sub-command. 
`,
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.MinimumNArgs(1)(cmd, args); err != nil {
			return err
		}
		if !fileExist(args[0]) {
			return fmt.Errorf("Invalid file\n")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Debug flag is hidden
		initLoggers()
		if isRootDebug {
			loggers.debug.Println("******** New \"Get Commenters\" debug session ********")
		}

		if isRootDebug {
			fmt.Print("*** Debug mode enabled ***\nSee \"debug.log\" for the trace\n\n")

			limit, remaining, _, _ := get_quota_data_v4()
			loggers.debug.Printf("Start quota: %d/%d\n", remaining, limit)
		}

		performAction(args[0])

		if isRootDebug {
			limit, remaining, _, _ := get_quota_data_v4()
			loggers.debug.Printf("End quota: %d/%d\n", remaining, limit)
		}
	},
}

func init() {
	getCmd.AddCommand(commentersCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// commentersCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// commentersCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

var referenceCSVheader = []string{"org", "repository", "number", "url", "state", "created_at", "merged_at", "user.login", "month_year", "title"}

// Loads the data from a file and try to parse it as a CSV
func loadPrListFile(fileName string, isVerbose bool) ([]string, bool) {

	f, err := os.Open(fileName)
	if err != nil {
		log.Printf("Unable to read input file "+fileName+"\n", err)
		return nil, false
	}
	defer f.Close()

	r := csv.NewReader(f)

	headerLine, err1 := r.Read()
	if err1 != nil {
		log.Printf("Unexpected error loading"+fileName+"\n", err)
		return nil, false
	}

	if isVerbose {
		fmt.Println("Checking input file")
	}

	if !validateHeader(headerLine, referenceCSVheader, isVerbose) {
		fmt.Println(" Error: header is incorrect.")
		return nil, false
	} else {
		if isVerbose {
			fmt.Printf("  - Header is correct\n")
		}
	}

	records, err := r.ReadAll()
	if err != nil {
		log.Printf("Unexpected error loading \""+fileName+"\"\n", err)
		return nil, false
	}

	if len(records) < 2 {
		fmt.Printf("Error: No data available after the header\n")
		return nil, false
	}
	if isVerbose {
		fmt.Println("  - At least one Pull Request data available")
	}

	var prList []string
	prj_regexp, _ := regexp.Compile(`^[\w-\.]+$`) // see https://stackoverflow.com/questions/59081778/rules-for-special-characters-in-github-repository-name
	pr_regexp, _ := regexp.Compile(`^\d+$`)

	// Check the loaded data
	for _, dataLine := range records {

		org := dataLine[0]
		if !isValidOrgFormat(org) {
			if isVerbose {
				fmt.Printf(" Error: ORG field \"%s\" doesn't seem to be a valid GitHub org.\n", org)
			}
			if isRootDebug{
				loggers.debug.Printf(" Error: ORG field \"%s\" doesn't seem to be a valid GitHub org.\n", org)
			}
			return nil, false
		}

		// project name must be "^[\w-\.]+$"
		prj := dataLine[1]
		if !prj_regexp.MatchString(strings.ToLower(prj)) {
			if isVerbose {
				fmt.Printf(" Error: PRJ field \"%s\" is not of the expected format", prj)
			}
			if isRootDebug{
				loggers.debug.Printf(" Error: PRJ field \"%s\" is not of the expected format", prj)
			}
			return nil, false
		}

		// PR number must be a number
		prNbr := dataLine[2]
		if !pr_regexp.MatchString(prNbr) {
			if isVerbose {
				fmt.Printf(" Error: PR field \"%s\" is not a (positive) number", prNbr)
			}
			if isRootDebug{
				loggers.debug.Printf(" Error: PR field \"%s\" is not a (positive) number", prNbr)
			}
			return nil, false
		}

		prInfo := fmt.Sprintf("%s/%s/%s", org, prj, prNbr)
		prList = append(prList, prInfo)

	}

	if isVerbose {
		fmt.Printf("Successfully loaded \"%s\" (%d Pull Request to analyze)\n\n", fileName, len(prList))
	}

	return prList, true
}

// Checks whether the retrieved header is equivalent to the reference header
func validateHeader(header []string, referenceHeader []string, isVerbose bool) bool {
	if len(header) != len(referenceHeader) {
		if isVerbose {
			fmt.Printf(" Error: field number mismatch (found %d, wanted %d)\n", len(header), len(referenceHeader))
		}
		return false
	}
	for i, v := range header {
		if v != referenceHeader[i] {
			if isVerbose {
				fmt.Printf(" Error: not the expected header field at column %d (found \"%v\", wanted \"%v\")\n", i+1, v, referenceHeader[i])
			}
			return false
		}
	}
	return true
}

// **************
// **************

// This is where it happens
func performAction(inputFile string) {

	fmt.Printf("Processing \"%s\"\n", inputFile)
	if isRootDebug {
		loggers.debug.Printf("Processing \"%s\"\n", inputFile)
	}

	// read the relevant data from the file (and checking it)
	prList, result := loadPrListFile(inputFile, isVerbose)
	if !result {
		fmt.Printf("Could not load \"%s\"\n", inputFile)
		os.Exit(1)
	}

	isAppend := globalIsAppend
	if !globalIsAppend {
		// Meaning that we need to create a new file
		if fileExist(outputFileName) {
			os.Remove(outputFileName)
		}
		isAppend = true
	}

	//check if we have enough quota left to process the whole file
	checkIfSufficientQuota(len(prList))

	var bar *progressbar.ProgressBar
	if !isVerbose {
		bar = progressbar.Default(int64(len(prList)))
	}

	nbrPR_noComment := 0
	nbrPR_withComments := 0
	totalComments := 0
	for _, pr_line := range prList {
		//Process the line
		nbrOfComments := getCommenters(pr_line, isAppend, globalIsNoHeader, outputFileName)

		totalComments = totalComments + nbrOfComments
		//do some accounting
		if nbrOfComments == 0 {
			nbrPR_noComment++
		} else {
			nbrPR_withComments++
		}

		// update the progress bar if in quiet mode
		if !isVerbose {
			err := bar.Add(1)
			if err != nil {
				log.Printf("Unexpected error updating progress bar (%v)\n", err)
			}
		}
	}
	fmt.Printf("Nbr of PR without comments: %d\n", nbrPR_noComment)
	fmt.Printf("Nbr of PR with comments:    %d\n", nbrPR_withComments)
	fmt.Printf("Total comments:             %d\n", totalComments)

	if isRootDebug {
		loggers.debug.Printf("Nbr of PR without comments: %d\n", nbrPR_noComment)
		loggers.debug.Printf("Nbr of PR with comments:    %d\n", nbrPR_withComments)
		loggers.debug.Printf("Total comments:             %d\n", totalComments)
	}

}
