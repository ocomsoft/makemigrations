#!/bin/bash
cd /workspaces/ocom/go/makemigrations
GOROOT=/home/ocom/go/go1.24.2 /home/ocom/go/go1.24.2/bin/go run test/test_all_providers.go 2>&1