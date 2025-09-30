#!/bin/bash
cd /workspaces/ocom/go/makemigrations
GOROOT=/home/ocom/go/go1.24.2 /home/ocom/go/go1.24.2/bin/go test ./cmd -run TestMakeMigrationsAddFields -v 2>&1