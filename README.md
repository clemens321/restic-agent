# Restic Agent

Backups done right... as agent!

This is a wrapper application for [Restic](https://github.com/restic/restic/) that:

- Automatically creates a repository
- Starts scheduled backups
- Can start backups HTTP triggered
- Integrates database dump methods
- Presents prometheus metrics
- Logs in JSON

## Usage

Restic-agent can be configured by environment variables and command line arguments.

Base environment variables:

- `RESTIC_REPOSITORY`: repository name
- `RESTIC_PASSWORD`: repository password
- `RESTIC_HOSTNAME`: overwrite hostname for snapshots
- `RUN_ON_STARTUP`: run a backup on container start
- `SCHEDULE`: cron schedule (with seconds)
- `DEBUG`: enable verbose output

## Docker Compose
Just add a restic-agent for simple backups:

```yml
services:
  #
  # some productive services...
  #

  backup:
    image: clemens321/restic-agent:latest
    restart: unless-stopped
    command:
      - "--volume=/data/app"
    volumes:
      # Bind whatever directories to the backup container.
      # You can safely bind the same directory to multiple containers.
      - "app_data:/data/app"
    environment:
      # start backup every day at 2am
      - "SCHEDULE=0 0 2 * * *"
      - "RESTIC_REPOSITORY=rest:https://foo:pass@host:8000/foo"
      - "RESTIC_PASSWORD=${MY_RESTIC_PASSWORD:-secret}"
```

## HTTP endpoints

### Start and status

- `/start` Start a backup set in background
- `/run` Start a backup set and wait for completion
- `/running` Check if a backup job is running (true/false)
- `/initialize` Explicitly initialize the repository

### Prometheus metrics

As `/metrics` restic-agent provides various prometheus metrics:

- `backups_all_total`: The total number of backups attempted, including failures.
- `backups_successful_total`: The total number of backups that succeeded.
- `backups_failed_total`: The total number of backups that failed.
- `backup_duration_milliseconds`: The duration of backups in milliseconds.
- `backup_files_new`: Amount of new files.
- `backup_files_changed`: Amount of files with changes.
- `backup_files_unmodified`: Amount of files unmodified since last backup.
- `backup_files_processed`: Total number of files scanned by the backup for changes.
- `backup_added_bytes`: Total number of bytes added to the repository.
- `backup_processed_bytes`: Total number of bytes scanned by the backup for changes

## Backup modules

### Volumes

Command line options:
- `--volume=/data/path`: volume path to snapshot
- `--volume=/data/path1,/data/path2`: multiple volumes in one step

Can be applied multiple times to add multiple restic steps.

#### Exclude Files from Volume

You can exclude folders and files by creating a `.resticexclude` in the root of each volume to be backed up.
If the file exists it will be passed to restic with the [`--exclude-file`](https://restic.readthedocs.io/en/latest/040_backup.html#excluding-files) parameter.  

### PostgreSQL

`POSTGRES_DB`, `POSTGRES_USER` and `POSTGRES_PASSWORD` are named as in the according docker image.

Environment options:
- `POSTGRES_NAME` (Virtual) filename in backup, default is "/psql-\<host>-\<database>.dmp"
- `POSTGRES_HOST` Host or service name of database server/container
- `POSTGRES_DB`
- `POSTGRES_USER`
- `POSTGRES_PASSWORD`

### MySQL / Mariadb

`MYSQL_DATABASE`, `MYSQL_USER` and `MYSQL_PASSWORD` are named as in the mariadb docker image.

Environment options:
- `MYSQL_NAME` (Virtual) filename in backup, default is "/mysql-\<host>-\<database>.dmp"
- `MYSQL_HOST` Host or service name of database server/container
- `MYSQL_DATABASE`
- `MYSQL_USER`
- `MYSQL_PASSWORD`

## Restore

There are no tools available besides the restic integrated ones. Here is a way to restore files and postgres:

```
docker-compose run --rm --entrypoint=/bin/sh restic

# First we will restore the database to an already created but empty one
restic snapshots
# search for the database dump to recover from
RESTIC_ID="paste_id_here"
# copy postgres password to clipboard, pg_pass would be written during the backup step
echo $POSTGRES_PASSWORD
restic dump $RESTIC_ID /psql-${POSTGRES_HOST}-${POSTGRES_DB}.dmp | psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DB
# paste postgres password here

# And now the files
restic snapshots
# search for the volume snapshot
RESTIC_ID="paste_id_here"
restic restore --target=/ $RESTIC_ID
```
