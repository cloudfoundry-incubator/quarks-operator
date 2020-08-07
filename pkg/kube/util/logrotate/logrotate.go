package logrotate

import (
	"io/ioutil"
	"os"
	"os/exec"
)

// Interval is the configured time in minutes between logrotate shell outs
var interval int

const rotate = `
/var/vcap/sys/log/*.log /var/vcap/sys/log/*/*.log /var/vcap/sys/log/*/*/*.log {
  missingok
  rotate 7
  compress
  delaycompress
  copytruncate
  maxsize=10M
  minsize=9M
}
/var/log/syslog
{
	rotate 7
	maxsize=10M
	minsize=9M
	missingok
	notifempty
	delaycompress
	compress
	copytruncate
}

/var/log/mail.info
/var/log/mail.warn
/var/log/mail.err
/var/log/mail.log
/var/log/daemon.log
/var/log/kern.log
/var/log/auth.log
/var/log/user.log
/var/log/lpr.log
/var/log/cron.log
/var/log/debug
/var/log/messages
{
	rotate 4
	maxsize=10M
	minsize=9M
	missingok
	notifempty
	compress
	delaycompress
	sharedscripts
	copytruncate
}
`

// SetInterval stores the interval as an int in the package scope
func SetInterval(m int) {
	interval = m
}

// GetInterval returns the configured interval as an int
func GetInterval() int {
	return interval
}

// Logrotate runs the logrotate binary
func Logrotate() ([]byte, error) {
	tmpfile, err := ioutil.TempFile("", "logrotate*.conf")
	if err != nil {
		return []byte{}, err
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(rotate)); err != nil {
		return []byte{}, err
	}

	if err := tmpfile.Close(); err != nil {
		return []byte{}, err
	}

	cmd := exec.Command("/usr/sbin/logrotate", tmpfile.Name())
	return cmd.CombinedOutput()
}
