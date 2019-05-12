package main

import (
	"errors"
	"os/exec"
	"sync"

	"go.uber.org/zap"
)

// Set of backup steps
type BackupSet struct {
	destination BackupDestination
	running     safeBool
	waitGroup   sync.WaitGroup
	steps       []BackupStep

	metrics *MetricsCollection
}

// Destination for restic
type BackupDestination struct {
	repository string // not used, see environment variable RESTIC_REPOSITORY
	password   string // not used, see environment variable RESTIC_PASSWORD
	hostname   string // used as --host argument
}

type BackupStep interface {
	IsRunning() bool
	Run(*MetricsCollection) error
	Type() string
	Description() string
	SetDestination(BackupDestination)
}

func (b *BackupSet) SetRepository(repository string, password string) {
	logger.Debug("set repository", zap.String("repository", repository), zap.Int("passwordLength", len(password)))
	b.destination.repository = repository
	b.destination.password = password
}

func (b *BackupSet) SetHostname(hostname string) {
	logger.Debug("set hostname", zap.String("hostname", hostname))
	b.destination.hostname = hostname
}
func (b *BackupSet) SetMetrics(m *MetricsCollection) {
	logger.Debug("assign metrics collection")
	b.metrics = m
}

func (b *BackupSet) AddStep(s BackupStep) {
	logger.Info("add backup step", zap.String("type", s.Type()), zap.String("description", s.Description()))
	s.SetDestination(b.destination)
	b.steps = append(b.steps, s)
}

func (b *BackupSet) IsRunning() bool {
	return b.running.Get()
}

// Start backup process and return not before finished
// This prototype important for cron scheduler
func (b *BackupSet) Run() {
	if !b.running.SetIf(true, false) {
		logger.Warn("backup already running")

		return
	}
	defer b.running.Set(false)

	b.run()
}

// Start backup process as goroutine and return immediately
func (b *BackupSet) Start() error {
	if !b.running.SetIf(true, false) {
		logger.Warn("backup already running")

		return errors.New("Backup already running")
	}
	go func() {
		defer b.running.Set(false)

		b.run()
	}()

	return nil
}

// Internal method to run backup steps
// Can be executed via Run() or Start(), which handle the 'running' property
func (b *BackupSet) run() {
	logger.Info("starting backup set", zap.Int("step_count", len(b.steps)))
	b.waitGroup = sync.WaitGroup{}

	if b.metrics == nil {
		logger.Error("metrics collection not assigned")
		return
	}

	err := b.InitializeRepository()
	if err != nil {
		// log output in subroutine
		return
	}

	for i, s := range b.steps {
		b.waitGroup.Add(1)
		go func() {
			defer b.waitGroup.Done()

			logger.Info("running backup step", zap.Int("index", i), zap.String("type", s.Type()), zap.String("description", s.Description()))

			err := s.Run(b.metrics)
			b.metrics.BackupsTotal.Inc()
			if err != nil {
				b.metrics.BackupsFailed.Inc()
				logger.Error("backup step failed", zap.Int("index", i), zap.String("type", s.Type()), zap.String("description", s.Description()), zap.Error(err))
				return
			}
			b.metrics.BackupsSuccessful.Inc()
			logger.Info("backup step finished", zap.Int("index", i), zap.String("type", s.Type()), zap.String("description", s.Description()))
		}()
	}

	b.waitGroup.Wait()
	logger.Info("all backup steps finished")
}

// Check if the repository exists, try to initialize otherwise
func (b *BackupSet) InitializeRepository() error {
	// drop this in favor of parseable output from "restic init --json" in a later version
	err := b.ensureRepository()
	if err == nil {
		logger.Info("repository alreay exists")
		return nil
	}

	logger.Warn("initalizing repository")
	// init does not support '--json' yet; but add it here so we see when support is there
	out, err := exec.Command("restic", "init", "--json").Output()
	if err != nil {
		exiterr, ok := err.(*exec.ExitError)
		if ok {
			logger.Error("command restic init failed", zap.Error(err), zap.ByteString("stdout", out),
				zap.ByteString("stderr", exiterr.Stderr), zap.Int("code", exiterr.ExitCode()),
			)
		} else {
			logger.Error("command restic init failed", zap.Error(err), zap.ByteString("stdout", out))
		}
		return err
	}

	// ok
	logger.Info("initializing complete", zap.ByteString("stdout", out))
	return nil
}

// Check if the repository exists
func (b *BackupSet) ensureRepository() error {
	logger.Debug("ensuring backup repository exists")
	cmd := exec.Command("restic", "snapshots", "--json", "--last")
	out, err := cmd.Output()
	if err != nil {
		exiterr, ok := err.(*exec.ExitError)
		if ok {
			logger.Warn("command restic snapshots failed", zap.Error(err), zap.ByteString("stdout", out),
				zap.ByteString("stderr", exiterr.Stderr), zap.Int("code", exiterr.ExitCode()),
			)
		} else {
			logger.Warn("command restic snapshots failed", zap.Error(err), zap.ByteString("stdout", out))
		}

		return err
	}

	return nil
}
