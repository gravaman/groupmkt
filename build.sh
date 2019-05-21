#!/bin/sh

cd api && go build
cd ../cli && go build
cd ../client && go build
cd ../db && go build
cd ../frontend && go build

