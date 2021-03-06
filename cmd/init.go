// Copyright (c) 2016, WSO2 Inc. (http://www.wso2.org) All Rights Reserved.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/renstrom/dedent"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/wso2/update-creator-tool/constant"
	"github.com/wso2/update-creator-tool/util"
	"gopkg.in/yaml.v2"
)

var (
	initCmdUse       = "init"
	initCmdShortDesc = "Generate '" + constant.UPDATE_DESCRIPTOR_FILE + "' file template"
	initCmdLongDesc  = dedent.Dedent(`
		This command will generate the 'update-descriptor.yaml' file. If
		the user does not specify a directory, it will use the current
		working directory. It will fill the data using any available
		README.txt file in the old patch format. If README.txt is not
		found, it will fill values using default values which you need
		to edit manually.`)

	initCmdExample = dedent.Dedent(`update_number: 0001
platform_version: 4.4.0
platform_name: wilkes
applies_to: All the products based on carbon 4.4.1
bug_fixes:
  CARBON-15395: Upgrade Hazelcast version to 3.5.2
  <MORE_JIRAS_HERE>
description: |
  This update contain the relavent fixes for upgrading Hazelcast version
  to its latest 3.5.2 version. When applying this update it requires a
  full cluster estart since if the nodes has multiple client versions of
  Hazelcast it can cause issues during connectivity.
file_changes:
  added_files: []
  removed_files: []
  modified_files: []`)
)

// initCmd represents the validate command
var initCmd = &cobra.Command{
	Use:   initCmdUse,
	Short: initCmdShortDesc,
	Long:  initCmdLongDesc,
	Run:   initializeInitCommand,
}

//This function will be called first and this will add flags to the command.
func init() {
	RootCmd.AddCommand(initCmd)

	initCmd.Flags().BoolVarP(&isDebugLogsEnabled, "debug", "d", util.EnableDebugLogs, "Enable debug logs")
	initCmd.Flags().BoolVarP(&isTraceLogsEnabled, "trace", "t", util.EnableTraceLogs, "Enable trace logs")

	initCmd.Flags().BoolP("sample", "s", false, "Show sample file")
	viper.BindPFlag(constant.SAMPLE, initCmd.Flags().Lookup("sample"))
}

//This function will be called when the create command is called.
func initializeInitCommand(cmd *cobra.Command, args []string) {
	logger.Debug("[Init] called")
	switch len(args) {
	case 0:
		if viper.GetBool(constant.SAMPLE) {
			logger.Debug("-s flag found. Printing sample...")
			fmt.Println(initCmdExample)
		} else {
			logger.Debug("-s flag not found. Initializing current working directory.")
			initCurrentDirectory()
		}
	case 1:
		logger.Debug("Initializing directory:", args[0])
		initDirectory(args[0])
	default:
		logger.Debug("Invalid number of argumants:", args)
		util.HandleErrorAndExit(errors.New("Invalid number of argumants. Run 'wum-uc init --help' to view " +
			"help."))
	}
}

//This function will be called if no arguments are provided by the user.
func initCurrentDirectory() {
	currentDirectory := "./"
	initDirectory(currentDirectory)
}

//This function will start the init process.
func initDirectory(destination string) {
	logger.Debug("Initializing started.")
	// Check whether the provided directory exists
	exists, err := util.IsDirectoryExists(destination)
	logger.Debug(fmt.Sprintf("'%s' directory exists: %v", destination, exists))

	// If the directory does not exists, prompt the user
	skip := false
	if !exists {
	userInputLoop:
		for {
			util.PrintInBold(fmt.Sprintf("'%s'does not exists. Do you want to create '%s' directory?"+
				"[Y/n]: ", destination, destination))
			preference, err := util.GetUserInput()
			if len(preference) == 0 {
				preference = "y"
			}
			util.HandleErrorAndExit(err, "Error occurred while getting input from the user.")

			// Get the user preference
			userPreference := util.ProcessUserPreference(preference)
			switch userPreference {
			case constant.YES:
				util.PrintInfo(fmt.Sprintf("'%s' directory does not exist. Creating '%s' directory.",
					destination, destination))
				err := util.CreateDirectory(destination)
				util.HandleErrorAndExit(err)
				logger.Debug(fmt.Sprintf("'%s' directory created.", destination))
				break userInputLoop
			case constant.NO:
				skip = true
				break userInputLoop
			default:
				util.PrintError("Invalid preference. Enter Y for Yes or N for No.")
			}
		}
	}
	// If the skip is selected, exit
	if skip {
		util.HandleErrorAndExit(errors.New("Directory creation skipped. Please enter a valid directory."))
	}

	// Create a new update descriptor struct
	updateDescriptor := util.UpdateDescriptor{}

	// Process README.txt and parse values
	processReadMe(destination, &updateDescriptor)

	// Marshall the update descriptor struct
	data, err := yaml.Marshal(&updateDescriptor)
	util.HandleErrorAndExit(err)

	dataString := string(data)
	//remove " enclosing the update number
	dataString = strings.Replace(dataString, "\"", "", -1)
	logger.Debug(fmt.Sprintf("update-descriptor:\n%s", dataString))

	// Construct the update descriptor file path
	updateDescriptorFile := filepath.Join(destination, constant.UPDATE_DESCRIPTOR_FILE)
	logger.Debug(fmt.Sprintf("updateDescriptorFile: %v", updateDescriptorFile))

	// Save the update descriptor
	file, err := os.OpenFile(
		updateDescriptorFile,
		os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
		0600,
	)
	util.HandleErrorAndExit(err)
	defer file.Close()

	// Write bytes to file
	_, err = file.Write([]byte(dataString))
	if err != nil {
		util.HandleErrorAndExit(err)
	}

	// Get the absolute location
	absDestination, err := filepath.Abs(destination)
	if err != nil {
		absDestination = destination
	}
	util.PrintInfo(fmt.Sprintf("'%s' has been successfully created at '%s'.", constant.UPDATE_DESCRIPTOR_FILE,
		absDestination))

	//Print whats next
	color.Set(color.Bold)
	fmt.Println("\nWhat's next?")
	color.Unset()
	fmt.Println(fmt.Sprintf("\trun 'wum-uc init --sample' to view a sample '%s' file.",
		constant.UPDATE_DESCRIPTOR_FILE))
}

//This function will set default valued to the update-descriptor.yaml.
func setUpdateDescriptorDefaultValues(updateDescriptor *util.UpdateDescriptor) {
	logger.Debug("Setting default values:")
	updateDescriptor.Update_number = constant.UPDATE_NO_DEFAULT
	updateDescriptor.Platform_name = constant.PLATFORM_NAME_DEFAULT
	updateDescriptor.Platform_version = constant.PLATFORM_VERSION_DEFAULT
	updateDescriptor.Applies_to = constant.APPLIES_TO_DEFAULT
	updateDescriptor.Description = constant.DESCRIPTION_DEFAULT
	bugFixes := map[string]string{
		constant.JIRA_KEY_DEFAULT: constant.JIRA_SUMMARY_DEFAULT,
	}
	updateDescriptor.Bug_fixes = bugFixes
	logger.Debug(fmt.Sprintf("bug_fixes: %v", bugFixes))
}

//This function will process the readme file and extract details to populate update-descriptor.yaml. If some data
// cannot be extracted, it will add default value and continue.
func processReadMe(directory string, updateDescriptor *util.UpdateDescriptor) {
	logger.Debug("Processing README started")
	// Construct the README.txt path
	readMePath := path.Join(directory, constant.README_FILE)
	logger.Debug(fmt.Sprintf("README Path: %v", readMePath))
	// Check whether the README.txt file exists
	_, err := os.Stat(readMePath)
	if err != nil {
		// If the file does not exist or any other error occur, return without printing warning messages
		logger.Debug(fmt.Sprintf("%s not found", readMePath))
		setUpdateDescriptorDefaultValues(updateDescriptor)
		return
	}
	// Read the README.txt file
	data, err := ioutil.ReadFile(readMePath)
	if err != nil {
		// If any error occurs, return without printing warning messages
		logger.Debug(fmt.Sprintf("Error occurred and processing README: %v", err))
		setUpdateDescriptorDefaultValues(updateDescriptor)
		return
	}

	logger.Debug("README.txt found")

	// Convert the byte array to a string
	stringData := string(data)
	// Compile the regex
	regex, err := regexp.Compile(constant.PATCH_ID_REGEX)
	if err == nil {
		result := regex.FindStringSubmatch(stringData)
		logger.Trace(fmt.Sprintf("PATCH_ID_REGEX result: %v", result))
		// Since the regex has 2 capturing groups, the result size will be 3 (because there is the full match)
		// If not match found, the size will be 0. We check whether the result size is not 0 to make sure both
		// capturing groups are identified.
		if len(result) != 0 {
			// Extract details
			updateDescriptor.Update_number = result[2]
			updateDescriptor.Platform_version = result[1]
			platformsMap := viper.GetStringMapString(constant.PLATFORM_VERSIONS)
			logger.Trace(fmt.Sprintf("Platform Map: %v", platformsMap))
			// Get the platform details from the map
			platformName, found := platformsMap[result[1]]
			if found {
				logger.Debug("PlatformName found in configs")
				updateDescriptor.Platform_name = platformName
			} else {
				//If the platform name is not found, set default
				logger.Debug("No matching platform name found for:", result[1])
				updateDescriptor.Platform_name = constant.PLATFORM_NAME_DEFAULT
			}
		} else {
			logger.Debug("PATCH_ID_REGEX results incorrect:", result)
		}
	} else {
		//If error occurred, set default values
		logger.Debug(fmt.Sprintf("Error occurred while processing PATCH_ID_REGEX: %v", err))
		updateDescriptor.Update_number = constant.UPDATE_NO_DEFAULT
		updateDescriptor.Platform_name = constant.PLATFORM_NAME_DEFAULT
		updateDescriptor.Platform_version = constant.PLATFORM_VERSION_DEFAULT
	}

	// Compile the regex
	regex, err = regexp.Compile(constant.APPLIES_TO_REGEX)
	if err == nil {
		result := regex.FindStringSubmatch(stringData)
		logger.Trace(fmt.Sprintf("APPLIES_TO_REGEX result: %v", result))
		// In the README, Associated Jiras section might not appear. If it does appear, result size will be 2.
		// If it does not appear, result size will be 3.
		if len(result) == 2 {
			// If the result size is 2, we know that 1st index contains the 1st capturing group.
			updateDescriptor.Applies_to = util.ProcessString(result[1], ", ", true)
		} else if len(result) == 3 {
			// If the result size is 3, 1st or 2nd string might contain the match. So we concat them
			// together and trim the spaces. If one field has an empty string, it will be trimmed.
			updateDescriptor.Applies_to = util.ProcessString(strings.TrimSpace(result[1]+result[2]), ", ",
				true)
		} else {
			logger.Debug("No matching results found for APPLIES_TO_REGEX:", result)
		}
	} else {
		//If error occurred, set default value
		logger.Debug(fmt.Sprintf("Error occurred while processing APPLIES_TO_REGEX: %v", err))
		updateDescriptor.Applies_to = constant.APPLIES_TO_DEFAULT
	}

	// Compile the regex
	regex, err = regexp.Compile(constant.ASSOCIATED_JIRAS_REGEX)
	if err == nil {
		// Get all matches because there might be multiple Jiras.
		allResult := regex.FindAllStringSubmatch(stringData, -1)
		logger.Trace(fmt.Sprintf("APPLIES_TO_REGEX result: %v", allResult))
		updateDescriptor.Bug_fixes = make(map[string]string)
		// If no Jiras found, set 'N/A: N/A' as the value
		if len(allResult) == 0 {
			logger.Debug("No matching results found for ASSOCIATED_JIRAS_REGEX. Setting default values.")
			updateDescriptor.Bug_fixes[constant.JIRA_NA] = constant.JIRA_NA
		} else {
			// If Jiras found, get summary for all Jiras
			logger.Debug("Matching results found for ASSOCIATED_JIRAS_REGEX")
			for i, match := range allResult {
				// Regex has a one capturing group. So the jira ID will be in the 1st index.
				logger.Debug(fmt.Sprintf("%d: %s", i, match[1]))
				logger.Debug(fmt.Sprintf("ASSOCIATED_JIRAS_REGEX results is correct: %v", match))
				updateDescriptor.Bug_fixes[match[1]] = util.GetJiraSummary(match[1])
			}
		}
	} else {
		//If error occurred, set default values
		logger.Debug(fmt.Sprintf("Error occurred while processing ASSOCIATED_JIRAS_REGEX: %v", err))
		logger.Debug("Setting defailt values to bug_fixes")
		updateDescriptor.Bug_fixes = make(map[string]string)
		updateDescriptor.Bug_fixes[constant.JIRA_KEY_DEFAULT] = constant.JIRA_SUMMARY_DEFAULT
	}

	// Compile the regex
	regex, err = regexp.Compile(constant.DESCRIPTION_REGEX)
	if err == nil {
		// Get the match
		result := regex.FindStringSubmatch(stringData)
		logger.Trace(fmt.Sprintf("DESCRIPTION_REGEX result: %v", result))
		// If there is a match, process it and store it
		if len(result) != 0 {
			updateDescriptor.Description = util.ProcessString(result[1], "\n", false)
		} else {
			logger.Debug(fmt.Sprintf("No matching results found for DESCRIPTION_REGEX: %v", result))
		}
	} else {
		//If error occurred, set default values
		logger.Debug(fmt.Sprintf("Error occurred while processing DESCRIPTION_REGEX: %v", err))
		updateDescriptor.Description = constant.DESCRIPTION_DEFAULT
	}
	logger.Debug("Processing README finished")
}
