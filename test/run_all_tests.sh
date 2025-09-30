#!/bin/bash
cd /workspaces/ocom/go/makemigrations
GOROOT=/home/ocom/go/go1.24.2 /home/ocom/go/go1.24.2/bin/go test ./cmd -run TestMakeMigrations -v 2>&1 | grep -E "PASS|FAIL"