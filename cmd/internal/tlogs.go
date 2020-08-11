package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gonvenience/bunt"
	"github.com/hpcloud/tail"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/logrotate"
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
	"code.cloudfoundry.org/quarks-utils/pkg/logger"
)

// dataGatherCmd represents the dataGather command
var tailLogsCmd = &cobra.Command{
	Use:   "tail-logs [flags]",
	Short: "Tail logs from a pod",
	Long: `Tail logs from a container in the same pod.

This will tail all logs under the specified dir.
The dir can be set using the "-z" flag, or setting
the LOGS_DIR env variable.
It will also run logrotate.

`,
	RunE: func(_ *cobra.Command, args []string) (err error) {
		log = logger.New(cmd.LogLevel())
		defer log.Sync()

		d := time.Duration(viper.GetInt("logrotate-interval")) * time.Minute
		go func() {
			for range time.Tick(d) {
				log.Debug("running logrotate")
				out, err := logrotate.Logrotate()
				if err != nil {
					log.Errorf("failed to run logrotate: %v", err)
					log.Debugf("logrotate: %s", out)
				}
			}
		}()
		return TailLogsFromDir(log)
	},
}

func init() {
	utilCmd.AddCommand(tailLogsCmd)

	tailLogsCmd.Flags().StringP("logs-dir", "z", "", "a path from where to tail logs")
	viper.BindPFlag("logs-dir", tailLogsCmd.Flags().Lookup("logs-dir"))

	tailLogsCmd.Flags().IntP("logrotate-interval", "i", 24*60, "interval between logrotates in minutes")
	viper.BindPFlag("logrotate-interval", tailLogsCmd.Flags().Lookup("logrotate-interval"))

	argToEnv := map[string]string{
		"logs-dir":           "LOGS_DIR",
		"logrotate-interval": "LOGROTATE_INTERVAL",
	}
	cmd.AddEnvToUsage(tailLogsCmd, argToEnv)

	// Force colors to be seen in the pod STDOUT
	// even though it is not a terminal
	bunt.ColorSetting = bunt.ON
}

// TailLogsFromDir will stream to the pod
// STDOUT all file logs per line in the
// following syntax:
// timestampt, file name, message
//
// This only tail logs from files names
// in the form *.log
func TailLogsFromDir(log *zap.SugaredLogger) error {
	monitorDir := viper.GetString("logs-dir")
	if len(monitorDir) == 0 {
		return fmt.Errorf("logs directory cannot be empty")
	}

	if _, err := os.Stat(monitorDir); os.IsNotExist(err) {
		return err
	}

	// Get any existing subDir, so that it
	// can be added to the watcher.
	// If no subdirs exists, it will add
	// the current parent dir, for future watch
	listDir, err := getSubDirs(monitorDir)
	if err != nil {
		return err
	}

	// will host all files to be tail
	fileList := make(chan string)
	done := make(chan bool)

	// regex for files that should be tailed
	fileNameRegex := regexp.MustCompile(`(.*log)$`)

	// add all already existing files
	// to the list of files to be tailed
	go func() {
		if err := addExistingFiles(monitorDir, fileNameRegex, fileList); err != nil {
			log.Error(err)
		}
	}()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Start a Go routine to process watcher
	// events, which can include:
	// - new directory added -> needs to be watched
	// - new file added -> needs to be tailed
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return // events channel was closed
				}

				if event.Op&fsnotify.Create == fsnotify.Create {
					info, err := os.Stat(event.Name)
					switch {
					case err == nil && info.IsDir():
						watcher.Add(event.Name)

					case err == nil && !info.IsDir():
						if isValidFile(event.Name, fileNameRegex) {
							fileList <- event.Name
						}
					}
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return // events channel was closed
				}

				log.Error(err)
			}
		}
	}()

	// Add watcher for each of the initially
	// identified subDirs
	for _, dir := range listDir {
		if err = watcher.Add(dir); err != nil {
			log.Error(err)
		}
	}

	// Start the log tailing to process each file,
	// either existing or new ones.
	if err := LogTailors(fileList); err != nil {
		return err
	}

	// Block indefinitely
	<-done
	return nil
}

func addExistingFiles(monitDir string, fileRegex *regexp.Regexp, fileList chan string) error {
	err := filepath.Walk(monitDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && isValidFile(path, fileRegex) {
			fileList <- path
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func getSubDirs(path string) ([]string, error) {
	var listDirs []string
	err := filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				listDirs = append(listDirs, path)
			}
			return nil
		})
	if err != nil {
		return nil, err
	}

	return listDirs, nil
}

// isValidFile returns file names that follow the "*.log"
// naming convention
func isValidFile(fileName string, fileRegex *regexp.Regexp) bool {
	match := fileRegex.FindStringSubmatch(fileName)
	return len(match) > 0
}

// LogTailors stream logs per file from a channel
// into STDOUT of the pod where it runs.
func LogTailors(files chan string) error {
	output := make(chan StdOutMsg)
	errors := make(chan error)
	done := make(chan bool)

	// Routine for streaming lines into
	// pod STDOUT
	go func() {
		PrintOutput(output)
	}()

	// Routine for logging errors
	go func() {
		for err := range errors {
			log.Error(err)
		}
	}()

	var i = 0
	for file := range files {
		go func(fileName string, id int) {
			t, err := tail.TailFile(fileName, tail.Config{Follow: true})
			if err != nil {
				errors <- err
			}

			for line := range t.Lines {
				outputMsg := StdOutMsg{
					Message:   line.Text,
					Origin:    fileName,
					Timestamp: line.Time,
					ID:        id,
				}
				output <- outputMsg
			}
		}(file, i)

		i++
	}

	<-done
	return nil
}

// PrintOutput ensures that all logs send to STDOUT stream
// will be displayed on an specific way:
// - an specific color per file logs
// - added italics font, for the file name
func PrintOutput(messages chan StdOutMsg) {
	colors := bunt.RandomTerminalFriendlyColors(64)

	for msg := range messages {
		idx := msg.ID % len(colors)
		color := colors[idx]

		line := bunt.Sprintf("%04d/%02d/%02d %02d:%02d:%02d, _%s_, %s",
			msg.Timestamp.Year(),
			msg.Timestamp.Month(),
			msg.Timestamp.Day(),
			msg.Timestamp.Hour(),
			msg.Timestamp.Minute(),
			msg.Timestamp.Second(),
			msg.Origin,
			msg.Message,
		)

		fmt.Println(bunt.Style(line, bunt.Foreground(color)))
	}
}

// StdOutMsg serves as an struct
// where a msg corresponding a file can be store.
// It also stores a file ID, which will help on assigning
// an specific color to that file logs, when sending to STDOUT
type StdOutMsg struct {
	Timestamp time.Time
	Origin    string
	Message   string
	ID        int
}
