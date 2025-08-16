# minecraft-server-manager

Random self project for managing a minecraft server hosted on Google Cloud.
This will handle tasks such as automated backups and uploads to a bucket, server recovery, and more.

Backups are meant to be scheduled via a CronJob or systemd-timer. I'm a stickler for perfect-looking time stamps,
so I went with the Crontab method. Maybe if I feel like it, I'll eventually add something to support something
in the binary natively.

By default, it assumes the base directory for the server and files to be located in `/etc/minecraft`. If you need
to change this, then use the `--modpackdir` flag to set the directory. This will need to be passed into every
single command call by `mcctl`, but eventually maybe I'll consider adding a configuration file to set these universally.