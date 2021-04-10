package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

type volumeStep struct {
	running     safeBool
	destination BackupDestination
	path        string
}

func NewVolumeStep(path string) *volumeStep {
	s := &volumeStep{}
	s.path = path

	return s
}

func (s *volumeStep) IsRunning() bool {
	return s.running.Get()
}

func (s *volumeStep) Type() string {
	return "volume"
}

func (s *volumeStep) Description() string {
	return s.path
}

func (s *volumeStep) SetDestination(destination BackupDestination) {
	s.destination = destination
}

func (s *volumeStep) Run(m *MetricsCollection) (err error) {
	if !s.running.SetIf(true, false) {
		return errors.New("Backup step already running")
	}
	defer s.running.Set(false)

	args := []string{"backup", "--json", "--host", s.destination.hostname}
	// Mitigate https://github.com/restic/restic/issues/2345
	// TODO: Remove when restic issue #2345 is fixed
	args = append(args, "--cache-dir="+os.Getenv("HOME")+"/.cache/restic"+strings.ReplaceAll(s.path, "/", "-"))
	if _, err := os.Stat(s.path + "/.resticexclude"); err == nil {
		args = append(args, "--exclude-file="+s.path+"/.resticexclude")
	}
	args = append(args, s.path)
	cmd := exec.Command("restic", args...)

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	if err != nil {
		exiterr, ok := err.(*exec.ExitError)
		if ok {
			logger.Info("command restic backup failed",
				zap.String("stdout", stdout.String()), zap.String("stderr", stderr.String()), zap.Error(err),
				zap.Int("code", exiterr.ExitCode()),
			)
		} else {
			logger.Error("command restic backup failed",
				zap.String("stdout", stdout.String()), zap.String("stderr", stderr.String()), zap.Error(err),
			)
			return err
		}
	}

	// ok
	logger.Debug("backup step done", zap.String("stdout", stdout.String()), zap.String("stderr", stderr.String()))

	// TODO: parse output, fill metrics

	/*
	   restic backup --json --host restic-agent /app

	   {"message_type":"status","percent_done":0,
	   "total_files":1,"total_bytes":20480}

	   {"message_type":"status","percent_done":0.09552008945401648,
	   "total_files":11,"files_done":10,"total_bytes":12718937,"bytes_done":1214914,"current_files":["/app/restic-agent"]}

	   {"message_type":"status","percent_done":1,
	   "total_files":11,"files_done":11,"total_bytes":12718937,"bytes_done":12718937,"current_files":["/app/restic-agent"]}

	   {"message_type":"summary",
	   "files_new":11,"files_changed":0,"files_unmodified":0,"dirs_new":0,"dirs_changed":0,"dirs_unmodified":0,
	   "data_blobs":20,"tree_blobs":1,"data_added":12719265,"total_files_processed":11,"total_bytes_processed":12718937,
	   "total_duration":0.422311019,
	   "snapshot_id":"cc344156"}
	*/

	return nil
}
