package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/kelseyhightower/envconfig"
	"github.com/pborman/getopt/v2"
	"github.com/robfig/cron"
	"go.uber.org/zap"
)

type config struct {
	Repository         string `envconfig:"RESTIC_REPOSITORY"`
	Password           string `envconfig:"RESTIC_PASSWORD"`
	Hostname           string `envconfig:"RESTIC_HOSTNAME"`
	RunOnStartup       bool   `envconfig:"RUN_ON_STARTUP"`
	Schedule           string `envconfig:"SCHEDULE"`
	ListenAddress      string `envconfig:"LISTEN_ADDRESS"`
	ListenPort         int    `envconfig:"LISTEN_PORT" default:"80"`
	PrometheusEndpoint string `envconfig:"PROMETHEUS_ENDPOINT" default:"/metrics"`

	PostgresName     string `envconfig:"POSTGRES_NAME"`
	PostgresHost     string `envconfig:"POSTGRES_HOST"`
	PostgresDatabase string `envconfig:"POSTGRES_DB"`
	PostgresPassword string `envconfig:"POSTGRES_PASSWORD"`
	PostgresUser     string `envconfig:"POSTGRES_USER"`
}

// main contains basic handling, primarily parsing the command line
// (along with the environment variables) and initialization of various
// endpoints and subsystems.
func main() {
	var wg sync.WaitGroup

	logger.Debug("restic-agent is starting")

	// parse configuration (environment)

	c := config{}

	logger.Debug("process environment variables")
	err := envconfig.Process("", &c)
	if err != nil {
		logger.Fatal("failed to configure", zap.Error(err))
	}

	// parse configuration (command-line)
	b := BackupSet{}
	parseCmdLine(&c, &b)
	// No Non-Debug output before this line

	// start http server
	if c.ListenPort != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Info("starting http server", zap.String("address", c.ListenAddress), zap.Int("port", c.ListenPort))
			err := http.ListenAndServe(c.ListenAddress+":"+strconv.Itoa(c.ListenPort), nil)
			logger.Fatal("http server closed", zap.Error(err))
		}()
	}
	// Even without http server listen&serving the http.Handle commands will be executed and the server will be configured

	// initialize prometheus metrics
	m := MetricsCollection{}
	m.Initialize()
	m.Register(nil)
	b.SetMetrics(&m)

	logger.Debug("serving prometheus endpoint", zap.String("endpoint", c.PrometheusEndpoint))
	http.Handle(c.PrometheusEndpoint, m.getHandler())

	// execute backup on startup
	if c.RunOnStartup {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Debug("run backup on startup")
			b.Run()
		}()
	}

	// add backup run handler
	logger.Debug("serving ui endpoints")
	// TODO: Move to separate handler object
	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		err := b.Start()
		if err != nil {
			fmt.Fprintf(w, err.Error())
		} else {
			fmt.Fprintf(w, "Backup started")
		}
	})

	http.HandleFunc("/run", func(w http.ResponseWriter, r *http.Request) {
		b.Run()
		// run does not return anything
		// TODO: When there is a Wait() which returns the exit state use Start()/Wait() here
		fmt.Fprintf(w, "done")
	})

	http.HandleFunc("/initalize", func(w http.ResponseWriter, r *http.Request) {
		err := b.InitializeRepository()
		if err == nil {
			fmt.Fprintf(w, "done")
		} else {
			fmt.Fprintf(w, "error: "+err.Error())
		}
	})

	http.HandleFunc("/running", func(w http.ResponseWriter, r *http.Request) {
		if b.IsRunning() {
			fmt.Fprintf(w, "true")
		} else {
			fmt.Fprintf(w, "false")
		}
	})

	// start cron scheduler
	if c.Schedule != "" {
		wg.Add(1)
		go func() {
			cr := cron.New()
			err := cr.AddJob(c.Schedule, &b)
			if err != nil {
				logger.Fatal("failed to schedule task", zap.Error(err))
			}
			cr.Run()
			logger.Fatal("cron scheduler terminated", zap.Error(err))
		}()
	}

	logger.Info("restic-agent startup complete")
	// wait for started goroutines
	wg.Wait()
}

func parseCmdLine(c *config, b *BackupSet) {
	var volumes []string

	help := getopt.BoolLong("help", '?', "print usage")
	getopt.FlagLong(&c.Hostname, "host", 'h', "set the hostname for restic snapshots")
	getopt.FlagLong(&volumes, "volume", 'v', "path to a volume to save, may added multiple times", "/data/path")
	getopt.FlagLong(&c.RunOnStartup, "run", 'r', "run on startup")
	getopt.FlagLong(&c.Schedule, "schedule", 's', "add cron schedule")
	getopt.FlagLong(&c.ListenAddress, "listen-host", 'l', "set listen address for http server")
	getopt.FlagLong(&c.ListenPort, "listen-port", 'p', "set listen port for http server")

	getopt.Parse()
	if *help {
		getopt.Usage()
		os.Exit(0)
	}

	b.SetRepository(c.Repository, c.Password)
	b.SetHostname(c.Hostname)

	// Add volume steps
	for _, v := range volumes {
		b.AddStep(NewVolumeStep(v))
	}

	// Add database steps by environment variables - PostgreSQL
	if c.PostgresHost != "" {
		s, err := NewPostgresStep(c.PostgresHost, c.PostgresUser, c.PostgresPassword, c.PostgresDatabase)
		if err != nil {
			logger.Fatal("Failed to add postgres step", zap.Error(err))
		}
		if c.PostgresName != "" {
			s.SetName(c.PostgresName)
		}
		b.AddStep(s)
	}
}
