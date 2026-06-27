package web

import "embed"

// Static contains the embedded admin UI assets.
//
//go:embed static/* static/core/* static/components/* static/admin/* static/account/* static/chat/* static/css/* static/dist/*
var Static embed.FS
