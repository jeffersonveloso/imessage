// mautrix-imessage - A Matrix-iMessage puppeting bridge.
// Copyright (C) 2024 Ludvig Rhodin
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"maunium.net/go/mautrix/bridgev2/matrix/mxmain"

	"github.com/lrhodin/imessage/pkg/connector"
)

// runAPIOnly starts the bridge in API-only mode: no Matrix homeserver required.
//
// It initialises the database, connector and HTTP API server, but skips the
// Matrix appservice connection entirely. This allows the HTTP REST API to run
// standalone for sending/receiving iMessages without a Matrix deployment.
func runAPIOnly(br *mxmain.BridgeMain) {
	// Remove "api-only" from args so flag parsing in PreInit works.
	os.Args = append(os.Args[:1], os.Args[2:]...)

	// Load config from disk (PreInit parses flags and calls LoadConfig).
	br.PreInit()

	// Ensure the HTTP API is enabled — that's the whole point.
	c := br.Connector.(*connector.IMConnector)
	c.APIOnly = true
	if !c.Config.API.Enabled {
		fmt.Fprintln(os.Stderr, "[!] API-only mode requires the HTTP API to be enabled.")
		fmt.Fprintln(os.Stderr, "    Set network.api.enabled = true in your config.")
		os.Exit(1)
	}

	// Patch required homeserver / appservice fields with dummy values so
	// BridgeMain.Init() validation passes. These are never used — no Matrix
	// connection is established in this mode.
	br.Config.Homeserver.Address = "http://localhost:1"
	br.Config.Homeserver.Domain = "api-only.local"
	br.Config.Homeserver.Software = "standard"

	// Generate random tokens so the "not configured" check passes.
	br.Config.AppService.ASToken = generateDummyToken()
	br.Config.AppService.HSToken = generateDummyToken()

	// Ensure permissions has an admin (needed by login adapter).
	if !br.Config.Bridge.Permissions.IsConfigured() {
		fmt.Fprintln(os.Stderr, "[!] bridge.permissions not configured. Add an admin entry.")
		fmt.Fprintln(os.Stderr, "    Example:  permissions:")
		fmt.Fprintln(os.Stderr, "               \"@admin:api-only.local\": admin")
		os.Exit(1)
	}

	// Init sets up logging, database, Matrix connector stub, and Bridge.
	br.Init()

	ctx := br.Log.WithContext(context.Background())

	// Run database migrations (normally done in StartConnectors).
	if err := br.DB.Upgrade(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "[!] Database migration failed: %v\n", err)
		os.Exit(1)
	}

	// Initialise BackgroundCtx (normally set in StartConnectors).
	var cancelBg context.CancelFunc
	br.Bridge.BackgroundCtx, cancelBg = context.WithCancel(context.Background())
	defer cancelBg()
	br.Bridge.BackgroundCtx = br.Log.WithContext(br.Bridge.BackgroundCtx)

	// Start the network connector (IMConnector) — this starts the HTTP API
	// server and restores any existing iMessage logins.
	br.Log.Info().Msg("Starting in API-only mode (no Matrix homeserver)")
	if err := br.Bridge.Network.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "[!] Failed to start connector: %v\n", err)
		os.Exit(1)
	}

	// Start existing logins so the iMessage connection is live.
	if err := br.Bridge.StartLogins(ctx); err != nil {
		br.Log.Warn().Err(err).Msg("Failed to start existing logins")
	}

	br.Log.Info().
		Str("listen", c.Config.API.Listen).
		Msg("HTTP REST API running — Ctrl+C to stop")

	// Wait for shutdown signal.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	br.Log.Info().Msg("Shutting down")
	os.Exit(0)
}

// generateDummyToken returns a random 32-byte hex string used as a placeholder
// appservice token that satisfies the "not configured" validation check.
func generateDummyToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
