# minecraft-server-manager

Random self project for managing a minecraft server hosted on Google Cloud.
This will handle tasks such as automated backups and uploads to a bucket, server recovery, and more.

Backups are meant to be scheduled via a CronJob or systemd-timer. I'm a stickler for perfect-looking time stamps,
so I went with the Crontab method. Maybe if I feel like it, I'll eventually add something to support something
in the binary natively.
