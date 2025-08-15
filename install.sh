#!/bin/bash

apt -y install dnsutils screen

# Run from base of repository.
go build ./src/cmd/manager
go build ./src/cmd/mcctl

cp minecraft-server-manager.service /usr/lib/systemd/system/
systemctl daemon-reload

systemctl stop minecraft-server-manager.service
cp manager /usr/bin/minecraft-server-manager
systemctl enable --now minecraft-server-manager.service

cp mcctl /usr/bin/