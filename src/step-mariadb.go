package main

import (
	"bytes"
	"errors"
	"os/exec"

	"go.uber.org/zap"
)

type mariadbStep struct {
	running     safeBool
	destination BackupDestination
	host        string
	port        int
	user        string
	password    string
	database    string
	name        string
}

func NewMariadbStep(host string, user string, password string, database string) (s *mariadbStep, err error) {
	s = &mariadbStep{}
	// TODO: match host for 's/(:\d+)$//' and $1 => port
	s.host = host
	s.port = 3306
	s.user = user
	s.password = password
	s.database = database

	// no sub-directory, see https://github.com/restic/restic/pull/2206 (fixed in master)
	s.name = "/mysql-" + s.host + "-" + s.database + ".dmp"

	return s, nil
}

func (s *mariadbStep) IsRunning() bool {
	return s.running.Get()
}

func (s *mariadbStep) Type() string {
	return "mariadb"
}

func (s *mariadbStep) Description() string {
	return s.user + "@" + s.host + "/" + s.database
}

func (s *mariadbStep) SetDestination(destination BackupDestination) {
	s.destination = destination
}

func (s *mariadbStep) SetName(name string) {
	s.name = name
}

func (s *mariadbStep) Run(m *MetricsCollection) (err error) {
	if !s.running.SetIf(true, false) {
		return errors.New("Backup step already running")
	}
	defer s.running.Set(false)

	args := []string{"-h", s.host, "-u", s.user, "-p", s.password}
	args = append(args, s.database)
	cmdDb := exec.Command("mariadb-dump", args...)

	args = []string{"backup", "--json", "--host", s.destination.hostname}
	args = append(args, "--stdin", "--stdin-filename", s.name)
	cmd := exec.Command("restic", args...)

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Better this way or vice versa - any difference?
	// cmdDb.Stdout, err = cmd.StdinPipe()
	cmd.Stdin, err = cmdDb.StdoutPipe()
	if err != nil {
		return err
	}

	// Start processes
	if err = cmdDb.Start(); err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}

	// Wait for dump to complete
	errDb := cmdDb.Wait()
	// Wait for snapshot writing to complete
	err = cmd.Wait()

	if errDb != nil {
		exiterr, ok := errDb.(*exec.ExitError)
		if ok {
			logger.Info("command mariadb-dump failed", zap.Error(err),
				zap.ByteString("stderr", exiterr.Stderr), zap.Int("code", exiterr.ExitCode()),
			)
		} else {
			logger.Error("command mariadb-dump failed", zap.Error(err))
		}
		return errDb
	}

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
		}
		return err
	}

	// ok
	logger.Info("ok", zap.String("stdout", stdout.String()), zap.String("stderr", stderr.String()))
	// TODO: parse output, fill metrics, see volumeStep

	return nil
}
