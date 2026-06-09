package web

import "embed"

// Static contains the embedded admin UI assets.
//
//go:embed static/*
var Static embed.FS
