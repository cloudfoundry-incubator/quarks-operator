package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gonvenience/bunt"
	colorful "github.com/lucasb-eyer/go-colorful"

	"github.com/hpcloud/tail"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
)

var clientThing *kubernetes.Clientset
var nene []string

// dataGatherCmd represents the dataGather command
var tailLogsCmd = &cobra.Command{
	Use:   "tail-logs [flags]",
	Short: "Tail logs from a pod",
	Long: `Tail logs from a container in the same pod.

This will tail all logs under the specified dir.
The dir can be set using the "-z" flag, or setting
the LOGS_DIR env variable.

`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		return TailLogsFromDir()
	},
}

// TailLogsFromDir will stream to the pod
// stdOut all file logs per line in the
// following syntax:
// timestampt, file name, message
//
// This only tail logs from files names
// in the form *.log
func TailLogsFromDir() error {
	log = newLogger()
	defer log.Sync()

	monitorDir := viper.GetString("logs-dir")
	if len(monitorDir) == 0 {
		return fmt.Errorf("logs directory cannot be empty")
	}

	if _, err := os.Stat(monitorDir); os.IsNotExist(err) {
		return err
	}

	fileList, err := WaitForFilesToAppear(monitorDir)
	if err != nil {
		return err
	}

	err = LogTailors(fileList)

	if err != nil {
		return err
	}

	return nil
}

func init() {
	utilCmd.AddCommand(tailLogsCmd)
}

// WaitForFilesToAppear use a ticker to wait until files appear
// under the specified dir, it will timeout after some time.
func WaitForFilesToAppear(monitorDir string) ([]string, error) {
	timeOut := time.After(120 * time.Second)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-timeOut:
			return nil, fmt.Errorf("timeout waiting for logs to appear under %s", monitorDir)
		case <-tick.C:
			list, err := GetTreeFiles(monitorDir)
			if err != nil {
				return nil, err
			}
			if len(list) > 0 {
				return list, nil
			}
		}
	}
}

// GetTreeFiles will return a list of *.log files
// under an specific path.
func GetTreeFiles(path string) ([]string, error) {
	var files []string
	var fileNameRegex = regexp.MustCompile(`(.*log)$`)

	err := filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && IsValidFile(info.Name(), fileNameRegex) {
				files = append(files, path)
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// IsValidFile returns file names that follow the "*.log"
// naming convention
func IsValidFile(fileName string, fileRegex *regexp.Regexp) bool {
	match := fileRegex.FindStringSubmatch(fileName)
	if len(match) > 0 {
		return true
	}
	return false
}

// LogTailors stream logs per file from a channel
// into stdOut of the pod where it runs.
func LogTailors(files []string) error {
	wg := &sync.WaitGroup{}
	output := make(chan StdOutMsg)
	errors := make(chan error, len(files))

	wg.Add(len(files))
	for i, file := range files {
		go func(fileName string, id int) {
			defer wg.Done()
			t, err := tail.TailFile(fileName, tail.Config{Follow: true})
			errors <- err

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
	}

	go func() {
		PrintOutput(output, len(files))
	}()

	wg.Wait()
	close(errors)
	close(output)

	if err := errorsFromChannel("tailing files failed.", errors); err != nil {
		return err
	}

	return nil
}

// Receive all errors from the error channel,
// and return them as a single multiline error.
func errorsFromChannel(errorCtx string, e chan error) error {
	errors := []string{}
	for err := range e {
		if err != nil {
			errors = append(errors, err.Error())
		}
	}

	switch len(errors) {
	case 0:
		return nil
	default:
		return fmt.Errorf("%v", strings.Join(errors, "\n"))
	}

}

// Force colors to be seen in the pod
// stdOutput
func init() {
	bunt.ColorSetting = bunt.ON
}

// PrintOutput ensures that all logs send to stdOut stream
// will be displayed on an specific way:
// - an specific color per file logs
// - added italics font, for the file name
func PrintOutput(messages chan StdOutMsg, items int) {
	var colors []colorful.Color
	colors = bunt.RandomTerminalFriendlyColors(items)

	for msg := range messages {
		humanReadableTime := fmt.Sprintf("%04d/%02d/%02d %02d:%02d:%02d", msg.Timestamp.Year(), msg.Timestamp.Month(), msg.Timestamp.Day(), msg.Timestamp.Hour(), msg.Timestamp.Minute(), msg.Timestamp.Second())

		fmt.Printf("%s, %s, %s\n",
			bunt.Style(humanReadableTime, bunt.Foreground(colors[msg.ID])),
			bunt.Style(msg.Origin, bunt.Foreground(colors[msg.ID]), bunt.Italic()),
			bunt.Style(msg.Message, bunt.Foreground(colors[msg.ID])),
		)
	}
}

// StdOutMsg serves as an struct
// where a msg corresponding a file can be store.
// It also stores a file ID, which will help on assigning
// an specific color to that file logs, when sending to stdOut
type StdOutMsg struct {
	Timestamp time.Time
	Origin    string
	Message   string
	ID        int
}
