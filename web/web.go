package web

import "embed"

// Static contains the embedded admin UI assets.
//
//go:embed static/* static/core/* static/components/* static/admin/* static/account/* static/chat/* static/home/* static/css/* static/dist/* static/pwa/*
var Static embed.FS
