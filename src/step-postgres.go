package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strconv"

	"go.uber.org/zap"
)

type postgresStep struct {
	running     safeBool
	destination BackupDestination
	host        string
	port        int
	user        string
	database    string
	name        string
}

func NewPostgresStep(host string, user string, password string, database string) (s *postgresStep, err error) {
	s = &postgresStep{}
	// TODO: match host for 's/(:\d+)$//' and $1 => port
	s.host = host
	s.port = 5432
	s.user = user
	s.database = database

	// no sub-directory, see https://github.com/restic/restic/pull/2206 (fixed in master)
	s.name = "/psql-" + s.host + "-" + s.database + ".dmp"

	// WRONLY = WRite-only
	f, err := os.OpenFile(os.Getenv("HOME")+"/.pgpass", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// TODO: Check for duplicate rows, overwrite old passwords instead of appending
	// TODO: Check for ":" in parameters and escape them
	text := s.host + ":" + strconv.Itoa(s.port)
	text = text + ":" + s.database
	text = text + ":" + s.user
	text = text + ":" + password
	if _, err = f.WriteString(text + "\n"); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *postgresStep) IsRunning() bool {
	return s.running.Get()
}

func (s *postgresStep) Type() string {
	return "postgres"
}

func (s *postgresStep) Description() string {
	return s.user + "@" + s.host + "/" + s.database
}

func (s *postgresStep) SetDestination(destination BackupDestination) {
	s.destination = destination
}

func (s *postgresStep) SetName(name string) {
	s.name = name
}

func (s *postgresStep) Run(m *MetricsCollection) (err error) {
	if !s.running.SetIf(true, false) {
		return errors.New("Backup step already running")
	}
	defer s.running.Set(false)

	args := []string{"-h", s.host, "-U", s.user, "-w"}
	args = append(args, "-d", s.database)
	cmdPg := exec.Command("pg_dump", args...)

	args = []string{"backup", "--json", "--host", s.destination.hostname}
	args = append(args, "--stdin", "--stdin-filename", s.name)
	cmd := exec.Command("restic", args...)

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Better this way or vice versa - any difference?
	// cmdPg.Stdout, err = cmd.StdinPipe()
	cmd.Stdin, err = cmdPg.StdoutPipe()
	if err != nil {
		return err
	}

	// Start processes
	if err = cmdPg.Start(); err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}

	// Wait for dump to complete
	errPg := cmdPg.Wait()
	// Wait for snapshot writing to complete
	err = cmd.Wait()

	if errPg != nil {
		exiterr, ok := errPg.(*exec.ExitError)
		if ok {
			logger.Info("command pg_dump failed", zap.Error(err),
				zap.ByteString("stderr", exiterr.Stderr), zap.Int("code", exiterr.ExitCode()),
			)
		} else {
			logger.Error("command pg_dump failed", zap.Error(err))
		}
		return errPg
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
